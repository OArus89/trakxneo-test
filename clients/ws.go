package clients

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"time"

	"github.com/OArus89/trakxneo-test/config"
	"github.com/gorilla/websocket"
)

// WSClient connects to the TrakXNeo WebSocket endpoint.
type WSClient struct {
	url  string
	conn *websocket.Conn
}

func NewWSClient(cfg *config.Config) *WSClient {
	return &WSClient{url: cfg.WebSocket.URL}
}

// Connect opens a WebSocket connection with the given JWT token.
func (w *WSClient) Connect(token string) error {
	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
		TLSClientConfig:  &tls.Config{InsecureSkipVerify: true},
	}
	url := fmt.Sprintf("%s?token=%s", w.url, token)
	conn, _, err := dialer.Dial(url, nil)
	if err != nil {
		return fmt.Errorf("ws dial: %w", err)
	}
	w.conn = conn
	return nil
}

// ReadMessage reads one JSON message with timeout.
func (w *WSClient) ReadMessage(timeout time.Duration) (map[string]any, error) {
	if w.conn == nil {
		return nil, fmt.Errorf("not connected")
	}
	w.conn.SetReadDeadline(time.Now().Add(timeout))
	_, msg, err := w.conn.ReadMessage()
	if err != nil {
		return nil, err
	}
	var result map[string]any
	if err := json.Unmarshal(msg, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// Close closes the WebSocket connection.
func (w *WSClient) Close() {
	if w.conn != nil {
		w.conn.Close()
	}
}
