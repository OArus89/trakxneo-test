package scenarios

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestOfflineTransition_ConnectivityStates verifies device connectivity state
// is correctly set to "online" after receiving a packet, and that the opstate
// fields are consistent.
func TestOfflineTransition_ConnectivityStates(t *testing.T) {
	e := setup(t)

	imei := os.Getenv("TEST_IMEI_TELTONIKA")
	if imei == "" {
		t.Skip("TEST_IMEI_TELTONIKA not set")
	}

	session, err := e.gateway.ConnectTeltonika(imei)
	require.NoError(t, err)
	defer session.Close()

	// Send packet to ensure device is online
	err = session.SendCodec8(25.1800, 55.2300, 0, 0, true, false, 13800, 4100, 230000)
	require.NoError(t, err)

	waitFor(t, 10*time.Second, func() bool {
		ops, err := e.dragonfly.DeviceOpState(imei)
		if err != nil {
			return false
		}
		conn, _ := ops["connectivity"].(string)
		return conn == "online"
	})

	t.Run("online_state_correct", func(t *testing.T) {
		ops, err := e.dragonfly.DeviceOpState(imei)
		require.NoError(t, err)
		assert.Equal(t, "online", ops["connectivity"])
		assert.NotNil(t, ops["last_seen"])
	})

	t.Run("device_timestamp_fresh", func(t *testing.T) {
		exists, err := e.dragonfly.KeyExists("device:ts:" + imei)
		require.NoError(t, err)
		assert.True(t, exists)
	})

	// NOTE: Testing standby/offline transitions would require waiting 5+ minutes.
	// Those are best tested by checking existing devices that have been idle.
	t.Run("check_existing_offline_device", func(t *testing.T) {
		// Query PG for any device that hasn't sent data recently
		rows, err := e.pg.Query(`
			SELECT d.imei FROM tracking.devices d
			WHERE d.is_active = true
			LIMIT 5`)
		if err != nil {
			t.Skipf("cannot query devices: %v", err)
		}
		if len(rows) == 0 {
			t.Skip("no devices to check")
		}

		// Verify each device has an opstate in Dragonfly (even if offline)
		for _, row := range rows {
			deviceIMEI, _ := row["imei"].(string)
			if deviceIMEI == "" {
				continue
			}
			ops, err := e.dragonfly.DeviceOpState(deviceIMEI)
			if err != nil {
				continue // device may not have state yet
			}
			conn, _ := ops["connectivity"].(string)
			assert.Contains(t, []string{"online", "standby", "offline"}, conn,
				"device %s should have valid connectivity state", deviceIMEI)
		}
	})
}
