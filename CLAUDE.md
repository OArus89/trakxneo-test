# CLAUDE.md

## Project Overview

**trakxneo-tests** — black-box E2E test suite for the TrakXNeo GPS tracking platform. Tests hit external interfaces (TCP, HTTP, WebSocket) and verify results via database queries. No internal package imports — treats the system as a black box.

Target system: **TrakXNeo** (fork of TrackX/AynShahin) — 17 Go microservices, gRPC + Kafka, polyglot data layer.

## Target Environments

| Environment | Host | Notes |
|-------------|------|-------|
| **trakxneo** (primary) | 192.168.1.9 | TrakXNeo production |
| **aynshahin** (dev) | 192.168.1.149 | TrackX dev stack |
| **trakx1** (prod) | 192.168.1.60 | TrackX production |

Config in `config/env.yaml` — select target before running.

## Architecture

```
trakxneo-tests/
├── CLAUDE.md
├── go.mod
├── config/           # Target environment configs (hosts, ports, creds)
│   └── env.yaml
├── clients/          # Protocol clients for hitting external interfaces
│   ├── gateway.go    # TCP: Teltonika Codec 8 + Ruptela packet sender
│   ├── api.go        # HTTP: REST API client (auth, devices, trips, alerts)
│   └── ws.go         # WebSocket: realtime subscription client
├── checks/           # Database verifiers — query and assert state
│   ├── postgres.go   # PG: devices, trips, users, geofences, commands
│   ├── clickhouse.go # CH: gps_telemetry, rollups
│   └── dragonfly.go  # Redis: device:state, device:opstate, device:trip
├── scenarios/        # Test scenarios (go test files)
│   ├── telemetry_pipeline_test.go   # TCP packet → Kafka → CH + Dragonfly
│   ├── trip_lifecycle_test.go       # start → drive → park → end → archived
│   ├── geofence_alerts_test.go      # enter/exit zone → alert fired
│   ├── auth_permissions_test.go     # JWT → guards → RBAC enforcement
│   ├── commands_test.go             # API → Kafka → Gateway → TCP → ACK
│   ├── device_provisioning_test.go  # CRUD + IMEI Luhn + suspend/reactivate
│   ├── offline_transition_test.go   # online → standby → offline, trip close
│   ├── websocket_push_test.go       # telemetry → PubSub → WS client
│   └── reports_test.go             # trip reports, geofence crossings
├── report/           # Report generation (HTML, JUnit XML)
└── Makefile
```

## How to Run

```bash
# All scenarios
make test

# Single scenario
go test ./scenarios/ -run TestTelemetryPipeline -v

# With specific target
TARGET=trakxneo make test

# Generate HTML report
make report
```

## Key Interfaces Tested

### TCP Gateway (binary protocols)
- **Teltonika** port 15002 — Codec 8 packets (IMEI handshake → data)
- **Ruptela** port 15003 — Extended Records (device ID → records)

### HTTP API (port 18090)
- Auth: `POST /api/v1/auth/login` → JWT
- Devices: CRUD at `/api/v1/devices`
- Trips: `/api/v1/trips/{imei}`
- Alerts: `/api/v1/alerts`
- Commands: `/api/v1/commands`
- Dashboard: `/api/v1/dashboard/device/{imei}`

### WebSocket (port 18443)
- Connect: `wss://host:18443/ws?token=JWT`
- Subscribe to device updates

### Databases (verification only — read, never write directly)
- **PostgreSQL** (15432): `tracking.devices`, `tracking.trips`, `tracking.geofences`
- **ClickHouse** (18123): `aynshahin.gps_telemetry`
- **Dragonfly** (16379): `device:state:{imei}`, `device:opstate:{imei}`, `device:trip:{imei}`

## Test Patterns

### waitFor helper
Most tests send data and wait for async processing:
```go
waitFor(t, 10*time.Second, func() bool {
    // poll until condition met or timeout
})
```

### Test device lifecycle
Each test creates a temporary device, runs the scenario, and cleans up:
```go
func TestSomething(t *testing.T) {
    imei := createTestDevice(t)    // POST /api/v1/devices
    defer deleteTestDevice(t, imei) // DELETE /api/v1/devices/{imei}
    // ... test logic ...
}
```

### Assertions
Use `testify/assert` and `testify/require` for assertions.

## Common Commands

```bash
make test              # Run all E2E tests
make test-v            # Verbose output
make test SCENARIO=trip # Run only trip tests
make report            # Generate HTML + JUnit XML report
make lint              # golangci-lint
```

## Dependencies

- Go 1.22+
- `github.com/stretchr/testify` — assertions
- `github.com/jackc/pgx/v5` — PostgreSQL client
- `github.com/ClickHouse/clickhouse-go/v2` — ClickHouse client
- `github.com/redis/go-redis/v9` — Dragonfly/Redis client
- `github.com/gorilla/websocket` — WebSocket client
- `gotestsum` — test runner with JUnit output

## Important Notes

- **Never write directly to databases** — only read for verification. All state changes go through the system's own interfaces (TCP, HTTP).
- **Test devices use reserved IMEI range** — `999000000000001–999000000000999` to avoid collision with real/simulated devices.
- **Clean up after tests** — every test that creates a device must delete it.
- **Timeouts** — async pipeline takes 1-5s. Use `waitFor` with 10-15s timeout.
- **TrakXNeo uses `trakxneo` as infra prefix** (not `aynshahin`). Database name is `trakxneo` on .9, `aynshahin` on .149/.60.
