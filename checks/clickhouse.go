package checks

import (
	"context"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/OArus89/trakxneo-test/config"
)

// ClickHouse verifies telemetry data in ClickHouse.
type ClickHouse struct {
	conn clickhouse.Conn
	db   string // database name (trakxneo or aynshahin)
}

func NewClickHouse(cfg *config.Config) (*ClickHouse, error) {
	conn, err := clickhouse.Open(&clickhouse.Options{
		// Tunnel forwards 28123→prod 8123 (HTTP). Native protocol on 9000
		// is not exposed; explicitly select HTTP so the handshake matches.
		Protocol: clickhouse.HTTP,
		Addr: []string{fmt.Sprintf("%s:%d", cfg.ClickHouse.Host, cfg.ClickHouse.Port)},
		Settings: clickhouse.Settings{
			"max_execution_time": 30,
		},
		DialTimeout: 5 * time.Second,
		ReadTimeout: 10 * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("connect clickhouse: %w", err)
	}
	return &ClickHouse{conn: conn, db: cfg.ClickHouse.Database}, nil
}

func (c *ClickHouse) Close() { c.conn.Close() }

// TelemetryCount returns the number of telemetry points for a device since a given time.
func (c *ClickHouse) TelemetryCount(deviceID string, since time.Time) (uint64, error) {
	ctx := context.Background()
	var count uint64
	err := c.conn.QueryRow(ctx,
		fmt.Sprintf(`SELECT count() FROM %s.gps_telemetry WHERE device_id = ? AND timestamp >= ?`, c.db),
		deviceID, since).Scan(&count)
	return count, err
}

// LatestTelemetry returns the most recent telemetry point for a device.
// Note: production ClickHouse columns are `lat`/`lon` (not latitude/longitude),
// `heading` is Float32 (not uint16), and `ignition`/`fix_valid` are UInt8
// (0/1) — scan accordingly. The returned map exposes the legacy aliases the
// scenario tests still assert against.
func (c *ClickHouse) LatestTelemetry(deviceID string) (map[string]any, error) {
	ctx := context.Background()
	rows, err := c.conn.Query(ctx,
		fmt.Sprintf(`SELECT device_id, timestamp, lat, lon, speed, heading,
		        ignition, fix_valid
		 FROM %s.gps_telemetry
		 WHERE device_id = ?
		 ORDER BY timestamp DESC LIMIT 1`, c.db), deviceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, fmt.Errorf("no telemetry for device %s", deviceID)
	}

	var devID string
	var ts time.Time
	var lat, lon float64
	var speed, hdg float32
	var ign, fix uint8

	if err := rows.Scan(&devID, &ts, &lat, &lon, &speed, &hdg, &ign, &fix); err != nil {
		return nil, err
	}

	return map[string]any{
		"device_id": devID,
		"timestamp": ts,
		"latitude":  lat,
		"longitude": lon,
		"speed":     speed,
		"heading":   hdg,
		"ignition":  ign != 0,
		"fix_valid": fix != 0,
	}, nil
}
