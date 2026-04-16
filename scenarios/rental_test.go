package scenarios

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRental_StaffEndpoints verifies the rental module staff API.
func TestRental_StaffEndpoints(t *testing.T) {
	e := setup(t)

	endpoints := []struct {
		name   string
		method string
		path   string
	}{
		{"list_vehicles", "GET", "/api/v1/rental/vehicles"},
		{"list_customers", "GET", "/api/v1/rental/customers"},
		{"list_bookings", "GET", "/api/v1/rental/bookings"},
		{"list_vehicle_classes", "GET", "/api/v1/rental/vehicle-classes"},
		{"list_tariffs", "GET", "/api/v1/rental/tariffs"},
		{"list_parking_lots", "GET", "/api/v1/rental/parking-lots"},
		{"list_zones", "GET", "/api/v1/rental/zones"},
		{"list_promo_codes", "GET", "/api/v1/rental/promo-codes"},
		{"list_protection_plans", "GET", "/api/v1/rental/protection-plans"},
	}

	for _, ep := range endpoints {
		t.Run(ep.name, func(t *testing.T) {
			resp, err := e.api.Raw(ep.method, ep.path, nil)
			require.NoError(t, err)
			defer resp.Body.Close()
			// 200 = working, 403 = no permission (feature flag off), both acceptable
			assert.Less(t, resp.StatusCode, 500, "should not return server error for %s", ep.path)
		})
	}
}

// TestRental_CustomerOAuthEndpoint verifies the OAuth token endpoint exists.
func TestRental_CustomerOAuthEndpoint(t *testing.T) {
	e := setup(t)

	// Should return 400 (missing params), not 404 (endpoint missing)
	resp, err := e.api.Raw("POST", "/oauth/token", map[string]string{})
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.NotEqual(t, http.StatusNotFound, resp.StatusCode, "OAuth endpoint should exist")
}
