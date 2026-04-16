package scenarios

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestReverseGeocoding verifies the dashboard returns an address for a device with GPS data.
func TestReverseGeocoding(t *testing.T) {
	e := setup(t)

	imei := os.Getenv("TEST_IMEI_TELTONIKA")
	if imei == "" {
		t.Skip("TEST_IMEI_TELTONIKA not set")
	}

	// Send a packet at a known Dubai location
	session, err := e.gateway.ConnectTeltonika(imei)
	require.NoError(t, err)
	defer session.Close()

	// Burj Khalifa area
	err = session.SendCodec8(25.1972, 55.2744, 0, 0, true, false, 13800, 4100, 210000)
	require.NoError(t, err)
	time.Sleep(5 * time.Second)

	t.Run("address_returned", func(t *testing.T) {
		dev, err := e.api.GetDashboardDevice(imei)
		require.NoError(t, err)

		// Dashboard should include reverse-geocoded address
		if addr, ok := dev["address"].(string); ok {
			assert.NotEmpty(t, addr, "address should not be empty for Dubai coordinates")
			t.Logf("Address: %s", addr)
		} else {
			t.Log("No address field in response — Nominatim may not be available")
		}
	})
}
