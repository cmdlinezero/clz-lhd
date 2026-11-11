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

## Git worktree discovery (Patch 37)

```bash
go test -count=1 -v ./internal/worktree ./cmd
clz-lhd worktree info
```

Run `clz-lhd worktree info` from the main worktree, a linked worktree, and
inside a nested directory. Confirm that the hostname stays stable within one
worktree and differs between worktrees. In detached HEAD, the hostname should
start with `detached-`. This command only prints a proposed hostname: it does
not reserve ports, register routes, or start services. The suffix is a truncated
path hash and does not constitute a globally guaranteed collision-free name.

## Worktree port environment (Patch 38)

Automated Go tests (no Node.js or Python required):

```sh
go test -count=1 ./internal/worktree ./cmd
go test -race ./internal/worktree ./cmd
```

Manual smoke test (Python is an **optional** HTTP test-server utility):

1. Start `clz-lhd up` with `LHD_MANAGEMENT_TOKEN` set.
2. In a Git worktree with the same token, run `eval "$(clz-lhd worktree env)"`.
3. Check `printf '%s %s\n' "$LHD_HOST" "$LHD_PORT"` and `clz-lhd routes --live`.
4. Optionally start `python3 -m http.server "$LHD_PORT"` and request `curl "$LHD_URL"`.
5. Repeat the environment command and confirm the same port is returned.
6. Try a second worktree and confirm distinct hostnames and ports; restart LHD
   and note that runtime allocations are not persistent.

`eval` is appropriate only for trusted local CLI output. Port checks are
best-effort, not exclusive leases; a separate process can claim a port later.

## Worktree route lifecycle (Patch 39)

Automated tests use Go only:

```sh
go test -count=1 ./cmd ./internal/worktree
go test -race ./cmd ./internal/worktree
```

Manual checks (Git and a running LHD instance required; no Node.js):

1. Start `clz-lhd up` with `LHD_MANAGEMENT_TOKEN` configured.
2. In a Git worktree, run `eval "$(clz-lhd worktree env)"`.
3. Run `clz-lhd worktree list` and verify the hostname, port and target.
4. Run `clz-lhd worktree remove`, then `clz-lhd worktree list`; verify removal.
5. Repeat `worktree remove`; it should report no registration, not fail.
6. Confirm the Git worktree and any running test server are untouched.
7. Verify YAML-managed routes cannot be removed by `worktree remove`.

`worktree list` recognizes hostname patterns, not proven Git ownership.
Orphaned routes from deleted worktrees require explicit `route remove <host>`.
