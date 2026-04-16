package checks

import (
	"context"
	"fmt"
	"time"

	"github.com/OArus89/trakxneo-test/config"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Postgres verifies state in PostgreSQL.
type Postgres struct {
	pool *pgxpool.Pool
}

func NewPostgres(cfg *config.Config) (*Postgres, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, cfg.Postgres.DSN())
	if err != nil {
		return nil, fmt.Errorf("connect postgres: %w", err)
	}
	return &Postgres{pool: pool}, nil
}

func (p *Postgres) Close() { p.pool.Close() }

// TripByID returns a trip record.
func (p *Postgres) TripByID(tripID string) (map[string]any, error) {
	ctx := context.Background()
	row := p.pool.QueryRow(ctx,
		`SELECT id, device_imei, status, start_time, end_time, distance_m,
		        end_reason, route_polyline IS NOT NULL as has_polyline
		 FROM tracking.trips WHERE id = $1`, tripID)

	var id, imei, status string
	var startTime time.Time
	var endTime *time.Time
	var distanceM *float64
	var endReason *string
	var hasPolyline bool

	if err := row.Scan(&id, &imei, &status, &startTime, &endTime, &distanceM, &endReason, &hasPolyline); err != nil {
		return nil, fmt.Errorf("query trip: %w", err)
	}

	result := map[string]any{
		"id":           id,
		"device_imei":  imei,
		"status":       status,
		"start_time":   startTime,
		"has_polyline":  hasPolyline,
	}
	if endTime != nil {
		result["end_time"] = *endTime
	}
	if distanceM != nil {
		result["distance_m"] = *distanceM
	}
	if endReason != nil {
		result["end_reason"] = *endReason
	}
	return result, nil
}

// ActiveTripForDevice returns the active trip for a device, or nil.
func (p *Postgres) ActiveTripForDevice(imei string) (map[string]any, error) {
	ctx := context.Background()
	row := p.pool.QueryRow(ctx,
		`SELECT id, status, start_time, distance_m
		 FROM tracking.trips WHERE device_imei = $1 AND status = 'active'
		 ORDER BY start_time DESC LIMIT 1`, imei)

	var id, status string
	var startTime time.Time
	var distanceM *float64

	if err := row.Scan(&id, &status, &startTime, &distanceM); err != nil {
		return nil, err
	}

	return map[string]any{
		"id":         id,
		"status":     status,
		"start_time": startTime,
		"distance_m": distanceM,
	}, nil
}

// DeviceExists checks if a device IMEI exists in tracking.devices.
func (p *Postgres) DeviceExists(imei string) (bool, error) {
	ctx := context.Background()
	var exists bool
	err := p.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM tracking.devices WHERE imei = $1)`, imei).Scan(&exists)
	return exists, err
}

// Query runs an arbitrary query and returns rows as maps.
func (p *Postgres) Query(query string, args ...any) ([]map[string]any, error) {
	ctx := context.Background()
	rows, err := p.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cols := rows.FieldDescriptions()
	var results []map[string]any
	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return nil, err
		}
		row := make(map[string]any, len(cols))
		for i, col := range cols {
			row[string(col.Name)] = values[i]
		}
		results = append(results, row)
	}
	return results, rows.Err()
}
