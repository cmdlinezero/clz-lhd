# CLZ Local Host Development (LHD)

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
complete with built-in Prometheus metrics and an admin dashboard.

Usage:
  clz-lhd [command]

Available Commands:
  completion  Generate the autocompletion script for the specified shell
  dash        Start proxy with Web UI dashboard
  help        Help about any command
  route       Start the proxy with host mappings

Flags:
  -h, --help          help for clz-lhd
  -p, --port string   The port the proxy listens on (default "80")

Use "clz-lhd [command] --help" for more information about a command.
```

(c) 2026 [cmdlinezero](https://cmdlinezero.dev)

