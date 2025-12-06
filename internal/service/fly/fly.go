package fly

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

type Service struct {
	apiToken  string
	appName   string
	machineID string
}

func NewService(apiToken, appName, machineID string) *Service {
	return &Service{
		apiToken:  apiToken,
		appName:   appName,
		machineID: machineID,
	}
}

type requestOptions struct {
	timeout time.Duration
	body    io.Reader
}

type RequestOption func(*requestOptions)

// WithTimeout sets the timeout for the HTTP request
func WithTimeout(timeout time.Duration) RequestOption {
	return func(opts *requestOptions) {
		opts.timeout = timeout
	}
}

// WithBody sets the request body
func WithBody(body io.Reader) RequestOption {
	return func(opts *requestOptions) {
		opts.body = body
	}
}

// request performs an HTTP request to the Fly API
func (s *Service) request(method, path string, opts ...RequestOption) (*http.Response, error) {
	// Default options
	options := &requestOptions{
		timeout: 10 * time.Second, // Default timeout
		body:    nil,              // Default no body
	}

	// Apply provided options
	for _, opt := range opts {
		opt(options)
	}

	url := fmt.Sprintf("https://api.machines.dev%s", path)

	req, err := http.NewRequest(method, url, options.body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+s.apiToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: options.timeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}

	return resp, nil
}

func (s *Service) StartMachine() error {
	path := fmt.Sprintf("/v1/apps/%s/machines/%s/start", s.appName, s.machineID)
	resp, err := s.request("POST", path, WithTimeout(30*time.Second))
	if err != nil {
		return fmt.Errorf("failed to start machine: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to start machine: status %d, body: %s", resp.StatusCode, string(body))
	}

	log.Printf("Machine %s started successfully", s.machineID)
	return nil
}

func (s *Service) CheckHealth() bool {
	machine, err := s.GetMachine()
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

func (s *Service) GetMachine() (*MachineResponse, error) {
	path := fmt.Sprintf("/v1/apps/%s/machines/%s", s.appName, s.machineID)
	resp, err := s.request("GET", path)
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
