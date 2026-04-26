package scenarios

import (
	"encoding/json"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBookingFullStateMachine tests: reserved → confirm → pickup → start → return → complete.
func TestBookingFullStateMachine(t *testing.T) {
	e := setup(t)
	orgID := getOrgID(t, e)

	// Skip if rental not enabled
	resp, err := e.api.Raw("GET", "/api/v1/rental/vehicles", nil)
	require.NoError(t, err)
	resp.Body.Close()
	if resp.StatusCode == 403 || resp.StatusCode == 404 {
		t.Skip("Rental module not enabled")
	}

	var vehicleClassID, vehicleID, personID, customerID, tariffID, bookingID string

	// Setup: vehicle class
	t.Run("setup_vehicle_class", func(t *testing.T) {
		body := map[string]any{"name": "SM Test Class", "code": "SM-CLASS", "vehicle_type": "car", "org_id": orgID}
		r := apiCreate(t, e, "/api/v1/rental/vehicle-classes", body)
		vehicleClassID, _ = r["id"].(string)
		require.NotEmpty(t, vehicleClassID)
	})

	// Setup: vehicle
	t.Run("setup_vehicle", func(t *testing.T) {
		body := map[string]any{
			"org_id": orgID, "vehicle_class_id": vehicleClassID, "vehicle_type": "car",
			"plate": "SM-TEST-1", "make": "Test", "model": "SM",
			"year": 2024, "color": "blue", "status": "available",
		}
		r := apiCreate(t, e, "/api/v1/rental/vehicles", body)
		vehicleID, _ = r["id"].(string)
		require.NotEmpty(t, vehicleID)
	})

	// Setup: person + customer
	t.Run("setup_customer", func(t *testing.T) {
		body := map[string]any{
			"phone": "+971500088001", "first_name": "SM", "last_name": "Test",
		}
		r := apiCreate(t, e, "/api/v1/rental/persons", body)
		personID, _ = r["id"].(string)

		body2 := map[string]any{
			"org_id": orgID, "customer_type": "individual", "person_id": personID,
		}
		r2 := apiCreate(t, e, "/api/v1/rental/customers", body2)
		customerID, _ = r2["id"].(string)
		require.NotEmpty(t, customerID)
	})

	// Setup: tariff
	t.Run("setup_tariff", func(t *testing.T) {
		body := map[string]any{
			"org_id": orgID, "name": "SM Test Tariff",
			"per_day": 100, "currency": "AED",
		}
		r := apiCreate(t, e, "/api/v1/rental/tariffs", body)
		tariffID, _ = r["id"].(string)
	})

	// 1. Create booking (→ reserved)
	t.Run("create_booking", func(t *testing.T) {
		body := map[string]any{
			"org_id": orgID, "vehicle_id": vehicleID,
			"customer_id": customerID, "tariff_id": tariffID,
			"pickup_scheduled_at": time.Now().Format(time.RFC3339),
			"return_scheduled_at":   time.Now().Add(24 * time.Hour).Format(time.RFC3339),
		}
		r := apiCreate(t, e, "/api/v1/rental/bookings", body)
		bookingID, _ = r["id"].(string)
		require.NotEmpty(t, bookingID)
		assert.Equal(t, "reserved", r["status"])
	})

	// 2. Confirm
	t.Run("confirm", func(t *testing.T) {
		requireBookingTransition(t, e, bookingID, "confirm", "confirmed")
	})

	// 3. Pickup
	t.Run("pickup", func(t *testing.T) {
		requireBookingTransition(t, e, bookingID, "pickup", "picked_up")
	})

	// 4. Start
	t.Run("start", func(t *testing.T) {
		requireBookingTransition(t, e, bookingID, "start", "in_use")
	})

	// 5. Return
	t.Run("return", func(t *testing.T) {
		requireBookingTransition(t, e, bookingID, "return", "returned")
	})

	// 6. Complete
	t.Run("complete", func(t *testing.T) {
		requireBookingTransition(t, e, bookingID, "complete", "completed")
	})

	// 7. Verify final state
	t.Run("verify_completed", func(t *testing.T) {
		if bookingID == "" {
			t.Skip("no booking")
		}
		resp, err := e.api.Raw("GET", "/api/v1/rental/bookings/"+bookingID, nil)
		require.NoError(t, err)
		defer resp.Body.Close()
		var r map[string]any
		b, _ := io.ReadAll(resp.Body)
		json.Unmarshal(b, &r)
		assert.Equal(t, "completed", r["status"])
	})
}

// TestBookingCancelFromReserved tests cancellation path.
func TestBookingCancelFromReserved(t *testing.T) {
	e := setup(t)
	orgID := getOrgID(t, e)

	resp, err := e.api.Raw("GET", "/api/v1/rental/vehicles", nil)
	require.NoError(t, err)
	resp.Body.Close()
	if resp.StatusCode == 403 || resp.StatusCode == 404 {
		t.Skip("Rental not enabled")
	}

	// Quick setup — reuse existing entities if possible
	r := apiCreate(t, e, "/api/v1/rental/vehicle-classes", map[string]any{"name": "Cancel Test", "code": "CANCEL-CLS", "vehicle_type": "car", "org_id": orgID})
	classID, _ := r["id"].(string)
	r = apiCreate(t, e, "/api/v1/rental/vehicles", map[string]any{
		"org_id": orgID, "vehicle_class_id": classID,
		"vehicle_type": "car", "plate": "CANCEL-1", "make": "Test", "model": "C", "status": "available",
	})
	vID, _ := r["id"].(string)

	r = apiCreate(t, e, "/api/v1/rental/persons", map[string]any{
		"phone": "+971500088002", "first_name": "Cancel", "last_name": "Test",
	})
	pID, _ := r["id"].(string)
	r = apiCreate(t, e, "/api/v1/rental/customers", map[string]any{
		"org_id": orgID, "customer_type": "individual", "person_id": pID,
	})
	cID, _ := r["id"].(string)

	// Create + cancel
	r = apiCreate(t, e, "/api/v1/rental/bookings", map[string]any{
		"org_id": orgID, "vehicle_id": vID, "customer_id": cID,
	})
	bID, _ := r["id"].(string)
	require.NotEmpty(t, bID)

	requireBookingTransition(t, e, bID, "cancel", "cancelled")
}

// ── helpers ──

func requireBookingTransition(t *testing.T, e *testEnv, bookingID, action, expectedStatus string) {
	t.Helper()
	if bookingID == "" {
		t.Skip("no booking")
	}
	resp, err := e.api.Raw("POST", "/api/v1/rental/bookings/"+bookingID+"/"+action, nil)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Less(t, resp.StatusCode, 300, "transition %s should succeed", action)

	var r map[string]any
	b, _ := io.ReadAll(resp.Body)
	json.Unmarshal(b, &r)
	assert.Equal(t, expectedStatus, r["status"], "after %s status should be %s", action, expectedStatus)
}



