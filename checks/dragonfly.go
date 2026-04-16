package checks

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/OArus89/trakxneo-test/config"
	"github.com/redis/go-redis/v9"
)

// Dragonfly verifies device state in Dragonfly (Redis-compatible).
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

// DeviceState returns the parsed device:state:{imei} JSON.
func (d *Dragonfly) DeviceState(imei string) (map[string]any, error) {
	return d.getJSON("device:state:" + imei)
}

// DeviceOpState returns the parsed device:opstate:{imei} JSON.
func (d *Dragonfly) DeviceOpState(imei string) (map[string]any, error) {
	return d.getJSON("device:opstate:" + imei)
}

// DeviceTrip returns the parsed device:trip:{imei} JSON (active trip stats).
func (d *Dragonfly) DeviceTrip(imei string) (map[string]any, error) {
	return d.getJSON("device:trip:" + imei)
}

// DeviceTripExists checks if an active trip key exists.
func (d *Dragonfly) DeviceTripExists(imei string) (bool, error) {
	ctx := context.Background()
	n, err := d.client.Exists(ctx, "device:trip:"+imei).Result()
	return n > 0, err
}

// KeyExists checks if any key exists.
func (d *Dragonfly) KeyExists(key string) (bool, error) {
	ctx := context.Background()
	n, err := d.client.Exists(ctx, key).Result()
	return n > 0, err
}

func (d *Dragonfly) getJSON(key string) (map[string]any, error) {
	ctx := context.Background()
	val, err := d.client.Get(ctx, key).Result()
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", key, err)
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(val), &result); err != nil {
		return nil, fmt.Errorf("unmarshal %s: %w", key, err)
	}
	return result, nil
}
