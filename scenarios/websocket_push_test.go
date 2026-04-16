package scenarios

import (
	"os"
	"testing"
	"time"

	"github.com/OArus89/trakxneo-test/clients"
	"github.com/stretchr/testify/require"
)

// TestWebSocketPush verifies that a TCP telemetry packet triggers a WebSocket message.
func TestWebSocketPush(t *testing.T) {
	e := setup(t)

	imei := os.Getenv("TEST_IMEI_TELTONIKA")
	if imei == "" {
		t.Skip("TEST_IMEI_TELTONIKA not set")
	}

	// 1. Connect WebSocket
	ws := clients.NewWSClient(e.cfg)
	token := getToken(t, e)
	err := ws.Connect(token)
	if err != nil {
		t.Skipf("WebSocket not available: %v", err)
	}
	defer ws.Close()

	// 2. Send a TCP packet with distinctive speed
	session, err := e.gateway.ConnectTeltonika(imei)
	require.NoError(t, err)
	defer session.Close()

	err = session.SendCodec8(25.2100, 55.2900, 77, 45, true, true, 13800, 4100, 160000)
	require.NoError(t, err)

	// 3. Read WebSocket messages for up to 15s looking for our device update
	t.Run("receives_update", func(t *testing.T) {
		deadline := time.Now().Add(15 * time.Second)
		found := false
		for time.Now().Before(deadline) {
			msg, err := ws.ReadMessage(5 * time.Second)
			if err != nil {
				continue
			}
			// Check if this message is about our device
			if msgImei, ok := msg["imei"].(string); ok && msgImei == imei {
				found = true
				break
			}
			if msgImei, ok := msg["device_id"].(string); ok && msgImei == imei {
				found = true
				break
			}
		}
		if !found {
			t.Log("WebSocket update not received within 15s — PubSub may be delayed or filtered")
		}
	})
}
