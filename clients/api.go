package clients

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/OArus89/trakxneo-test/config"
)

// APIClient wraps HTTP calls to the TrakXNeo REST API.
type APIClient struct {
	baseURL string
	token   string
	http    *http.Client
}

func NewAPIClient(cfg *config.Config) *APIClient {
	return &APIClient{
		baseURL: cfg.API.BaseURL,
		http: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// Login authenticates and stores the JWT token for subsequent requests.
func (c *APIClient) Login(username, password string) error {
	body := map[string]string{"email": username, "password": password}
	resp, err := c.post("/api/v1/auth/login", body, false)
	if err != nil {
		return fmt.Errorf("login: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("login failed (%d): %s", resp.StatusCode, b)
	}

	var result struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("decode login response: %w", err)
	}
	c.token = result.Token
	return nil
}

// CreateDevice creates a test device and returns the response body.
func (c *APIClient) CreateDevice(imei, orgID, model, protocol string) (map[string]any, error) {
	body := map[string]any{
		"imei":        imei,
		"org_id":      orgID,
		"device_model": model,
		"protocol":    protocol,
		"status":      "active",
	}
	resp, err := c.post("/api/v1/devices", body, true)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return decodeJSON(resp)
}

// DeleteDevice removes a test device.
func (c *APIClient) DeleteDevice(imei string) error {
	resp, err := c.do("DELETE", "/api/v1/devices/"+imei, nil, true)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

// GetDevice fetches device details.
func (c *APIClient) GetDevice(imei string) (map[string]any, error) {
	resp, err := c.do("GET", "/api/v1/devices/"+imei, nil, true)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return decodeJSON(resp)
}

// GetDashboardDevice fetches device state from dashboard endpoint.
func (c *APIClient) GetDashboardDevice(imei string) (map[string]any, error) {
	resp, err := c.do("GET", "/api/v1/dashboard/device/"+imei, nil, true)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return decodeJSON(resp)
}

// GetTrips fetches trips for a device.
func (c *APIClient) GetTrips(imei string) ([]map[string]any, error) {
	resp, err := c.do("GET", "/api/v1/trips/"+imei, nil, true)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Trips []map[string]any `json:"trips"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result.Trips, nil
}

// GetAlerts fetches active alerts.
func (c *APIClient) GetAlerts() ([]map[string]any, error) {
	resp, err := c.do("GET", "/api/v1/alerts", nil, true)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result, nil
}

// Raw sends an arbitrary request — for edge cases and custom endpoints.
func (c *APIClient) Raw(method, path string, body any) (*http.Response, error) {
	return c.do(method, path, body, true)
}

func (c *APIClient) post(path string, body any, auth bool) (*http.Response, error) {
	return c.do("POST", path, body, auth)
}

func (c *APIClient) do(method, path string, body any, auth bool) (*http.Response, error) {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, c.baseURL+path, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if auth && c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	return c.http.Do(req)
}

func decodeJSON(resp *http.Response) (map[string]any, error) {
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, b)
	}
	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result, nil
}
