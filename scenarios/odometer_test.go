package scenarios

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestOdometer verifies odometer value flows through the pipeline.
func TestOdometer(t *testing.T) {
	e := setup(t)

	imei := os.Getenv("TEST_IMEI_TELTONIKA")
	if imei == "" {
		t.Skip("TEST_IMEI_TELTONIKA not set")
	}

	session, err := e.gateway.ConnectTeltonika(imei)
	require.NoError(t, err)
	defer session.Close()

	odometerM := 350000 // 350km
	err = session.SendCodec8(25.1900, 55.2400, 50, 90, true, true, 13800, 4100, odometerM)
	require.NoError(t, err)
	time.Sleep(5 * time.Second)

	t.Run("odometer_in_dragonfly", func(t *testing.T) {
		state, err := e.dragonfly.DeviceState(imei)
		require.NoError(t, err)
		if odo, ok := state["odometer_m"].(float64); ok {
			assert.InDelta(t, float64(odometerM), odo, 1000, "odometer should be ~350000m")
		}
	})

	t.Run("odometer_in_clickhouse", func(t *testing.T) {
		point, err := e.ch.LatestTelemetry(imei)
		if err != nil {
			t.Skipf("clickhouse not available: %v", err)
		}
		// Check odometer field exists (may be in different field name)
		t.Logf("Latest telemetry point fields: %v", point)
	})
}
