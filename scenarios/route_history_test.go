package scenarios

import (
	"encoding/json"
	"io"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRouteHistory sends multiple GPS points and verifies they appear in route history API.
func TestRouteHistory(t *testing.T) {
	e := setup(t)

	imei := os.Getenv("TEST_IMEI_TELTONIKA")
	if imei == "" {
		t.Skip("TEST_IMEI_TELTONIKA not set")
	}

	session, err := e.gateway.ConnectTeltonika(imei)
	require.NoError(t, err)
	defer session.Close()

	beforeSend := time.Now().UTC()

	// Send 5 points along a route
	points := [][2]float64{
		{25.2100, 55.2700},
		{25.2110, 55.2720},
		{25.2120, 55.2740},
		{25.2130, 55.2760},
		{25.2140, 55.2780},
	}
	for _, p := range points {
		err = session.SendCodec8(p[0], p[1], 40, 90, true, true, 13800, 4100, 220000)
		require.NoError(t, err)
		time.Sleep(2 * time.Second)
	}

	// Wait for ClickHouse to flush
	time.Sleep(8 * time.Second)

	t.Run("route_api_returns_points", func(t *testing.T) {
		from := beforeSend.Format(time.RFC3339)
		to := time.Now().UTC().Format(time.RFC3339)
		resp, err := e.api.Raw("GET", "/api/v1/routes/history/"+imei+"?from="+from+"&to="+to, nil)
		require.NoError(t, err)
		defer resp.Body.Close()

		b, _ := io.ReadAll(resp.Body)
		var result struct {
			Points []any `json:"points"`
		}
		json.Unmarshal(b, &result)
		assert.GreaterOrEqual(t, len(result.Points), 3, "should return at least 3 route points")
	})

	t.Run("clickhouse_has_all_points", func(t *testing.T) {
		count, err := e.ch.TelemetryCount(imei, beforeSend)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, count, uint64(5), "ClickHouse should have at least 5 points")
	})
}
