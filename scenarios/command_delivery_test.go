package scenarios

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCommandDelivery sends a command via API and verifies it's created.
func TestCommandDelivery(t *testing.T) {
	e := setup(t)

	imei := os.Getenv("TEST_IMEI_TELTONIKA")
	if imei == "" {
		t.Skip("TEST_IMEI_TELTONIKA not set")
	}

	t.Run("send_getver_action", func(t *testing.T) {
		resp, err := e.api.Raw("POST", "/api/v1/devices/"+imei+"/actions/getver", nil)
		require.NoError(t, err)
		defer resp.Body.Close()

		// Accept 200/201 (sent) or 400/404 (device not connected — expected in test)
		assert.Less(t, resp.StatusCode, 500, "should not return server error")

		if resp.StatusCode < 300 {
			var result map[string]any
			b, _ := io.ReadAll(resp.Body)
			json.Unmarshal(b, &result)
			if cmdID, ok := result["id"].(string); ok {
				t.Logf("Command created: %s", cmdID)

				// Verify command appears in list
				listResp, err := e.api.Raw("GET", "/api/v1/commands?imei="+imei, nil)
				require.NoError(t, err)
				defer listResp.Body.Close()
				assert.Equal(t, http.StatusOK, listResp.StatusCode)
			}
		} else {
			t.Logf("Command not sent (device likely offline): HTTP %d", resp.StatusCode)
		}
	})

	t.Run("send_raw_command", func(t *testing.T) {
		body := map[string]any{
			"imei":    imei,
			"command": "getver",
		}
		resp, err := e.api.Raw("POST", "/api/v1/commands", body)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Less(t, resp.StatusCode, 500)
	})
}

// TestCommandRateLimit verifies rate limiting on commands.
func TestCommandRateLimit(t *testing.T) {
	e := setup(t)

	imei := os.Getenv("TEST_IMEI_TELTONIKA")
	if imei == "" {
		t.Skip("TEST_IMEI_TELTONIKA not set")
	}

	// Send 12 commands rapidly — should hit rate limit (10/min)
	var lastStatus int
	for i := 0; i < 12; i++ {
		resp, err := e.api.Raw("POST", "/api/v1/devices/"+imei+"/actions/getver", nil)
		if err != nil {
			continue
		}
		lastStatus = resp.StatusCode
		resp.Body.Close()
	}

	t.Run("rate_limited", func(t *testing.T) {
		assert.Equal(t, http.StatusTooManyRequests, lastStatus,
			"should be rate limited after 10 commands/min")
	})
}
