package discord

import (
	"bytes"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	discordAPIBaseURL = "https://discord.com/api/v10"
)

// Service wraps http.Client to interact with Discord REST API
type Service struct {
	client    *http.Client
	botToken  string
	appID     string
	publicKey ed25519.PublicKey
}

// NewService creates a new Discord service
func NewService(botToken, publicKeyHex string) (*Service, error) {
	publicKeyBytes, err := hex.DecodeString(publicKeyHex)
	if err != nil {
		return nil, fmt.Errorf("invalid public key: %w", err)
	}

	if len(publicKeyBytes) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("public key must be %d bytes", ed25519.PublicKeySize)
	}

	return &Service{
		client:    &http.Client{Timeout: 30 * time.Second},
		botToken:  botToken,
		publicKey: ed25519.PublicKey(publicKeyBytes),
	}, nil
}

// SetApplicationID sets the application ID (can be fetched via GetApplicationInfo)
func (s *Service) SetApplicationID(appID string) {
	s.appID = appID
}

// ApplicationInfo represents Discord application information
type ApplicationInfo struct {
	ID string `json:"id"`
}

// GetApplicationInfo fetches the bot's application information
func (s *Service) GetApplicationInfo() (*ApplicationInfo, error) {
	req, err := http.NewRequest("GET", discordAPIBaseURL+"/oauth2/applications/@me", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bot "+s.botToken)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get application info: status %d, body: %s", resp.StatusCode, string(body))
	}

	var appInfo ApplicationInfo
	if err := json.NewDecoder(resp.Body).Decode(&appInfo); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &appInfo, nil
}

// ApplicationCommand represents a Discord slash command
type ApplicationCommand struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Type        int    `json:"type,omitempty"` // 1 for slash command
}

// CreateCommand registers a slash command with Discord
func (s *Service) CreateCommand(command *ApplicationCommand) error {
	if s.appID == "" {
		return fmt.Errorf("application ID not set")
	}

	command.Type = 1 // Slash command type

	body, err := json.Marshal(command)
	if err != nil {
		return fmt.Errorf("failed to marshal command: %w", err)
	}

	url := fmt.Sprintf("%s/applications/%s/commands", discordAPIBaseURL, s.appID)
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bot "+s.botToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create command: status %d, body: %s", resp.StatusCode, string(body))
	}

	return nil
}

// VerifySignature verifies the Discord webhook signature
func (s *Service) VerifySignature(timestamp, signatureHex string, body []byte) bool {
	signature, err := hex.DecodeString(signatureHex)
	if err != nil {
		return false
	}

	message := append([]byte(timestamp), body...)
	return ed25519.Verify(s.publicKey, message, signature)
}

// InteractionResponse represents a response to a Discord interaction
type InteractionResponse struct {
	Type int                      `json:"type"`
	Data *InteractionResponseData `json:"data,omitempty"`
}

// InteractionResponseData contains the response data
type InteractionResponseData struct {
	Content string `json:"content,omitempty"`
}

const (
	InteractionResponseTypeChannelMessageWithSource = 4
)

// RespondToInteraction responds to a Discord interaction
func (s *Service) RespondToInteraction(interactionID, interactionToken string, response *InteractionResponse) error {
	body, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("failed to marshal response: %w", err)
	}

	url := fmt.Sprintf("%s/interactions/%s/%s/callback", discordAPIBaseURL, interactionID, interactionToken)
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to respond to interaction: status %d, body: %s", resp.StatusCode, string(body))
	}

	return nil
}

// WebhookEdit represents an edit to a webhook message
type WebhookEdit struct {
	Content *string `json:"content,omitempty"`
}

// EditInteractionResponse edits the original interaction response
func (s *Service) EditInteractionResponse(interactionToken string, edit *WebhookEdit) error {
	body, err := json.Marshal(edit)
	if err != nil {
		return fmt.Errorf("failed to marshal edit: %w", err)
	}

	url := fmt.Sprintf("%s/webhooks/%s/%s/messages/@original", discordAPIBaseURL, s.appID, interactionToken)
	req, err := http.NewRequest("PATCH", url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bot "+s.botToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to edit interaction response: status %d, body: %s", resp.StatusCode, string(body))
	}

	return nil
}

// WebhookParams represents parameters for creating a followup message
type WebhookParams struct {
	Content string `json:"content"`
}

// CreateFollowupMessage creates a followup message for an interaction
func (s *Service) CreateFollowupMessage(interactionToken string, params *WebhookParams) error {
	body, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("failed to marshal params: %w", err)
	}

	url := fmt.Sprintf("%s/webhooks/%s/%s", discordAPIBaseURL, s.appID, interactionToken)
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bot "+s.botToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create followup message: status %d, body: %s", resp.StatusCode, string(body))
	}

	return nil
}
