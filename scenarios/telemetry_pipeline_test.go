package scenarios

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTelemetryPipeline verifies the full path:
// TCP packet → Gateway → Kafka → Telemetry Processor → ClickHouse + Dragonfly
func TestTelemetryPipeline(t *testing.T) {
	e := setup(t)

	// Test IMEI from reserved range
	imei := "999000000000001"
	lat, lon := 25.2048, 55.2708 // Dubai
	speed := 60
	heading := 90

	// 1. Connect to gateway as Teltonika device
	session, err := e.gateway.ConnectTeltonika(imei)
	require.NoError(t, err, "should connect to gateway")
	defer session.Close()

	beforeSend := time.Now()

	// 2. Send a Codec 8 packet: moving, ignition ON
	err = session.SendCodec8(lat, lon, speed, heading,
		true,   // ignition
		true,   // movement
		13800,  // ext voltage (engine running)
		4100,   // battery
		150000, // odometer 150km
	)
	require.NoError(t, err, "should send telemetry packet")

	// 3. Verify Dragonfly device:state updated
	t.Run("dragonfly_state", func(t *testing.T) {
		waitFor(t, 10*time.Second, func() bool {
			state, err := e.dragonfly.DeviceState(imei)
			if err != nil {
				return false
			}
			// Check lat/lon are close to what we sent
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

	// 4. Verify Dragonfly device:opstate updated
	t.Run("dragonfly_opstate", func(t *testing.T) {
		waitFor(t, 10*time.Second, func() bool {
			ops, err := e.dragonfly.DeviceOpState(imei)
			if err != nil {
				return false
			}
			conn, _ := ops["connectivity"].(string)
			return conn == "online"
		})

		ops, err := e.dragonfly.DeviceOpState(imei)
		require.NoError(t, err)
		assert.Equal(t, "online", ops["connectivity"])
	})

	// 5. Verify ClickHouse received the telemetry point
	t.Run("clickhouse_telemetry", func(t *testing.T) {
		waitFor(t, 15*time.Second, func() bool {
			count, err := e.ch.TelemetryCount(imei, beforeSend)
			return err == nil && count > 0
		})

		point, err := e.ch.LatestTelemetry(imei)
		require.NoError(t, err)
		assert.InDelta(t, lat, point["latitude"], 0.01)
		assert.InDelta(t, lon, point["longitude"], 0.01)
		assert.Equal(t, true, point["ignition"])
	})
}
