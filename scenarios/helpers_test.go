package scenarios

import (
	"encoding/json"
	"io"
	"testing"
	"time"

	"github.com/OArus89/trakxneo-test/checks"
	"github.com/OArus89/trakxneo-test/clients"
	"github.com/OArus89/trakxneo-test/config"
)

// testEnv holds shared clients for all test scenarios.
type testEnv struct {
	cfg       *config.Config
	api       *clients.APIClient
	gateway   *clients.GatewayClient
	pg        *checks.Postgres
	ch        *checks.ClickHouse
	dragonfly *checks.Dragonfly
	orgID     string // cached first org ID
}

var env *testEnv

func setup(t *testing.T) *testEnv {
	t.Helper()
	if env != nil {
		return env
	}

	cfg, err := config.Load("")
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	api := clients.NewAPIClient(cfg)
	if err := api.Login(cfg.Auth.Username, cfg.Auth.Password); err != nil {
		t.Fatalf("login: %v", err)
	}

	gw := clients.NewGatewayClient(cfg)

	pg, err := checks.NewPostgres(cfg)
	if err != nil {
		t.Fatalf("connect postgres: %v", err)
	}

	ch, err := checks.NewClickHouse(cfg)
	if err != nil {
		t.Fatalf("connect clickhouse: %v", err)
	}

	df := checks.NewDragonfly(cfg)

	env = &testEnv{
		cfg:       cfg,
		api:       api,
		gateway:   gw,
		pg:        pg,
		ch:        ch,
		dragonfly: df,
	}
	return env
}

// waitFor polls fn every 500ms until it returns true or timeout expires.
func waitFor(t *testing.T, timeout time.Duration, fn func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if fn() {
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatalf("waitFor timed out after %s", timeout)
}

// decodeBody decodes JSON response body into target struct.
func decodeBody(body io.Reader, target any) error {
	return json.NewDecoder(body).Decode(target)
}
