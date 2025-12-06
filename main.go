package main

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/big"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/thattomperson/barry/internal/service/discord"
	"github.com/thattomperson/barry/internal/service/fly"
)

type Bot struct {
	discordService *discord.Service
	flyService     *fly.Service
}

// Interaction represents a Discord interaction
type Interaction struct {
	ID     string       `json:"id"`
	Token  string       `json:"token"`
	Type   int          `json:"type"`
	Data   *CommandData `json:"data,omitempty"`
	Member *Member      `json:"member,omitempty"`
	User   *User        `json:"user,omitempty"`
}

// CommandData represents command data in an interaction
type CommandData struct {
	Name string `json:"name"`
}

// Member represents a Discord guild member
type Member struct {
	User *User `json:"user"`
}

// User represents a Discord user
type User struct {
	ID string `json:"id"`
}

const (
	InteractionTypePing               = 1
	InteractionTypeApplicationCommand = 2
)

func main() {
	botToken := os.Getenv("DISCORD_BOT_TOKEN")
	if botToken == "" {
		log.Fatal("DISCORD_BOT_TOKEN environment variable is required")
	}

	publicKey := os.Getenv("DISCORD_PUBLIC_KEY")
	if publicKey == "" {
		log.Fatal("DISCORD_PUBLIC_KEY environment variable is required")
	}

	flyAPIToken := os.Getenv("MC_FLY_API_TOKEN")
	if flyAPIToken == "" {
		log.Fatal("MC_FLY_API_TOKEN environment variable is required")
	}

	flyAppName := os.Getenv("MC_FLY_APP_NAME")
	if flyAppName == "" {
		log.Fatal("MC_FLY_APP_NAME environment variable is required")
	}

	machineID := os.Getenv("MC_FLY_MACHINE_ID")
	if machineID == "" {
		log.Fatal("MC_FLY_MACHINE_ID environment variable is required")
	}

	// Create Discord service
	discordService, err := discord.NewService(botToken, publicKey)
	if err != nil {
		log.Fatalf("Error creating Discord service: %v", err)
	}

	// Get application ID
	appInfo, err := discordService.GetApplicationInfo()
	if err != nil {
		log.Fatalf("Error getting application info: %v", err)
	}
	discordService.SetApplicationID(appInfo.ID)
	log.Printf("Bot application ID: %s", appInfo.ID)

	flyService := fly.NewService(flyAPIToken, flyAppName, machineID)

	bot := &Bot{
		discordService: discordService,
		flyService:     flyService,
	}

	// Register slash command
	commands := []*discord.ApplicationCommand{
		{
			Name:        "start-server",
			Description: "✨ Squeak! Let Barry the magical mouse start your server! 🐭",
		},
	}

	for _, command := range commands {
		if err := discordService.CreateCommand(command); err != nil {
			log.Printf("Cannot create command '%s': %v", command.Name, err)
		} else {
			log.Printf("Created command '%s'", command.Name)
		}
	}

	// Set up HTTP server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/interactions", bot.handleWebhook)

	server := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	// Start server in goroutine
	go func() {
		log.Printf("HTTP server listening on port %s", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Error starting HTTP server: %v", err)
		}
	}()

	log.Println("Bot is now running. Press CTRL-C to exit.")

	// Wait for interrupt signal
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc

	log.Println("Bot is shutting down...")
	if err := server.Close(); err != nil {
		log.Printf("Error closing server: %v", err)
	}
}

func (b *Bot) handleWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		log.Println("handleWebhook: exit - method not allowed")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Read body for signature verification
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("handleWebhook: exit - failed to read body: %v", err)
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		return
	}
	r.Body = io.NopCloser(bytes.NewBuffer(body))

	// Verify signature
	timestamp := r.Header.Get("X-Signature-Timestamp")
	signature := r.Header.Get("X-Signature-Ed25519")

	if timestamp == "" || signature == "" {
		log.Println("handleWebhook: exit - missing signature headers")
		http.Error(w, "Missing signature headers", http.StatusUnauthorized)
		return
	}

	if !b.discordService.VerifySignature(timestamp, signature, body) {
		log.Println("handleWebhook: exit - invalid signature")
		http.Error(w, "Invalid signature", http.StatusUnauthorized)
		return
	}

	// Parse interaction
	var interaction Interaction
	if err := json.Unmarshal(body, &interaction); err != nil {
		log.Printf("handleWebhook: exit - invalid JSON: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Handle ping
	if interaction.Type == InteractionTypePing {
		log.Println("handleWebhook: exit - PING interaction handled")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]int{"type": 1}) // PONG
		return
	}

	// Handle application command
	if interaction.Type == InteractionTypeApplicationCommand {
		if interaction.Data != nil && interaction.Data.Name == "start-server" {
			log.Println("handleWebhook: exit - start-server command handled")
			b.handleStartServer(w, &interaction)
			return
		}
	}

	log.Printf("handleWebhook: exit - unknown interaction type: %d", interaction.Type)
	http.Error(w, "Unknown interaction type", http.StatusBadRequest)
}

// getUserMention returns a Discord mention string for the user who triggered the interaction
func getUserMention(i *Interaction) string {
	if i.Member != nil && i.Member.User != nil {
		return fmt.Sprintf("<@%s>", i.Member.User.ID)
	}
	if i.User != nil {
		return fmt.Sprintf("<@%s>", i.User.ID)
	}
	return ""
}

func (b *Bot) handleStartServer(w http.ResponseWriter, i *Interaction) {
	// Acknowledge the interaction immediately (Discord requires response within 3 seconds)
	response := &discord.InteractionResponse{
		Type: discord.InteractionResponseTypeChannelMessageWithSource,
		Data: &discord.InteractionResponseData{
			Content: "✨ *squeak squeak* Oh! Time to work my magic! 🪄✨ Let me wake up that sleepy server for you... This might take a moment, but I'm on it! 🐭",
		},
	}

	// Respond via HTTP response (Discord expects this for webhook interactions)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Error encoding response: %v", err)
		return
	}

	// Helper to update the original response message
	updateMessage := func(content string) {
		edit := &discord.WebhookEdit{
			Content: &content,
		}
		if err := b.discordService.EditInteractionResponse(i.Token, edit); err != nil {
			log.Printf("Error updating message: %v", err)
		}
	}

	// Start the machine in a goroutine
	go func() {
		userMention := getUserMention(i)

		// Start the Fly machine
		if err := b.flyService.StartMachine(); err != nil {
			log.Printf("Error starting Fly machine: %v", err)
			updateMessage(fmt.Sprintf("😿 *squeak* Oh no! My magic spell didn't work quite right... The server didn't want to wake up! %v\n\nMaybe try again? I'll do my best! 🐭✨", err))
			return
		}

		log.Println("Fly machine starting")

		// Update message to show we're checking health
		updateMessage("✨ *squeak squeak* Oh! Time to work my magic! 🪄✨ Let me wake up that sleepy server for you... This might take a moment, but I'm on it! 🐭\n\n🔮 *twitching whiskers* The server is stirring! Let me peek into my crystal ball and check if it's feeling healthy... ✨")

		// Poll health checks every 30 seconds
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		checkCount := 0

		// Check immediately first
		checkCount++
		if b.flyService.CheckHealth() {
			log.Println("Health check passed")
			updateMessage("🎉 *happy squeaks* ✨ Ta-da! My magic worked perfectly! The server is all awake and ready to play! 🐭🎮\n\n*does a little mouse dance* 🕺✨")
			return
		}

		// Then check every 30 seconds
		for range ticker.C {
			checkCount++

			// Make a remark on the 6th health check
			if checkCount == 6 {
				remarks := []string{
					"🐭 *adjusts tiny wizard hat* Hmm, this server is being quite sleepy today! But don't worry, I'm a patient mouse! ✨",
					"🔮 *peers into crystal ball again* Still checking... This server must be having some really good dreams! 😴✨",
					"*squeak* Still working on it! My magic is strong, but some servers need extra time to wake up properly! 🪄🐭",
					"✨ *twitches whiskers thoughtfully* Hmm, this is taking longer than usual! But I won't give up - I'm a determined little mouse! 🐭💪",
					"🔮 *checks crystal ball for the 6th time* Still not quite ready yet... But I can feel it getting closer! My whiskers are tingling! ✨🐭",
				}
				// Use crypto/rand for randomness so multiple bots/goroutines don't repeat in lockstep.
				n, err := rand.Int(rand.Reader, big.NewInt(int64(len(remarks))))
				idx := 0
				if err == nil {
					idx = int(n.Int64())
				}
				remark := remarks[idx]
				updateMessage(remark)
			}

			if b.flyService.CheckHealth() {
				log.Println("Health check passed")
				followup := &discord.WebhookParams{
					Content: fmt.Sprintf("%s 🎉 *happy squeaks* ✨ Ta-da! My magic worked perfectly! The server is all awake and ready to play! 🐭🎮\n\n*does a little mouse dance* 🕺✨", userMention),
				}
				if err := b.discordService.CreateFollowupMessage(i.Token, followup); err != nil {
					log.Printf("Error creating followup message: %v", err)
				}
				return
			}
		}
	}()
}
