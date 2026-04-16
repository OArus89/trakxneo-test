package scenarios

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRuptelaProtocol verifies the Ruptela TCP path works end-to-end.
func TestRuptelaProtocol(t *testing.T) {
	e := setup(t)

	imei := os.Getenv("TEST_IMEI_RUPTELA")
	if imei == "" {
		t.Skip("TEST_IMEI_RUPTELA not set")
	}

	session, err := e.gateway.ConnectRuptela(imei)
	require.NoError(t, err, "should connect to Ruptela port")
	defer session.Close()

	beforeSend := time.Now()

	err = session.SendRecord(25.1800, 55.2500, 45, 180, true, true, 13600, 4000, 300000)
	require.NoError(t, err, "should send Ruptela packet")

	t.Run("dragonfly_state_updated", func(t *testing.T) {
		waitFor(t, 15*time.Second, func() bool {
			state, err := e.dragonfly.DeviceState(imei)
			if err != nil {
				return false
			}
			if lat, ok := state["lat"].(float64); ok {
				return lat > 25.0 && lat < 26.0
			}
			return false
		})
	})

	t.Run("clickhouse_has_point", func(t *testing.T) {
		waitFor(t, 15*time.Second, func() bool {
			count, err := e.ch.TelemetryCount(imei, beforeSend)
			return err == nil && count > 0
		})

		point, err := e.ch.LatestTelemetry(imei)
		require.NoError(t, err)
		assert.InDelta(t, 25.18, point["latitude"], 0.01)
	})
}
