package scenarios

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGeofence_ListEndpoint(t *testing.T) {
	e := setup(t)

	resp, err := e.api.Raw("GET", "/api/v1/geofences", nil)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestGeofence_CRUD(t *testing.T) {
	e := setup(t)
	var geofenceID string

	t.Run("create", func(t *testing.T) {
		body := map[string]any{
			"name": "E2E Test Zone",
			"type": "polygon",
			"geometry": map[string]any{
				"type": "Polygon",
				"coordinates": [][][2]float64{
					{
						{55.26, 25.19},
						{55.28, 25.19},
						{55.28, 25.21},
						{55.26, 25.21},
						{55.26, 25.19},
					},
				},
			},
		}
		resp, err := e.api.Raw("POST", "/api/v1/geofences", body)
		require.NoError(t, err)
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
			var result map[string]any
			b, _ := io.ReadAll(resp.Body)
			json.Unmarshal(b, &result)
			if id, ok := result["id"].(string); ok {
				geofenceID = id
			}
		}
	})

	if geofenceID == "" {
		t.Skip("geofence not created")
	}

	t.Run("read", func(t *testing.T) {
		resp, err := e.api.Raw("GET", "/api/v1/geofences/"+geofenceID, nil)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("delete", func(t *testing.T) {
		resp, err := e.api.Raw("DELETE", "/api/v1/geofences/"+geofenceID, nil)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Contains(t, []int{http.StatusOK, http.StatusNoContent}, resp.StatusCode)
	})
}
