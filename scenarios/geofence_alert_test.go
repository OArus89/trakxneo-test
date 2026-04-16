package scenarios

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestGeofenceAlert creates a geofence, sends a device inside it, verifies alert fires.
func TestGeofenceAlert(t *testing.T) {
	e := setup(t)

	imei := os.Getenv("TEST_IMEI_TELTONIKA")
	if imei == "" {
		t.Skip("TEST_IMEI_TELTONIKA not set")
	}
	orgID := getOrgID(t, e)

	// 1. Create a test geofence around a known point
	centerLat, centerLon := 25.2500, 55.3000
	geofenceBody := map[string]any{
		"name":   "E2E Alert Zone",
		"org_id": orgID,
		"type":   "polygon",
		"geometry": map[string]any{
			"type": "Polygon",
			"coordinates": [][][2]float64{
				{
					{centerLon - 0.005, centerLat - 0.005},
					{centerLon + 0.005, centerLat - 0.005},
					{centerLon + 0.005, centerLat + 0.005},
					{centerLon - 0.005, centerLat + 0.005},
					{centerLon - 0.005, centerLat - 0.005},
				},
			},
		},
	}
	resp, err := e.api.Raw("POST", "/api/v1/geofences", geofenceBody)
	require.NoError(t, err)
	defer resp.Body.Close()

	var gfResult map[string]any
	if resp.StatusCode < 300 {
		b, _ := io.ReadAll(resp.Body)
		json.Unmarshal(b, &gfResult)
	}
	geofenceID, _ := gfResult["id"].(string)
	if geofenceID != "" {
		defer func() {
			e.api.Raw("DELETE", "/api/v1/geofences/"+geofenceID, nil)
		}()
	}

	// Wait for Tile38 to sync the geofence
	time.Sleep(3 * time.Second)

	// 2. Send device OUTSIDE the geofence first
	session, err := e.gateway.ConnectTeltonika(imei)
	require.NoError(t, err)
	defer session.Close()

	err = session.SendCodec8(25.2000, 55.2500, 40, 90, true, true, 13800, 4100, 250000)
	require.NoError(t, err)
	time.Sleep(3 * time.Second)

	// 3. Send device INSIDE the geofence
	err = session.SendCodec8(centerLat, centerLon, 30, 90, true, true, 13800, 4100, 251000)
	require.NoError(t, err)

	// 4. Check for geofence event
	t.Run("geofence_event_created", func(t *testing.T) {
		waitFor(t, 20*time.Second, func() bool {
			resp, err := e.api.Raw("GET", "/api/v1/geofence-events?org_id="+orgID+"&device_id="+imei+"&from="+time.Now().Add(-5*time.Minute).Format(time.RFC3339), nil)
			if err != nil {
				return false
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				return false
			}
			b, _ := io.ReadAll(resp.Body)
			var events []map[string]any
			json.Unmarshal(b, &events)
			return len(events) > 0
		})
	})
}
