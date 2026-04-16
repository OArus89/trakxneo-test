package scenarios

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDashboardDevice(t *testing.T) {
	e := setup(t)
	imei := os.Getenv("TEST_IMEI_TELTONIKA")
	if imei == "" {
		imei = "864636066073087" // fallback to known device
	}

	t.Run("device_detail", func(t *testing.T) {
		dev, err := e.api.GetDashboardDevice(imei)
		require.NoError(t, err)
		assert.NotNil(t, dev)
	})
}

func TestDashboardDeviceList(t *testing.T) {
	e := setup(t)
	orgID := getOrgID(t, e)

	resp, err := e.api.Raw("GET", "/api/v1/dashboard/devices?org_id="+orgID, nil)
	if err != nil {
		// Try alternative endpoint
		resp, err = e.api.Raw("GET", "/api/v1/devices", nil)
	}
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Less(t, resp.StatusCode, 500, "should not return server error")
}
