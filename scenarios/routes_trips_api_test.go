package scenarios

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRoutes_HistoryEndpoint(t *testing.T) {
	e := setup(t)

	// Use a known production IMEI that has data
	resp, err := e.api.Raw("GET", "/api/v1/routes/history/864636066073087?from=2026-04-01T00:00:00Z&to=2026-04-16T23:59:59Z", nil)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestTrips_ListEndpoint(t *testing.T) {
	e := setup(t)

	resp, err := e.api.Raw("GET", "/api/v1/trips/864636066073087", nil)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestTrips_MonthFilter(t *testing.T) {
	e := setup(t)

	resp, err := e.api.Raw("GET", "/api/v1/trips/864636066073087?month=2026-04", nil)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}
