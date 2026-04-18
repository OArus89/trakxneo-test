package scenarios

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"testing"
	"time"

	"github.com/OArus89/trakxneo-test/clients"
	"github.com/OArus89/trakxneo-test/config"
)

const (
	testIMEITeltonika = "999000000000029"
	testIMEIRuptela   = "999000000000037"
)

func TestMain(m *testing.M) {
	cfg, err := config.Load("")
	if err != nil {
		log.Fatalf("TestMain: load config: %v", err)
	}

	api := clients.NewAPIClient(cfg)
	if err := api.Login(cfg.Auth.Username, cfg.Auth.Password); err != nil {
		log.Fatalf("TestMain: login: %v", err)
	}

	orgID, err := fetchOrgID(api)
	if err != nil {
		log.Fatalf("TestMain: fetch org ID: %v", err)
	}

	// Create Teltonika test device.
	if err := provisionDevice(api, testIMEITeltonika, orgID, "teltonika"); err != nil {
		log.Fatalf("TestMain: create Teltonika device: %v", err)
	}
	log.Printf("TestMain: created Teltonika device %s", testIMEITeltonika)

	// Create Ruptela test device.
	if err := provisionDevice(api, testIMEIRuptela, orgID, "ruptela"); err != nil {
		log.Fatalf("TestMain: create Ruptela device: %v", err)
	}
	log.Printf("TestMain: created Ruptela device %s", testIMEIRuptela)

	// Expose IMEIs to individual tests via env vars.
	os.Setenv("TEST_IMEI_TELTONIKA", testIMEITeltonika)
	os.Setenv("TEST_IMEI_RUPTELA", testIMEIRuptela)

	// Wait for devices to propagate through the system (API → PG → Gateway cache).
	log.Printf("TestMain: waiting 30s for device propagation to gateway cache...")
	time.Sleep(30 * time.Second)

	code := m.Run()

	// Cleanup — delete both devices regardless of test outcome.
	if err := api.DeleteDevice(testIMEITeltonika); err != nil {
		log.Printf("TestMain: cleanup Teltonika: %v", err)
	} else {
		log.Printf("TestMain: deleted Teltonika device %s", testIMEITeltonika)
	}
	if err := api.DeleteDevice(testIMEIRuptela); err != nil {
		log.Printf("TestMain: cleanup Ruptela: %v", err)
	} else {
		log.Printf("TestMain: deleted Ruptela device %s", testIMEIRuptela)
	}

	os.Exit(code)
}

func fetchOrgID(api *clients.APIClient) (string, error) {
	resp, err := api.Raw("GET", "/api/v1/admin/organizations", nil)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		Organizations []struct {
			ID string `json:"id"`
		} `json:"organizations"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if len(result.Organizations) == 0 {
		return "", fmt.Errorf("no organizations found")
	}
	return result.Organizations[0].ID, nil
}

func provisionDevice(api *clients.APIClient, imei, orgID, protocol string) error {
	// Remove leftover from a previous failed run.
	_ = api.DeleteDevice(imei)

	body := map[string]any{
		"imei":         imei,
		"org_id":       orgID,
		"device_type":  "tracker",
		"device_model": "test-model",
		"protocol":     protocol,
		"name":         "E2E " + protocol + " test",
	}
	resp, err := api.Raw("POST", "/api/v1/devices", body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("create HTTP %d: %s", resp.StatusCode, b)
	}

	// API creates devices as inactive — activate explicitly.
	resp2, err := api.Raw("POST", "/api/v1/devices/"+imei+"/reactivate", nil)
	if err != nil {
		return fmt.Errorf("reactivate: %w", err)
	}
	resp2.Body.Close()
	if resp2.StatusCode >= 300 {
		return fmt.Errorf("reactivate HTTP %d", resp2.StatusCode)
	}
	return nil
}
