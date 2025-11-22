package main

import (
	"crypto/rand"
	"fmt"
	"log"
	"math/big"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"
)

type Bot struct {
	session     *discordgo.Session
	flyAPIToken string
	flyAppName  string
	machineID   string
}

func main() {
	botToken := os.Getenv("DISCORD_BOT_TOKEN")
	if botToken == "" {
		log.Fatal("DISCORD_BOT_TOKEN environment variable is required")
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

	// Create Discord session
	session, err := discordgo.New("Bot " + botToken)
	if err != nil {
		log.Fatalf("Error creating Discord session: %v", err)
	}

	bot := &Bot{
		session:     session,
		flyAPIToken: flyAPIToken,
		flyAppName:  flyAppName,
		machineID:   machineID,
	}

	// Register slash command handler
	session.AddHandler(bot.handleInteraction)

	// Open Discord connection
	session.Identify.Intents = discordgo.IntentsGuildMessages
	if err = session.Open(); err != nil {
		log.Fatalf("Error opening Discord connection: %v", err)
	}
	defer session.Close()

	log.Println("Bot is now running. Press CTRL-C to exit.")

	// Register slash command
	commands := []*discordgo.ApplicationCommand{
		{
			Name:        "start-server",
			Description: "✨ Squeak! Let Barry the magical mouse start your server! 🐭",
		},
	}

	for _, command := range commands {
		_, err = session.ApplicationCommandCreate(session.State.User.ID, "", command)
		if err != nil {
			log.Printf("Cannot create command '%s': %v", command.Name, err)
		}
	}

	// Wait for interrupt signal
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc

	log.Println("Bot is shutting down...")
}

func (b *Bot) handleInteraction(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i.ApplicationCommandData().Name == "start-server" {
		b.handleStartServer(s, i)
	}
}

// getUserMention returns a Discord mention string for the user who triggered the interaction
func getUserMention(i *discordgo.InteractionCreate) string {
	if i.Member != nil && i.Member.User != nil {
		return fmt.Sprintf("<@%s>", i.Member.User.ID)
	}
	if i.User != nil {
		return fmt.Sprintf("<@%s>", i.User.ID)
	}
	return ""
}

func (b *Bot) handleStartServer(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// Acknowledge the interaction immediately (Discord requires response within 3 seconds)
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "✨ *squeak squeak* Oh! Time to work my magic! 🪄✨ Let me wake up that sleepy server for you... This might take a moment, but I'm on it! 🐭",
		},
	})
	if err != nil {
		log.Printf("Error responding to interaction: %v", err)
		return
	}

	// Helper to update the original response message
	updateMessage := func(content string) {
		_, _ = s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content: &content,
		})
	}

	// Start the machine in a goroutine
	go func() {
		userMention := getUserMention(i)

		// Start the Fly machine
		if err := b.startFlyMachine(); err != nil {
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
		if b.checkHealth() {
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

			if b.checkHealth() {
				log.Println("Health check passed")
				_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
					Content: fmt.Sprintf("%s 🎉 *happy squeaks* ✨ Ta-da! My magic worked perfectly! The server is all awake and ready to play! 🐭🎮\n\n*does a little mouse dance* 🕺✨", userMention),
				})
				return
			}
		}
	}()
}
