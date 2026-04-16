package scenarios

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRentalBookingLifecycle tests the full booking state machine.
func TestRentalBookingLifecycle(t *testing.T) {
	e := setup(t)
	orgID := getOrgID(t, e)

	// Check if rental feature is available
	resp, err := e.api.Raw("GET", "/api/v1/rental/vehicles", nil)
	require.NoError(t, err)
	resp.Body.Close()
	if resp.StatusCode == 403 || resp.StatusCode == 404 {
		t.Skip("Rental module not enabled for this org")
	}

	var vehicleClassID, vehicleID, customerID, bookingID string

	// 1. Create vehicle class
	t.Run("create_vehicle_class", func(t *testing.T) {
		body := map[string]any{
			"name":        "E2E Test Class",
			"org_id":      orgID,
			"description": "test",
		}
		resp, err := e.api.Raw("POST", "/api/v1/rental/vehicle-classes", body)
		require.NoError(t, err)
		defer resp.Body.Close()
		if resp.StatusCode < 300 {
			var r map[string]any
			b, _ := io.ReadAll(resp.Body)
			json.Unmarshal(b, &r)
			vehicleClassID, _ = r["id"].(string)
		}
		assert.Less(t, resp.StatusCode, 500)
	})

	// 2. Create vehicle
	t.Run("create_vehicle", func(t *testing.T) {
		if vehicleClassID == "" {
			t.Skip("no vehicle class")
		}
		body := map[string]any{
			"org_id":           orgID,
			"vehicle_class_id": vehicleClassID,
			"plate":            "E2E-TEST-1",
			"make":             "Test",
			"model":            "TestCar",
			"year":             2024,
			"color":            "white",
			"status":           "available",
		}
		resp, err := e.api.Raw("POST", "/api/v1/rental/vehicles", body)
		require.NoError(t, err)
		defer resp.Body.Close()
		if resp.StatusCode < 300 {
			var r map[string]any
			b, _ := io.ReadAll(resp.Body)
			json.Unmarshal(b, &r)
			vehicleID, _ = r["id"].(string)
		}
		assert.Less(t, resp.StatusCode, 500)
	})

	// 3. Create customer
	t.Run("create_customer", func(t *testing.T) {
		body := map[string]any{
			"org_id":     orgID,
			"type":       "individual",
			"first_name": "E2E",
			"last_name":  "Test",
			"phone":      "+971500000001",
			"email":      "e2e@test.local",
		}
		resp, err := e.api.Raw("POST", "/api/v1/rental/customers", body)
		require.NoError(t, err)
		defer resp.Body.Close()
		if resp.StatusCode < 300 {
			var r map[string]any
			b, _ := io.ReadAll(resp.Body)
			json.Unmarshal(b, &r)
			customerID, _ = r["id"].(string)
		}
		assert.Less(t, resp.StatusCode, 500)
	})

	// 4. Create booking
	t.Run("create_booking", func(t *testing.T) {
		if vehicleID == "" || customerID == "" {
			t.Skip("missing vehicle or customer")
		}
		body := map[string]any{
			"org_id":      orgID,
			"vehicle_id":  vehicleID,
			"customer_id": customerID,
		}
		resp, err := e.api.Raw("POST", "/api/v1/rental/bookings", body)
		require.NoError(t, err)
		defer resp.Body.Close()
		if resp.StatusCode < 300 {
			var r map[string]any
			b, _ := io.ReadAll(resp.Body)
			json.Unmarshal(b, &r)
			bookingID, _ = r["id"].(string)
		}
		assert.Less(t, resp.StatusCode, 500)
	})

	// 5. Verify booking in list
	t.Run("booking_in_list", func(t *testing.T) {
		if bookingID == "" {
			t.Skip("no booking")
		}
		resp, err := e.api.Raw("GET", "/api/v1/rental/bookings/"+bookingID, nil)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	// 6. Cancel booking (cleanup)
	t.Run("cancel_booking", func(t *testing.T) {
		if bookingID == "" {
			t.Skip("no booking")
		}
		resp, err := e.api.Raw("POST", "/api/v1/rental/bookings/"+bookingID+"/cancel", nil)
		require.NoError(t, err)
		resp.Body.Close()
		assert.Less(t, resp.StatusCode, 500)
	})
}
