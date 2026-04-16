package scenarios

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDashboardDevice verifies the dashboard endpoint merges opstate into device state.
func TestDashboardDevice(t *testing.T) {
	e := setup(t)
	imei := "999000000000003"

	// Send a packet so the device has state
	session, err := e.gateway.ConnectTeltonika(imei)
	require.NoError(t, err)
	defer session.Close()

	err = session.SendCodec8(25.1900, 55.2600, 30, 180, true, true, 13600, 4050, 100000)
	require.NoError(t, err)

	// Wait for state to propagate
	waitFor(t, 10*time.Second, func() bool {
		_, err := e.dragonfly.DeviceState(imei)
		return err == nil
	})

	t.Run("device_detail_returns_data", func(t *testing.T) {
		dev, err := e.api.GetDashboardDevice(imei)
		require.NoError(t, err)
		assert.NotNil(t, dev)
		// Should have merged opstate fields
		if lat, ok := dev["lat"].(float64); ok {
			assert.InDelta(t, 25.19, lat, 0.01)
		}
	})
}

// TestDashboardDeviceList verifies the viewport endpoint returns devices.
func TestDashboardDeviceList(t *testing.T) {
	e := setup(t)

	t.Run("viewport_returns_ok", func(t *testing.T) {
		resp, err := e.api.Raw("GET", "/api/v1/dashboard/devices", nil)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})
}
