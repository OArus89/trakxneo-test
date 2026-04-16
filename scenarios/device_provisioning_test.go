package scenarios

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeviceProvisioning_CRUD(t *testing.T) {
	e := setup(t)
	imei := "999000000000011"
	orgID := getOrgID(t, e)

	t.Run("create", func(t *testing.T) {
		body := map[string]any{
			"imei":         imei,
			"org_id":       orgID,
			"device_type":  "tracker",
			"device_model": "test-model",
			"protocol":     "teltonika",
			"name":         "E2E Test Device",
		}
		resp, err := e.api.Raw("POST", "/api/v1/devices", body)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Less(t, resp.StatusCode, 300, "create should succeed")
	})

	t.Run("read", func(t *testing.T) {
		dev, err := e.api.GetDevice(imei)
		require.NoError(t, err)
		assert.Equal(t, imei, dev["imei"])
	})

	t.Run("suspend", func(t *testing.T) {
		resp, err := e.api.Raw("POST", "/api/v1/devices/"+imei+"/suspend", nil)
		require.NoError(t, err)
		resp.Body.Close()
		assert.Less(t, resp.StatusCode, 300)
	})

	t.Run("reactivate", func(t *testing.T) {
		resp, err := e.api.Raw("POST", "/api/v1/devices/"+imei+"/reactivate", nil)
		require.NoError(t, err)
		resp.Body.Close()
		assert.Less(t, resp.StatusCode, 300)
	})

	t.Run("delete", func(t *testing.T) {
		err := e.api.DeleteDevice(imei)
		require.NoError(t, err)
	})
}

func TestDeviceProvisioning_LuhnValidation(t *testing.T) {
	e := setup(t)

	t.Run("invalid_luhn_rejected", func(t *testing.T) {
		body := map[string]any{
			"imei":         "123456789012345",
			"org_id":       getOrgID(t, e),
			"device_type":  "tracker",
			"device_model": "test-model",
			"protocol":     "teltonika",
		}
		resp, err := e.api.Raw("POST", "/api/v1/devices", body)
		require.NoError(t, err)
		resp.Body.Close()
		assert.GreaterOrEqual(t, resp.StatusCode, 400, "invalid Luhn should be rejected")
	})
}
