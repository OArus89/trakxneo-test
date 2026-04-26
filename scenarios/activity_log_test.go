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

// TestActivityLog_LoginRecorded verifies that login events appear in the activity log.
func TestActivityLog_LoginRecorded(t *testing.T) {
	e := setup(t)

	// Activity log should have our login from TestMain
	resp, err := e.api.Raw("GET", "/api/v1/admin/activity-log?limit=20", nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusForbidden {
		t.Skip("Activity log endpoint not available")
	}
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var result struct {
		Items []map[string]any `json:"entries"`
	}
	b, _ := io.ReadAll(resp.Body)
	json.Unmarshal(b, &result)

	// Look for a login_success event
	found := false
	for _, item := range result.Items {
		action, _ := item["action"].(string)
		if action == "login_success" {
			found = true
			break
		}
	}
	assert.True(t, found, "activity log should contain login_success event")
}

// TestActivityLog_DeviceCreateRecorded verifies device creation is logged.
func TestActivityLog_DeviceCreateRecorded(t *testing.T) {
	e := setup(t)
	orgID := getOrgID(t, e)

	testIMEI := "999000000000951"
	_ = e.api.DeleteDevice(testIMEI)

	body := map[string]any{
		"imei": testIMEI, "org_id": orgID,
		"device_type": "tracker", "device_model": "test",
		"protocol": "teltonika", "name": "Activity log test",
	}
	resp, err := e.api.Raw("POST", "/api/v1/devices", body)
	require.NoError(t, err)
	resp.Body.Close()
	defer e.api.DeleteDevice(testIMEI)

	// Wait for async activity log write
	time.Sleep(1 * time.Second)

	// Check activity log
	resp, err = e.api.Raw("GET", "/api/v1/admin/activity-log?limit=5", nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Skip("Activity log not available")
	}

	var result struct {
		Items []map[string]any `json:"entries"`
	}
	b, _ := io.ReadAll(resp.Body)
	json.Unmarshal(b, &result)

	found := false
	for _, item := range result.Items {
		action, _ := item["action"].(string)
		resource, _ := item["resource"].(string)
		if action == "create" && resource == "device" {
			found = true
			break
		}
	}
	assert.True(t, found, "activity log should contain device create event")
}
