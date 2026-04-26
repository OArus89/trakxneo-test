package checks

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/OArus89/trakxneo-test/config"
	"github.com/redis/go-redis/v9"
)

// Dragonfly verifies device state in Dragonfly (Redis-compatible).
//
// In the current production schema (TrakXNeo, 2026-04+) every device has a
// SINGLE Redis HASH at key `dev:{imei}` carrying all live state — position,
// IO, operational fields, metadata. The legacy split into `device:state:`,
// `device:opstate:`, `device:ts:` JSON STRINGs has been retired. This
// checker keeps the original method names for compatibility with the
// scenario tests, but routes them all through `HGETALL dev:{imei}`.
//
// Active-trip running stats remain at `device:trip:{imei}` (JSON STRING).
type Dragonfly struct {
	client *redis.Client
}

func NewDragonfly(cfg *config.Config) *Dragonfly {
	return &Dragonfly{
		client: redis.NewClient(&redis.Options{
			Addr:        cfg.Dragonfly.Addr(),
			DialTimeout: 5 * time.Second,
			ReadTimeout: 5 * time.Second,
		}),
	}
}

func (d *Dragonfly) Close() { d.client.Close() }

// DeviceState returns the unified device HASH at dev:{imei}. Returns the
// raw map (all fields are redis strings) so existing tests can keep their
// current assertion shape.
func (d *Dragonfly) DeviceState(imei string) (map[string]any, error) {
	return d.devHash(imei)
}

// DeviceOpState is now an alias of DeviceState — operational fields
// (`connectivity`, `motion`, `last_seen`, `active_trip_id`) live alongside
// position fields in the same HASH.
func (d *Dragonfly) DeviceOpState(imei string) (map[string]any, error) {
	return d.devHash(imei)
}

// DeviceTrip returns the JSON of the active trip running stats. Unchanged.
func (d *Dragonfly) DeviceTrip(imei string) (map[string]any, error) {
	return d.getJSON("device:trip:" + imei)
}

// DeviceTripExists checks if an active trip key exists.
func (d *Dragonfly) DeviceTripExists(imei string) (bool, error) {
	ctx := context.Background()
	n, err := d.client.Exists(ctx, "device:trip:"+imei).Result()
	return n > 0, err
}

// KeyExists checks if any key exists. Tests should prefer DeviceState plus
// a field assertion ("last_seen" is non-empty etc.) over checking arbitrary
// key names — the schema has consolidated. The legacy `device:ts:{imei}`
// key was rolled into the dev:{imei} HASH as the `last_seen` field.
func (d *Dragonfly) KeyExists(key string) (bool, error) {
	ctx := context.Background()
	n, err := d.client.Exists(ctx, key).Result()
	return n > 0, err
}

// HasDeviceField returns true if the dev:{imei} HASH has the named field
// populated (non-empty). Replacement for legacy probes like
// `KeyExists("device:ts:...")`.
func (d *Dragonfly) HasDeviceField(imei, field string) (bool, error) {
	ctx := context.Background()
	v, err := d.client.HGet(ctx, "dev:"+imei, field).Result()
	if err == redis.Nil {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("HGET dev:%s %s: %w", imei, field, err)
	}
	return v != "", nil
}

func (d *Dragonfly) devHash(imei string) (map[string]any, error) {
	ctx := context.Background()
	m, err := d.client.HGetAll(ctx, "dev:"+imei).Result()
	if err != nil {
		return nil, fmt.Errorf("HGETALL dev:%s: %w", imei, err)
	}
	if len(m) == 0 {
		return nil, fmt.Errorf("HGETALL dev:%s: empty (state not yet ingested)", imei)
	}
	// Coerce values for assertion compatibility with the JSON-shape the
	// tests historically expected. Redis HGETALL returns everything as
	// string; tests written against the legacy `device:state:{imei}` JSON
	// often type-assert `state["lat"].(float64)` etc. Try to parse numeric
	// fields as float64 (matches encoding/json behaviour); fall back to
	// the original string when parsing fails.
	out := make(map[string]any, len(m))
	for k, v := range m {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			out[k] = f
			continue
		}
		out[k] = v
	}
	return out, nil
}

func (d *Dragonfly) getJSON(key string) (map[string]any, error) {
	ctx := context.Background()
	val, err := d.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return nil, fmt.Errorf("GET %s: not found", key)
	}
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", key, err)
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(val), &result); err != nil {
		return nil, fmt.Errorf("unmarshal %s: %w", key, err)
	}
	return result, nil
}
