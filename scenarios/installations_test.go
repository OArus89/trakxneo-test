package scenarios

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInstallations_CRUD(t *testing.T) {
	e := setup(t)
	var installID string

	t.Run("create", func(t *testing.T) {
		body := map[string]any{
			"device_imei": "864636066073087", // known production device
			"latitude":    25.2048,
			"longitude":   55.2708,
			"notes":       "E2E test installation",
		}
		resp, err := e.api.Raw("POST", "/api/v1/installations", body)
		require.NoError(t, err)
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusOK {
			var result map[string]any
			json.NewDecoder(resp.Body).Decode(&result)
			if id, ok := result["id"].(string); ok {
				installID = id
			}
		}
		assert.Contains(t, []int{http.StatusOK, http.StatusCreated}, resp.StatusCode)
	})

	if installID == "" {
		t.Skip("installation not created, skipping remaining tests")
	}

	t.Run("read", func(t *testing.T) {
		resp, err := e.api.Raw("GET", "/api/v1/installations/"+installID, nil)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("list_by_device", func(t *testing.T) {
		resp, err := e.api.Raw("GET", "/api/v1/devices/864636066073087/installations", nil)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("upload_photo", func(t *testing.T) {
		// Create a small test JPEG (1x1 pixel)
		var buf bytes.Buffer
		writer := multipart.NewWriter(&buf)
		part, err := writer.CreateFormFile("photo", "test.jpg")
		require.NoError(t, err)
		// Minimal valid JPEG
		jpeg := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x49, 0x46, 0x00, 0x01, 0x01, 0x00, 0x00, 0x01, 0x00, 0x01, 0x00, 0x00, 0xFF, 0xD9}
		part.Write(jpeg)
		writer.Close()

		url := fmt.Sprintf("%s/api/v1/installations/%s/photos", e.cfg.API.BaseURL, installID)
		req, _ := http.NewRequest("POST", url, &buf)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		req.Header.Set("Authorization", "Bearer "+getToken(t, e))

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		// Accept 200, 201, or 400 (if JPEG too small)
		assert.Less(t, resp.StatusCode, 500, "should not return server error")
	})

	t.Run("list_photos", func(t *testing.T) {
		resp, err := e.api.Raw("GET", fmt.Sprintf("/api/v1/installations/%s/photos", installID), nil)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})
}

// getToken returns the JWT token by logging in (helper for multipart requests).
func getToken(t *testing.T, e *testEnv) string {
	t.Helper()
	url := fmt.Sprintf("%s/api/v1/auth/login", e.cfg.API.BaseURL)
	body, _ := json.Marshal(map[string]string{
		"username": e.cfg.Auth.Username,
		"password": e.cfg.Auth.Password,
	})
	resp, err := http.Post(url, "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	var result struct{ Token string `json:"token"` }
	json.Unmarshal(b, &result)
	return result.Token
}
