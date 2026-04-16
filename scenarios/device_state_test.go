package scenarios

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDeviceStateKeys verifies all 4 Dragonfly keys are populated after telemetry.
func TestDeviceStateKeys(t *testing.T) {
	e := setup(t)

	imei := os.Getenv("TEST_IMEI_TELTONIKA")
	if imei == "" {
		t.Skip("TEST_IMEI_TELTONIKA not set")
	}

	session, err := e.gateway.ConnectTeltonika(imei)
	require.NoError(t, err)
	defer session.Close()

	err = session.SendCodec8(25.1700, 55.2200, 35, 120, true, true, 13600, 4050, 170000)
	require.NoError(t, err)

	waitFor(t, 10*time.Second, func() bool {
		_, err := e.dragonfly.DeviceState(imei)
		return err == nil
	})

	t.Run("device_state", func(t *testing.T) {
		state, err := e.dragonfly.DeviceState(imei)
		require.NoError(t, err)
		assert.NotNil(t, state["lat"])
		assert.NotNil(t, state["lon"])
		assert.NotNil(t, state["speed"])
	})

	t.Run("device_opstate", func(t *testing.T) {
		ops, err := e.dragonfly.DeviceOpState(imei)
		require.NoError(t, err)
		conn, _ := ops["connectivity"].(string)
		assert.Equal(t, "online", conn)
		assert.NotNil(t, ops["last_seen"])
	})

	t.Run("device_timestamp", func(t *testing.T) {
		exists, err := e.dragonfly.KeyExists("device:ts:" + imei)
		require.NoError(t, err)
		assert.True(t, exists, "device:ts key should exist")
	})
}

// TestDeviceIgnitionState verifies ignition ON/OFF is reflected in device state.
func TestDeviceIgnitionState(t *testing.T) {
	e := setup(t)

	imei := os.Getenv("TEST_IMEI_TELTONIKA")
	if imei == "" {
		t.Skip("TEST_IMEI_TELTONIKA not set")
	}

	session, err := e.gateway.ConnectTeltonika(imei)
	require.NoError(t, err)
	defer session.Close()

	// Send ignition ON
	err = session.SendCodec8(25.1750, 55.2250, 0, 0, true, false, 13800, 4100, 180000)
	require.NoError(t, err)
	time.Sleep(5 * time.Second)

	t.Run("ignition_on", func(t *testing.T) {
		state, err := e.dragonfly.DeviceState(imei)
		require.NoError(t, err)
		ign, ok := state["ignition"].(bool)
		if ok {
			assert.True(t, ign, "ignition should be ON")
		}
	})

	// Send ignition OFF
	err = session.SendCodec8(25.1750, 55.2250, 0, 0, false, false, 12400, 4000, 180000)
	require.NoError(t, err)
	time.Sleep(5 * time.Second)

	t.Run("ignition_off", func(t *testing.T) {
		state, err := e.dragonfly.DeviceState(imei)
		require.NoError(t, err)
		ign, ok := state["ignition"].(bool)
		if ok {
			assert.False(t, ign, "ignition should be OFF")
		}
	})
}

// TestDeviceVoltageReflection verifies ext_voltage is stored correctly.
func TestDeviceVoltageReflection(t *testing.T) {
	e := setup(t)

	imei := os.Getenv("TEST_IMEI_TELTONIKA")
	if imei == "" {
		t.Skip("TEST_IMEI_TELTONIKA not set")
	}

	session, err := e.gateway.ConnectTeltonika(imei)
	require.NoError(t, err)
	defer session.Close()

	// Send with specific voltage
	err = session.SendCodec8(25.1760, 55.2260, 50, 90, true, true, 14200, 4100, 190000)
	require.NoError(t, err)
	time.Sleep(5 * time.Second)

	state, err := e.dragonfly.DeviceState(imei)
	require.NoError(t, err)

	if voltage, ok := state["ext_voltage_mv"].(float64); ok {
		assert.InDelta(t, 14200, voltage, 200, "voltage should be ~14200mV")
	}
}
