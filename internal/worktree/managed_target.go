package worktree

import (
	"net/url"
	"strconv"
)

// ManagedTargetPort recognizes only the canonical target format emitted by
// worktree env. A matching target is not proof of worktree ownership.
func ManagedTargetPort(target string) (int, bool) {
	u, err := url.Parse(target)
	if err != nil || u.Scheme != "http" || u.Hostname() != "127.0.0.1" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" || u.Opaque != "" {
		return 0, false
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil || port < FirstPort || port >= FirstPort+PortCount {
		return 0, false
	}
	if u.Host != "127.0.0.1:"+strconv.Itoa(port) {
		return 0, false
	}
	return port, true
}
