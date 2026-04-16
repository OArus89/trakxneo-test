package scenarios

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFleetAPI verifies fleet-related endpoints.
func TestFleetAPI(t *testing.T) {
	e := setup(t)

	t.Run("list_devices", func(t *testing.T) {
		resp, err := e.api.Raw("GET", "/api/v1/devices", nil)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		b, _ := io.ReadAll(resp.Body)
		var result struct {
			Devices []any `json:"devices"`
			Count   int   `json:"count"`
		}
		json.Unmarshal(b, &result)
		// Might be array or wrapped
		if result.Count == 0 {
			var arr []any
			json.Unmarshal(b, &arr)
			assert.Greater(t, len(arr), 0, "should have devices")
		}
	})

	t.Run("filter_by_status", func(t *testing.T) {
		resp, err := e.api.Raw("GET", "/api/v1/devices?status=active", nil)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("filter_by_org", func(t *testing.T) {
		orgID := getOrgID(t, e)
		resp, err := e.api.Raw("GET", "/api/v1/devices?org_id="+orgID, nil)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("device_models_list", func(t *testing.T) {
		resp, err := e.api.Raw("GET", "/api/v1/admin/device-models", nil)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Less(t, resp.StatusCode, 500)
	})

	t.Run("sim_cards_list", func(t *testing.T) {
		resp, err := e.api.Raw("GET", "/api/v1/admin/sim-cards", nil)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Less(t, resp.StatusCode, 500)
	})
}
