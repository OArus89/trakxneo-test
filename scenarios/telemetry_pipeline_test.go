package scenarios

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTelemetryPipeline verifies the full path:
// TCP packet → Gateway → Kafka → Telemetry Processor → ClickHouse + Dragonfly
func TestTelemetryPipeline(t *testing.T) {
	e := setup(t)

	imei := os.Getenv("TEST_IMEI_TELTONIKA")
	if imei == "" {
		t.Skip("TEST_IMEI_TELTONIKA not set — need a registered device IMEI")
	}

	lat, lon := 25.2048, 55.2708
	speed := 60
	heading := 90

	session, err := e.gateway.ConnectTeltonika(imei)
	require.NoError(t, err, "should connect to gateway")
	defer session.Close()

	beforeSend := time.Now()

	err = session.SendCodec8(lat, lon, speed, heading, true, true, 13800, 4100, 150000)
	require.NoError(t, err, "should send telemetry packet")

	t.Run("dragonfly_state", func(t *testing.T) {
		waitFor(t, 10*time.Second, func() bool {
			state, err := e.dragonfly.DeviceState(imei)
			if err != nil {
				return false
			}
			if stateLat, ok := state["lat"].(float64); ok {
				return stateLat > 25.0 && stateLat < 26.0
			}
			return false
		})

		state, err := e.dragonfly.DeviceState(imei)
		require.NoError(t, err)
		assert.InDelta(t, lat, state["lat"], 0.01)
		assert.InDelta(t, lon, state["lon"], 0.01)
	})

	t.Run("dragonfly_opstate", func(t *testing.T) {
		waitFor(t, 10*time.Second, func() bool {
			ops, err := e.dragonfly.DeviceOpState(imei)
			if err != nil {
				return false
			}
			conn, _ := ops["connectivity"].(string)
			return conn == "online"
		})
	})

	t.Run("clickhouse_telemetry", func(t *testing.T) {
		waitFor(t, 15*time.Second, func() bool {
			count, err := e.ch.TelemetryCount(imei, beforeSend)
			return err == nil && count > 0
		})

		point, err := e.ch.LatestTelemetry(imei)
		require.NoError(t, err)
		assert.InDelta(t, lat, point["latitude"], 0.01)
	})
}
