package scenarios

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestVehicleCalendars_CRUD tests vehicle calendar holds: create, list, overlap, delete.
func TestVehicleCalendars_CRUD(t *testing.T) {
	e := setup(t)
	orgID := getOrgID(t, e)

	// Setup vehicle
	r := apiCreate(t, e, "/api/v1/rental/vehicle-classes", map[string]any{"name": "Cal Test", "code": "CAL-CLS", "vehicle_type": "car", "org_id": orgID})
	classID, _ := r["id"].(string)
	r = apiCreate(t, e, "/api/v1/rental/vehicles", map[string]any{
		"org_id": orgID, "vehicle_class_id": classID,
		"plate": "CAL-TEST-1", "make": "Test", "model": "C", "status": "available",
	})
	vehicleID, _ := r["id"].(string)
	if vehicleID == "" {
		t.Skip("could not create vehicle")
	}

	now := time.Now()
	var holdID string

	t.Run("create_hold", func(t *testing.T) {
		body := map[string]any{
			"start_at": now.Add(24 * time.Hour).Format(time.RFC3339),
			"end_at":   now.Add(48 * time.Hour).Format(time.RFC3339),
			"reason":   "maintenance",
			"notes":    "E2E test hold",
		}
		resp, err := e.api.Raw("POST", "/api/v1/rental/vehicles/"+vehicleID+"/calendar", body)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Less(t, resp.StatusCode, 300)

		var cr map[string]any
		b, _ := io.ReadAll(resp.Body)
		json.Unmarshal(b, &cr)
		holdID, _ = cr["id"].(string)
		assert.Equal(t, "maintenance", cr["reason"])
	})

	t.Run("list_holds", func(t *testing.T) {
		resp, err := e.api.Raw("GET", "/api/v1/rental/vehicles/"+vehicleID+"/calendar", nil)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("overlap_rejected", func(t *testing.T) {
		body := map[string]any{
			"start_at": now.Add(30 * time.Hour).Format(time.RFC3339),
			"end_at":   now.Add(40 * time.Hour).Format(time.RFC3339),
			"reason":   "service",
		}
		resp, err := e.api.Raw("POST", "/api/v1/rental/vehicles/"+vehicleID+"/calendar", body)
		require.NoError(t, err)
		resp.Body.Close()
		assert.Equal(t, http.StatusConflict, resp.StatusCode, "overlapping hold should be rejected")
	})

	t.Run("delete_hold", func(t *testing.T) {
		if holdID == "" {
			t.Skip("no hold")
		}
		resp, err := e.api.Raw("DELETE", "/api/v1/rental/calendar/"+holdID, nil)
		require.NoError(t, err)
		resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})
}

// TestBookingOptions_CRUD tests booking options: add, list, delete.
func TestBookingOptions_CRUD(t *testing.T) {
	e := setup(t)
	orgID := getOrgID(t, e)

	resp, err := e.api.Raw("GET", "/api/v1/rental/bookings", nil)
	require.NoError(t, err)
	resp.Body.Close()
	if resp.StatusCode == 403 || resp.StatusCode == 404 {
		t.Skip("Rental not enabled")
	}

	// Create minimal booking chain
	r := apiCreate(t, e, "/api/v1/rental/vehicle-classes", map[string]any{"name": "Opt Test", "code": "OPT-CLS", "vehicle_type": "car", "org_id": orgID})
	classID, _ := r["id"].(string)
	r = apiCreate(t, e, "/api/v1/rental/vehicles", map[string]any{
		"org_id": orgID, "vehicle_class_id": classID,
		"plate": "OPT-TEST-1", "make": "T", "model": "O", "status": "available",
	})
	vID, _ := r["id"].(string)
	r = apiCreate(t, e, "/api/v1/rental/persons", map[string]any{
		"phone": "+971500077001", "first_name": "Opt", "last_name": "Test",
	})
	pID, _ := r["id"].(string)
	r = apiCreate(t, e, "/api/v1/rental/customers", map[string]any{
		"org_id": orgID, "customer_type": "individual", "person_id": pID,
	})
	cID, _ := r["id"].(string)
	r = apiCreate(t, e, "/api/v1/rental/bookings", map[string]any{
		"org_id": orgID, "vehicle_id": vID, "customer_id": cID,
	})
	bookingID, _ := r["id"].(string)
	if bookingID == "" {
		t.Skip("could not create booking")
	}

	var optionID string

	t.Run("add_option", func(t *testing.T) {
		body := map[string]any{
			"option_type": "child_seat",
			"option_name": "Infant seat",
			"quantity":    1,
			"unit_price":  25.0,
			"total_price": 25.0,
		}
		resp, err := e.api.Raw("POST", "/api/v1/rental/bookings/"+bookingID+"/options", body)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Less(t, resp.StatusCode, 300)

		var cr map[string]any
		b, _ := io.ReadAll(resp.Body)
		json.Unmarshal(b, &cr)
		optionID, _ = cr["id"].(string)
		assert.Equal(t, "child_seat", cr["option_type"])
	})

	t.Run("list_options", func(t *testing.T) {
		resp, err := e.api.Raw("GET", "/api/v1/rental/bookings/"+bookingID+"/options", nil)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("delete_option", func(t *testing.T) {
		if optionID == "" {
			t.Skip("no option")
		}
		resp, err := e.api.Raw("DELETE", "/api/v1/rental/booking-options/"+optionID, nil)
		require.NoError(t, err)
		resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})
}

// TestViolations_CRUD tests violations: create, list, acknowledge.
func TestViolations_CRUD(t *testing.T) {
	e := setup(t)
	orgID := getOrgID(t, e)

	resp, err := e.api.Raw("GET", "/api/v1/rental/violations", nil)
	require.NoError(t, err)
	resp.Body.Close()
	if resp.StatusCode == 403 || resp.StatusCode == 404 {
		t.Skip("Rental not enabled")
	}

	// Create booking for violation
	r := apiCreate(t, e, "/api/v1/rental/vehicle-classes", map[string]any{"name": "Viol Test", "code": "VIOL-CLS", "vehicle_type": "car", "org_id": orgID})
	classID, _ := r["id"].(string)
	r = apiCreate(t, e, "/api/v1/rental/vehicles", map[string]any{
		"org_id": orgID, "vehicle_class_id": classID,
		"plate": "VIOL-1", "make": "T", "model": "V", "status": "available",
	})
	vID, _ := r["id"].(string)
	r = apiCreate(t, e, "/api/v1/rental/persons", map[string]any{
		"phone": "+971500077002", "first_name": "Viol", "last_name": "Test",
	})
	pID, _ := r["id"].(string)
	r = apiCreate(t, e, "/api/v1/rental/customers", map[string]any{
		"org_id": orgID, "customer_type": "individual", "person_id": pID,
	})
	cID, _ := r["id"].(string)
	r = apiCreate(t, e, "/api/v1/rental/bookings", map[string]any{
		"org_id": orgID, "vehicle_id": vID, "customer_id": cID,
	})
	bookingID, _ := r["id"].(string)
	if bookingID == "" || vID == "" {
		t.Skip("could not create booking")
	}

	var violationID string

	t.Run("create_violation", func(t *testing.T) {
		body := map[string]any{
			"booking_id":     bookingID,
			"vehicle_id":     vID,
			"violation_type": "speed_limit",
			"severity":       "high",
			"occurred_at":    time.Now().Format(time.RFC3339),
			"location_lat":   25.2048,
			"location_lon":   55.2708,
			"fine_amount":    500.0,
			"fine_currency":  "AED",
			"details":        map[string]any{"recorded_speed": 95, "limit": 60},
		}
		resp, err := e.api.Raw("POST", "/api/v1/rental/violations", body)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Less(t, resp.StatusCode, 300)

		var cr map[string]any
		b, _ := io.ReadAll(resp.Body)
		json.Unmarshal(b, &cr)
		violationID, _ = cr["id"].(string)
		assert.Equal(t, "speed_limit", cr["violation_type"])
		assert.Equal(t, "pending", cr["fine_status"])
	})

	t.Run("list_violations", func(t *testing.T) {
		resp, err := e.api.Raw("GET", "/api/v1/rental/violations?booking_id="+bookingID, nil)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("acknowledge_violation", func(t *testing.T) {
		if violationID == "" {
			t.Skip("no violation")
		}
		body := map[string]any{
			"fine_status": "charged",
			"notes":       "Confirmed by E2E test",
		}
		resp, err := e.api.Raw("POST", "/api/v1/rental/violations/"+violationID+"/acknowledge", body)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Less(t, resp.StatusCode, 300)

		var cr map[string]any
		b, _ := io.ReadAll(resp.Body)
		json.Unmarshal(b, &cr)
		assert.Equal(t, "charged", cr["fine_status"])
		assert.NotNil(t, cr["acknowledged_at"])
	})
}
