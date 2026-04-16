package scenarios

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTripHybridGPSCorrection verifies that in hybrid mode,
// ignition OFF + speed > threshold does NOT end the trip (GPS overrides ignition).
func TestTripHybridGPSCorrection(t *testing.T) {
	e := setup(t)

	imei := os.Getenv("TEST_IMEI_TELTONIKA")
	if imei == "" {
		t.Skip("TEST_IMEI_TELTONIKA not set")
	}

	session, err := e.gateway.ConnectTeltonika(imei)
	require.NoError(t, err)
	defer session.Close()

	// 1. Start trip: ignition ON, moving
	err = session.SendCodec8(25.1500, 55.2000, 50, 90, true, true, 13800, 4100, 400000)
	require.NoError(t, err)
	time.Sleep(3 * time.Second)

	err = session.SendCodec8(25.1510, 55.2020, 60, 90, true, true, 13800, 4100, 400500)
	require.NoError(t, err)

	// Wait for trip to start
	waitFor(t, 15*time.Second, func() bool {
		ops, _ := e.dragonfly.DeviceOpState(imei)
		if ops == nil {
			return false
		}
		tid, _ := ops["active_trip_id"].(string)
		return tid != ""
	})

	ops, _ := e.dragonfly.DeviceOpState(imei)
	tripID, _ := ops["active_trip_id"].(string)
	require.NotEmpty(t, tripID)

	// 2. Send: ignition OFF but speed > threshold (GPS correction scenario)
	// In hybrid mode, trip should continue because GPS says we're still moving
	err = session.SendCodec8(25.1520, 55.2040, 45, 85, false, true, 12400, 4000, 401000)
	require.NoError(t, err)
	time.Sleep(5 * time.Second)

	t.Run("trip_still_active", func(t *testing.T) {
		ops, err := e.dragonfly.DeviceOpState(imei)
		require.NoError(t, err)
		currentTrip, _ := ops["active_trip_id"].(string)
		assert.Equal(t, tripID, currentTrip, "trip should still be active (GPS correction)")
	})

	// 3. Now actually stop: ignition OFF + speed 0 → trip should end
	err = session.SendCodec8(25.1530, 55.2060, 0, 0, false, false, 12400, 4000, 401500)
	require.NoError(t, err)

	t.Run("trip_ends_on_full_stop", func(t *testing.T) {
		waitFor(t, 20*time.Second, func() bool {
			trip, err := e.pg.TripByID(tripID)
			if err != nil {
				return false
			}
			return trip["status"] == "completed"
		})
	})
}

// TestTripDiscardShort verifies trips under 1000m are discarded.
func TestTripDiscardShort(t *testing.T) {
	e := setup(t)

	imei := os.Getenv("TEST_IMEI_TELTONIKA")
	if imei == "" {
		t.Skip("TEST_IMEI_TELTONIKA not set")
	}

	session, err := e.gateway.ConnectTeltonika(imei)
	require.NoError(t, err)
	defer session.Close()

	// Very short trip: ~100m
	err = session.SendCodec8(25.1600, 55.2100, 30, 90, true, true, 13800, 4100, 500000)
	require.NoError(t, err)
	time.Sleep(3 * time.Second)

	// Get trip ID if started
	ops, _ := e.dragonfly.DeviceOpState(imei)
	tripID, _ := ops["active_trip_id"].(string)

	// Immediate stop (tiny distance)
	err = session.SendCodec8(25.1601, 55.2101, 0, 0, false, false, 12400, 4000, 500050)
	require.NoError(t, err)

	if tripID == "" {
		t.Skip("no trip started for short distance test")
	}

	t.Run("short_trip_discarded", func(t *testing.T) {
		waitFor(t, 20*time.Second, func() bool {
			trip, err := e.pg.TripByID(tripID)
			if err != nil {
				return false
			}
			s, _ := trip["status"].(string)
			return s == "discarded" || s == "completed"
		})

		trip, err := e.pg.TripByID(tripID)
		require.NoError(t, err)
		// Short trips (<1000m) should be discarded
		assert.Equal(t, "discarded", trip["status"], "trip under 1000m should be discarded")
	})
}
