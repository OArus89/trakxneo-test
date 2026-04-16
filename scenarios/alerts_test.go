package scenarios

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAlerts_ListEndpoint(t *testing.T) {
	e := setup(t)

	resp, err := e.api.Raw("GET", "/api/v1/alerts", nil)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestAlerts_GeofenceEvents(t *testing.T) {
	e := setup(t)

	resp, err := e.api.Raw("GET", "/api/v1/geofence-events", nil)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestAlerts_SOSEvents(t *testing.T) {
	e := setup(t)

	resp, err := e.api.Raw("GET", "/api/v1/sos-events", nil)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestAlerts_AuditTrail(t *testing.T) {
	e := setup(t)

	resp, err := e.api.Raw("GET", "/api/v1/audit", nil)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}
