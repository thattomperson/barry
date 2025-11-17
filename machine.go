package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

type MachineResponse struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	State  string `json:"state"`
	Checks []struct {
		Name      string    `json:"name"`
		Output    string    `json:"output"`
		Status    string    `json:"status"`
		UpdatedAt time.Time `json:"updated_at"`
	} `json:"checks"`
	Config struct {
		Services []struct {
			Protocol     string `json:"protocol"`
			InternalPort int    `json:"internal_port"`
			Ports        []struct {
				Port     int      `json:"port"`
				Handlers []string `json:"handlers"`
			} `json:"ports"`
			Checks []struct {
				Type        string `json:"type"`
				Interval    string `json:"interval"`
				Timeout     string `json:"timeout"`
				GracePeriod string `json:"grace_period"`
				Method      string `json:"method,omitempty"`
				Path        string `json:"path,omitempty"`
				Protocol    string `json:"protocol,omitempty"`
				Port        int    `json:"port,omitempty"`
			} `json:"checks"`
		} `json:"services"`
	} `json:"config"`
	Events []struct {
		ID        string    `json:"id"`
		Type      string    `json:"type"`
		Status    string    `json:"status"`
		Request   *struct{} `json:"request,omitempty"`
		Source    string    `json:"source"`
		Timestamp int64     `json:"timestamp"`
	} `json:"events"`
}

func (b *Bot) startFlyMachine() error {
	url := fmt.Sprintf("https://api.machines.dev/v1/apps/%s/machines/%s/start", b.flyAppName, b.machineID)

	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+b.flyAPIToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to start machine: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to start machine: status %d, body: %s", resp.StatusCode, string(body))
	}

	log.Printf("Machine %s started successfully", b.machineID)
	return nil
}

func (b *Bot) checkHealth() bool {
	machine, err := b.getMachine()
	if err != nil {
		log.Printf("Failed to get machine status: %v", err)
		return false
	}

	// Check if machine is started
	if machine.State != "started" {
		log.Printf("Machine state is: %s (waiting for 'started')", machine.State)
		return false
	}

	// Check if health checks are configured
	hasHealthChecks := false
	if len(machine.Config.Services) > 0 {
		for _, service := range machine.Config.Services {
			if len(service.Checks) > 0 {
				hasHealthChecks = true
				break
			}
		}
	}

	// If health checks are configured, check the actual check statuses
	if hasHealthChecks {
		if len(machine.Checks) == 0 {
			// Health checks are configured but no check results yet
			log.Println("Machine is started with health checks configured, waiting for health check results...")
			return false
		}

		// Check all health checks - they all need to be passing
		allPassing := true
		for _, check := range machine.Checks {
			// Status can be "passing", "warning", "critical", or "unknown"
			if check.Status != "passing" {
				log.Printf("Health check '%s' status: %s (output: %s)", check.Name, check.Status, check.Output)
				allPassing = false
			} else {
				log.Printf("Health check '%s' is passing", check.Name)
			}
		}

		if allPassing {
			log.Println("All health checks are passing!")
			return true
		}

		log.Println("Some health checks are not passing yet")
		return false
	}

	// No health checks configured, just check if machine is started
	log.Println("Machine is started (no health checks configured)")
	return true
}

func (b *Bot) getMachine() (*MachineResponse, error) {
	url := fmt.Sprintf("https://api.machines.dev/v1/apps/%s/machines/%s", b.flyAppName, b.machineID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+b.flyAPIToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get machine: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get machine: status %d, body: %s", resp.StatusCode, string(body))
	}

	var machine MachineResponse
	if err := json.NewDecoder(resp.Body).Decode(&machine); err != nil {
		return nil, fmt.Errorf("failed to decode machine response: %w", err)
	}

	return &machine, nil
}
