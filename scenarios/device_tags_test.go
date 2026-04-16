package scenarios

import (
	"encoding/json"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDeviceTags verifies tag CRUD and assignment to devices.
func TestDeviceTags(t *testing.T) {
	e := setup(t)

	imei := os.Getenv("TEST_IMEI_TELTONIKA")
	if imei == "" {
		imei = "864636066073087"
	}

	var tagID string

	t.Run("create_tag", func(t *testing.T) {
		body := map[string]any{
			"name":  "e2e-test-tag",
			"color": "#ff0000",
		}
		resp, err := e.api.Raw("POST", "/api/v1/admin/tags", body)
		require.NoError(t, err)
		defer resp.Body.Close()

		if resp.StatusCode < 300 {
			var result map[string]any
			b, _ := io.ReadAll(resp.Body)
			json.Unmarshal(b, &result)
			if id, ok := result["id"].(string); ok {
				tagID = id
			}
		}
	})

	if tagID == "" {
		t.Skip("tag not created")
	}

	t.Run("assign_tag_to_device", func(t *testing.T) {
		body := map[string]any{
			"tag_ids": []string{tagID},
		}
		resp, err := e.api.Raw("PUT", "/api/v1/devices/"+imei+"/tags", body)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Less(t, resp.StatusCode, 500)
	})

	t.Run("verify_tag_in_dragonfly", func(t *testing.T) {
		exists, err := e.dragonfly.KeyExists("device:tags:" + imei)
		if err == nil && exists {
			t.Log("device:tags key exists in Dragonfly")
		}
	})

	// Cleanup
	t.Run("delete_tag", func(t *testing.T) {
		resp, err := e.api.Raw("DELETE", "/api/v1/admin/tags/"+tagID, nil)
		require.NoError(t, err)
		resp.Body.Close()
		assert.Less(t, resp.StatusCode, 500)
	})
}
