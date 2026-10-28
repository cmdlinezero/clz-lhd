# CLZ Local Host Development (LHD)

![Beta](https://img.shields.io/badge/Release%20Status-Beta-red)
![Tag](https://img.shields.io/github/v/tag/cmdlinezero/clz-lhd?label=Tag)
[![License: GPL v3](https://img.shields.io/badge/License-GPL%20v3-blue.svg)](/LICENSE)
![Go](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go&logoColor=white)
![Platforms](https://img.shields.io/badge/Platform-Windows%20%7C%20Linux%20%7C%20macOS-green)

`clz-lhd` is a small local development reverse proxy that maps friendly `.localhost` hostnames to applications running on different local ports. It supports project configuration through `lhd.yaml`, ad-hoc routes from the command line, diagnostics, and Prometheus metrics.

For example, a request to `http://app.localhost` can be proxied to an application listening on `http://localhost:3000`, while `http://api.localhost` can be sent to another service on port 4000.

## Requirements

- Go 1.25 or later when building from source.
- A platform that resolves `*.localhost` to the local machine. Modern browsers and operating systems normally provide the special-use `.localhost` behaviour without editing a hosts file.
- Permission to bind the selected proxy port. The default is port `80`; use `--port` (or `-p`) to choose another port when needed.

## Build

Clone the repository and build the executable:

```bash
git clone https://github.com/cmdlinezero/clz-lhd.git
cd clz-lhd
go build -o clz-lhd .
```

You can also use `go run .` in the examples below while developing the project.

## Quick start with `lhd.yaml`

The recommended project workflow is to keep an `lhd.yaml` file in the project directory:

```yaml
routes:
  app.localhost: 3000
  api.localhost: 4000
```

A numeric target is shorthand for `http://localhost:<port>`. Start the proxy with:

```bash
./clz-lhd up
```

By default LHD listens on port `80`, so the routes above are available as:

```text
http://app.localhost  -> http://localhost:3000
http://api.localhost  -> http://localhost:4000
```

To use a non-privileged or otherwise different proxy port:

```bash
./clz-lhd up --port 8088
```

Then include that port in the browser or client URL, for example `http://app.localhost:8088`.

Use a different configuration file with `--config` / `-c`:

```bash
./clz-lhd up --config ./config/lhd.yaml --port 8088
```

LHD prints the resolved routes, listening address, and metrics endpoint when it starts. `Ctrl-C` (SIGINT) and SIGTERM trigger a graceful server shutdown.

## Configuration

The configuration file contains a `routes` mapping from request hostnames to upstream targets:

```yaml
routes:
  app.localhost: 3000
  api.localhost: http://127.0.0.1:8080
  secure-api.localhost: https://127.0.0.1:8443
```

Targets can be either:

- a port number from `1` to `65535`, expanded to `http://localhost:<port>`; or
- an explicit `http://` or `https://` URL with a host.

Hostnames are normalized to lowercase and a trailing dot is removed. Invalid hosts, invalid targets, duplicate routes, unknown YAML fields, malformed YAML, and empty route sets are rejected before the proxy starts.

An `https://` target means LHD connects **to the upstream using HTTPS**. v0.2 does not terminate HTTPS for incoming local requests; the LHD listener itself is HTTP.

## Commands

Commands accept flags only; unexpected positional arguments are rejected.

### `up`

Start the proxy using a configuration file. The default file is `lhd.yaml`.

```bash
./clz-lhd up
./clz-lhd up --config ./other.yaml
./clz-lhd up --port 8088
```

### `routes`

Load and validate a configuration file, then print the normalized routes without starting the proxy:

```bash
./clz-lhd routes
./clz-lhd routes --config ./other.yaml
```

Output is sorted by hostname:

```text
HOST            TARGET
api.localhost   http://localhost:4000
app.localhost   http://localhost:3000
```

### `doctor`

Check the configuration and local connectivity before starting LHD:

```bash
./clz-lhd doctor
./clz-lhd doctor --config ./other.yaml --port 8088
```

`doctor` verifies that the configuration loads, reports the number of routes, checks whether the selected proxy port can be bound, and attempts a TCP connection to each configured upstream. It does not make HTTP requests to the upstream applications.

The command exits non-zero when one or more diagnostics fail.

### `route`

Start LHD with ad-hoc routes instead of a YAML file. Repeat `--route` / `-r` for multiple mappings:

```bash
./clz-lhd route -r app.localhost=3000
./clz-lhd route \
  -r app.localhost=3000 \
  -r api.localhost=http://127.0.0.1:4000 \
  --port 8088
```

The same host and target validation used for `lhd.yaml` applies to command-line routes.

## Proxy behaviour

LHD routes requests by the HTTP `Host` header. Host matching is case-insensitive and ignores a request port and trailing dot.

When proxying a request, LHD sets `X-Forwarded-Host` and `X-Forwarded-Proto`; Go's reverse proxy also maintains `X-Forwarded-For`.

If a requested host is not configured, LHD responds with `404 Not Found` and:

```text
Proxy: Host not configured
```

If a configured upstream cannot be reached, LHD responds with `502 Bad Gateway` and:

```text
Proxy: upstream unavailable
```

## Prometheus metrics

While the proxy is running, Prometheus metrics are exposed at:

```text
http://localhost:<proxy-port>/metrics
```

LHD's application metrics are:

| Metric | Labels | Meaning |
| --- | --- | --- |
| `lhd_requests_total` | `host`, `status` | Total requests successfully routed to a configured upstream proxy, labelled by the observed HTTP response status. Upstream connection failures are recorded as `502`. |
| `lhd_request_duration_seconds` | `host` | Histogram of time spent proxying requests to configured hosts. |
| `lhd_requests_in_flight` | `host` | Requests currently being handled. This includes routing attempts for unconfigured hosts, which return to zero when the request completes. |

For example:

```promql
sum by (host) (rate(lhd_requests_total[5m]))
```

The `/metrics` endpoint also contains the standard Go and Prometheus process/runtime metrics registered by the client library.

## CLI help

The current command surface is:

```text
clz-lhd
├── up       Start the proxy using a configuration file
├── routes   Show configured routes
├── doctor   Check local proxy configuration and connectivity
└── route    Start the proxy with host mappings
```

Use `--help` at the root or command level for the authoritative flag reference:

```bash
./clz-lhd --help
./clz-lhd up --help
./clz-lhd doctor --help
```

The global `--port` / `-p` flag defaults to `80`. Both `up` and `doctor` use `--config` / `-c`, which defaults to `lhd.yaml`.

## License

CLZ LHD is licensed under the GNU General Public License v3.0. See [LICENSE](/LICENSE).

(c) 2026 [cmdlinezero](https://cmdlinezero.dev)
