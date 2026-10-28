# CLZ Local Host Development (LHD)

![Beta](https://img.shields.io/badge/Release%20Status-Beta-red)
![Tag](https://img.shields.io/github/v/tag/cmdlinezero/clz-lhd?label=Tag)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](/LICENSE)
![Go](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go&logoColor=white)
![Platforms](https://img.shields.io/badge/Platform-Windows%20%7C%20Linux%20%7C%20MacOS-green)

A simple tool to help developers who need to map to multiple ports on their local host.

## Getting Started 
---

1. Port map 8080 to app.localhost

   ```bash
   ./clz-lhd route -r app.localhost=8080
   ```

> Open your browser and visit http://app.localhost to view localhost:8080

## Usage
---


```
clz-lhd is a CLI tool designed to simplify local host development (LHD) by
mapping custom local domains (like api.localhost) to specific internal ports,
with built-in Prometheus metrics for local observability.

Usage:
  clz-lhd [command]

Available Commands:
  completion  Generate the autocompletion script for the specified shell
  help        Help about any command
  route       Start the proxy with host mappings

Flags:
  -h, --help          help for clz-lhd
  -p, --port string   The port the proxy listens on (default "80")

Use "clz-lhd [command] --help" for more information about a command.
```

## Metrics

While the proxy is running, Prometheus metrics are available at `http://localhost:<port>/metrics`.

LHD exposes these application metrics:

- `lhd_requests_total{host,status}` — total proxied requests, labelled by host and observed HTTP response status. Upstream connection failures are recorded as status `502`.
- `lhd_request_duration_seconds{host}` — histogram of proxy request duration by host.
- `lhd_requests_in_flight{host}` — requests currently being handled by host.

(c) 2026 [cmdlinezero](https://cmdlinezero.dev)

