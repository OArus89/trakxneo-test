package scenarios

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTripLifecycle verifies:
// 1. Ignition ON + speed → trip starts
// 2. Drive along a route → trip accumulates distance
// 3. Ignition OFF + speed=0 → trip ends immediately
// 4. Trip appears in PG with status=completed
// 5. Route polyline is archived
func TestTripLifecycle(t *testing.T) {
	e := setup(t)
	imei := "999000000000002"

	session, err := e.gateway.ConnectTeltonika(imei)
	require.NoError(t, err)
	defer session.Close()

	// 1. Start engine — ignition ON, speed 0 (cranking)
	err = session.SendCodec8(25.2000, 55.2700, 0, 0, true, false, 11200, 4100, 200000)
	require.NoError(t, err)
	time.Sleep(2 * time.Second)

	// 2. Start moving — speed > 10 km/h triggers trip start
	err = session.SendCodec8(25.2010, 55.2710, 40, 90, true, true, 13800, 4100, 200100)
	require.NoError(t, err)

	// Wait for trip to start in Dragonfly
	t.Run("trip_starts", func(t *testing.T) {
		waitFor(t, 15*time.Second, func() bool {
			ops, err := e.dragonfly.DeviceOpState(imei)
			if err != nil {
				return false
			}
			motion, _ := ops["motion"].(string)
			tripID, _ := ops["active_trip_id"].(string)
			return motion == "moving" && tripID != ""
		})
	})

	// 3. Drive a few points to build distance (>1000m for validation)
	points := [][4]float64{
		{25.2020, 55.2720, 60, 90},
		{25.2030, 55.2740, 65, 85},
		{25.2040, 55.2760, 55, 80},
		{25.2050, 55.2780, 50, 75},
		{25.2060, 55.2800, 45, 70},
	}
	for _, p := range points {
		err = session.SendCodec8(p[0], p[1], int(p[2]), int(p[3]), true, true, 13800, 4100, 200500)
		require.NoError(t, err)
		time.Sleep(2 * time.Second)
	}

	// Get active trip ID
	ops, err := e.dragonfly.DeviceOpState(imei)
	require.NoError(t, err)
	tripID, _ := ops["active_trip_id"].(string)
	require.NotEmpty(t, tripID, "should have active trip ID")

	// 4. Stop — ignition OFF, speed 0 → immediate trip end
	err = session.SendCodec8(25.2070, 55.2820, 0, 0, false, false, 12400, 4000, 201000)
	require.NoError(t, err)

	// Wait for trip to complete in PostgreSQL
	t.Run("trip_completes", func(t *testing.T) {
		waitFor(t, 20*time.Second, func() bool {
			trip, err := e.pg.TripByID(tripID)
			if err != nil {
				return false
			}
			return trip["status"] == "completed"
		})

		trip, err := e.pg.TripByID(tripID)
		require.NoError(t, err)
		assert.Equal(t, "completed", trip["status"])
		assert.Equal(t, "ignition_off", trip["end_reason"])
	})

	// 5. Route polyline should be archived
	t.Run("polyline_archived", func(t *testing.T) {
		trip, err := e.pg.TripByID(tripID)
		require.NoError(t, err)
		assert.True(t, trip["has_polyline"].(bool), "route polyline should be archived")
	})

	// 6. Trip visible via API
	t.Run("trip_in_api", func(t *testing.T) {
		trips, err := e.api.GetTrips(imei)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(trips), 1)
	})
}
