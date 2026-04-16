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

	resp, err := e.api.Raw("GET", "/api/v1/geofence-events?org_id="+getOrgID(t, e)+"&from=2026-04-01T00:00:00Z&to=2026-04-30T23:59:59Z", nil)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Less(t, resp.StatusCode, 500, "should not return server error")
}

func TestAlerts_SOSEvents(t *testing.T) {
	e := setup(t)

	resp, err := e.api.Raw("GET", "/api/v1/sos-events?org_id="+getOrgID(t, e), nil)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Less(t, resp.StatusCode, 500, "should not return server error")
}

func TestAlerts_AuditTrail(t *testing.T) {
	e := setup(t)

	resp, err := e.api.Raw("GET", "/api/v1/audit?org_id="+getOrgID(t, e), nil)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Less(t, resp.StatusCode, 500, "should not return server error")
}

// getOrgID fetches the first available org ID.
func getOrgID(t *testing.T, e *testEnv) string {
	t.Helper()
	if e.orgID != "" {
		return e.orgID
	}
	resp, err := e.api.Raw("GET", "/api/v1/admin/organizations", nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	var result struct {
		Organizations []struct {
			ID string `json:"id"`
		} `json:"organizations"`
	}
	require.NoError(t, decodeBody(resp.Body, &result))
	require.NotEmpty(t, result.Organizations, "need at least one org")
	e.orgID = result.Organizations[0].ID
	return e.orgID
}
