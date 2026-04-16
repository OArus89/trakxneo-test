package scenarios

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeviceProvisioning_CRUD(t *testing.T) {
	e := setup(t)
	imei := "999000000000011"

	// Create
	t.Run("create", func(t *testing.T) {
		dev, err := e.api.CreateDevice(imei, "", "test-model", "teltonika")
		require.NoError(t, err)
		assert.Equal(t, imei, dev["imei"])
	})

	// Read
	t.Run("read", func(t *testing.T) {
		dev, err := e.api.GetDevice(imei)
		require.NoError(t, err)
		assert.Equal(t, imei, dev["imei"])
		assert.Equal(t, "active", dev["status"])
	})

	// Suspend
	t.Run("suspend", func(t *testing.T) {
		resp, err := e.api.Raw("POST", "/api/v1/devices/"+imei+"/suspend", nil)
		require.NoError(t, err)
		resp.Body.Close()
		assert.Equal(t, 200, resp.StatusCode)

		dev, err := e.api.GetDevice(imei)
		require.NoError(t, err)
		assert.Equal(t, "suspended", dev["status"])
	})

	// Reactivate
	t.Run("reactivate", func(t *testing.T) {
		resp, err := e.api.Raw("POST", "/api/v1/devices/"+imei+"/reactivate", nil)
		require.NoError(t, err)
		resp.Body.Close()
		assert.Equal(t, 200, resp.StatusCode)

		dev, err := e.api.GetDevice(imei)
		require.NoError(t, err)
		assert.Equal(t, "active", dev["status"])
	})

	// Delete (soft)
	t.Run("delete", func(t *testing.T) {
		err := e.api.DeleteDevice(imei)
		require.NoError(t, err)
	})
}

func TestDeviceProvisioning_LuhnValidation(t *testing.T) {
	e := setup(t)

	// Invalid Luhn IMEI should be rejected
	t.Run("invalid_luhn_rejected", func(t *testing.T) {
		_, err := e.api.CreateDevice("123456789012345", "", "test-model", "teltonika")
		assert.Error(t, err, "invalid Luhn IMEI should be rejected")
	})
}
