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

## v0.6.0 release-candidate checks (Patch 40)

```sh
go clean -testcache
go test ./...
go test -race ./...
go vet ./...
go build ./...
git diff --check
```

In addition to the manual route and worktree checks above, verify:

1. Both HTTP and HTTPS use new, updated and removed runtime routes immediately.
2. A request already in flight finishes after its route is updated or removed.
3. Missing or invalid tokens and browser-originated mutations are rejected.
4. YAML routes cannot be updated or removed by management or worktree commands.
5. `worktree list` excludes noncanonical targets (for example `localhost`
   rather than `127.0.0.1`, or ports outside 20000–49999).
6. `worktree remove` refuses a noncanonical route even if its hostname matches.
7. Two worktrees get distinct names and normally distinct ports; remember that
   port checks are best-effort and not exclusive reservations.
8. Restart clears runtime routes and their assignments without modifying Git
   worktrees or stopping application processes.

**Known limitation:** canonical hostname/target checks cannot prove ownership.
A manually registered route with the same worktree-shaped hostname and
canonical target can still be removed by `worktree remove`. Do not treat
worktree commands as an authorization boundary; they use the same management
token as other route mutations.


## Patch 41: explicit route ownership

```sh
go test -count=1 ./internal/proxy ./internal/statushttp ./cmd
go test -race ./internal/proxy ./internal/statushttp ./cmd
go vet ./...
```

1. Register a route using `clz-lhd route add` and verify `GET /v1/routes`
   reports `"owner":"manual"` for it.
2. Run `clz-lhd worktree env` and verify its route reports
   `"owner":"worktree"`; `worktree list` includes it.
3. Confirm ordinary `clz-lhd route update` and `route remove` return 409
   for that worktree-owned route, without changing the route.
4. Confirm `clz-lhd worktree remove` removes the current worktree route.
5. Restart LHD and confirm runtime routes and ownership are not persisted.

Ownership is not a separate authentication boundary: a holder of the shared
management token can explicitly request another owner. Concurrent port
allocation remains best-effort. The older Patch 40 warning about matching
hostname/target alone no longer applies to Patch 41-aware servers.

## Patch 42: worktree ownership and port collision regression

Automated Go tests cover rejection of missing-owner legacy routes, refusal
to reuse a port shared with another loopback route, and avoidance of ports
used by noncanonical loopback targets. Run `go test ./...`,
`go test -race ./...`, `go vet ./...`, and `go build ./...`.

Manual checks with a running LHD instance and a management token:

1. Register a normal manual route targeting `http://localhost:23456` and
   confirm `worktree env` never selects 23456 for a *new* worktree.
2. Confirm a worktree-owned route appears in `worktree list`; a manual-owned
   route with a similar hostname does not appear and cannot be removed by
   `worktree remove`.
3. Confirm repeated `worktree env` reuses the same unshared route and that
   `worktree remove` succeeds once, then reports absence.
4. Restart LHD and confirm runtime registrations are gone; running
   applications and Git worktree files are unaffected.
5. For concurrent worktrees, verify the resulting ports manually. The
   snapshot-based allocation is not an atomic reservation and may race.

Legacy API responses with missing `owner` are treated as untrusted for
worktree reuse/removal. The shared management token is not an owner identity.

## Patch 43: MCP read-only foundation

Run `go test ./...`, `go test -race ./...`, `go vet ./...`, and
`go build ./...`. Protocol tests verify initialization, tool discovery,
read-only API calls, notifications, malformed JSON and error responses.

With LHD running, configure an MCP stdio client to launch `clz-lhd mcp`
(or `clz-lhd --status-port 9099 mcp`). Verify that it lists only
`lhd_list_routes` and `lhd_get_status`, returns route ownership and status,
and cannot create, update or remove routes. Stop LHD and confirm tool calls
return errors rather than fabricating data. No management token is needed.

## Patch 44: MCP read-only security hardening

Run `go test ./internal/mcp -count=1 -v` followed by `go test ./...`,
`go test -race ./...`, `go vet ./...`, and `go build ./...`. Regression
tests confirm that HTTP redirects are not followed and that proxy
environment variables do not redirect the built-in MCP client.

Manual check: with LHD running, set `HTTP_PROXY` and `HTTPS_PROXY` to a
nonexistent proxy and run `clz-lhd mcp` from an MCP client. Both read-only
tools should still work. Verify `tools/list` still exposes exactly two
tools, with no create/update/delete tools. Do not supply a management token
to the MCP process. Future mutation authorization is a separate milestone.

## Patch 45: Restricted agent API credential

Run `go test ./internal/statushttp -count=1 -v`, `go test ./...`,
`go test -race ./...`, `go vet ./...`, and `go build ./...`.
With different `LHD_MANAGEMENT_TOKEN` and `LHD_AGENT_TOKEN`, confirm agent
POST/PUT/DELETE succeeds only for `owner=agent` and canonical loopback
`.localhost` routes. Confirm it cannot mutate manual/worktree/config routes,
redirect to remote targets, or use a missing/wrong token. With no agent token,
agent mutations must not be enabled by a separate credential. Check MCP still
advertises only the two read-only tools. This patch does not add MCP writes.

## Patch 46: MCP agent mutation tools

Run `go test ./internal/mcp -count=1 -v`, `go test ./...`,
`go test -race ./...`, `go vet ./...`, `go build ./...` and
`git diff --check`.

With LHD running and a distinct `LHD_AGENT_TOKEN` configured for both the
server and the MCP process, check that `clz-lhd mcp` advertises only two
read-only tools. Restart MCP using `clz-lhd mcp --enable-agent-writes` and
check that five tools are advertised. Create, update and remove an
`agent`-owned `.localhost` route with the three new tools; verify the changes
with `clz-lhd routes --live`. Confirm attempts to modify a `manual` or
`worktree` route fail, as do non-loopback upstreams and invalid ports.
Confirm the flag rejects a missing token and a token equal to the admin token.
The token is shared, not a per-agent identity.
