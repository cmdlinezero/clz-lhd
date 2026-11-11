# Testing CLZ LHD

Run these commands from the repository root with the Go toolchain specified by
`go.mod`. The project currently requires Go 1.25.5.

## Automated verification

```bash
go clean -testcache
go test ./...
go test -race ./...
go vet ./...
go build ./...
git diff --check
```

To focus on route management:

```bash
go test -count=1 -v ./cmd ./internal/proxy ./internal/statushttp
```

The tests use local HTTP servers and do not require a real Ollama instance,
remote SSH host, or active LHD daemon. A passing automated suite does not
replace manual checks of the running proxy and its listeners.

## Manual smoke test: runtime routes

1. Start LHD using a valid `lhd.yaml` and a strong `LHD_MANAGEMENT_TOKEN`.
2. Start a local HTTP application on port 5173 (or adjust the examples).
3. From a separate shell with the same token, run:

```bash
clz-lhd route add demo.localhost http://127.0.0.1:5173
clz-lhd routes --live
curl -i http://demo.localhost/
clz-lhd route update demo.localhost http://127.0.0.1:5174
clz-lhd route remove demo.localhost
```

4. Verify that the route appears in the Routes TUI view, changes target without
   restarting the proxy, and disappears after removal. If HTTPS is configured,
   check that the same runtime route works on HTTPS as well (certificate coverage
   is still required).
5. Confirm that modifying a YAML-managed route is rejected and that a missing or
   incorrect management token cannot mutate routes.
6. Confirm `clz-lhd routes` (without `--live`) still displays YAML routes and
   `clz-lhd route -r demo.localhost=5173` retains its existing startup behaviour.

Runtime routes are ephemeral. A restart clears them. The management API must
remain bound to loopback; do not expose the token or management listener.
