# benchbug

`benchbug` is a small local HTTP load testing CLI written in Go.

It is intentionally simpler than k6: scenarios are YAML/JSON files, execution is a single local process with many VU goroutines, and analytics are printed in the terminal.

## Install / Build

```sh
go build -o benchbug ./cmd/benchbug
```

This project targets Go 1.26.

## Usage

```sh
./benchbug validate -f examples/httpbin.yaml
./benchbug run -f examples/httpbin.yaml
```

Useful flags:

```sh
./benchbug run -f scenario.yaml -vus 20 -duration 30s
./benchbug run -f scenario.yaml -arrival-rate 100 -duration 30s -max-vus 50
./benchbug run -f scenario.yaml -quiet
./benchbug run -f scenario.yaml -json
./benchbug run -f scenario.yaml -timeout 5s -max-body 1048576
```

## Local Target Service

The repository includes a small local HTTP target for testing `benchbug` without relying on external services.

Start it:

```sh
go run ./cmd/target -addr :8080
```

Run the local scenario in another terminal:

```sh
./benchbug validate -f examples/local-target.yaml
./benchbug run -f examples/local-target.yaml
./benchbug run -f examples/local-arrival-rate.yaml
```

Target endpoints:

- `GET /health`
- `POST /login`
- `GET /users`
- `POST /users`
- `GET /users/{id}`
- `GET /slow?ms=250`
- `GET /flaky?rate=10`
- `GET /bytes?n=1mb`

`/users` endpoints require `Authorization: Bearer <token>` by default. Use `/login` to get the token. Disable this with:

```sh
go run ./cmd/target -auth=false
```

Per-request logs are disabled by default because they distort load-test output. Enable them only while debugging:

```sh
go run ./cmd/target -log-requests
```

Exit codes:

- `0`: run completed and thresholds passed
- `1`: runtime/config error
- `2`: at least one threshold failed

## Scenario Shape

```yaml
name: api-smoke
base_url: https://example.com
vus: 10
duration: 30s

defaults:
  timeout: 5s
  headers:
    User-Agent: benchbug/0.1

vars:
  client_id: demo-${__rand_int(1000,9999)}

steps:
  - name: list-users
    group: users
    request:
      method: GET
      url: /users?vu=${__vu}&iter=${__iter}
    checks:
      - status_in: [200]
      - jsonpath_exists: data

thresholds:
  - metric: http_req_failed_rate
    op: <
    value: 0.01
  - metric: http_req_duration_p95
    op: <
    value: 300ms
```

## Arrival Rate Mode

The default `vus` mode is a closed model: each VU waits for its iteration to finish before starting the next one.

`arrival_rate` is an open model: `benchbug` starts a fixed number of new iterations per time window. If all `max_vus` slots are busy, the new iteration is dropped and counted as `dropped_iterations`.

```yaml
name: open-model
base_url: http://localhost:8080

arrival_rate:
  rate: 100
  per: 1s
  duration: 30s
  max_vus: 50

steps:
  - name: slow
    group: api
    request:
      method: GET
      url: /slow?ms=100
```

Useful thresholds for this mode:

- `dropped_iterations`
- `dropped_iterations_rate`

Supported template values:

- `${var}` from `vars` or previous `extract`
- `${__vu}`
- `${__iter}`
- `${__rand_int(min,max)}`

Supported checks:

- `status_in: [200, 201]`
- `jsonpath_exists: path.to.value`
- `jsonpath_eq: { path: ok, value: true }`
- `header_eq: { X-Header: value }`

Supported thresholds:

- `http_req_failed_rate`
- `checks_pass_rate`
- `http_req_duration_p50`
- `http_req_duration_p90`
- `http_req_duration_p95`
- `http_req_duration_p99`
- `dropped_iterations`
- `dropped_iterations_rate`
