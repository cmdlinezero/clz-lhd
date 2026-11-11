package worktree

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
)

const (
	FirstPort = 20000
	PortCount = 30000
)

// CandidatePorts returns a stable permutation of the development-port range.
// It avoids known route targets but does not reserve a TCP port.
func CandidatePorts(root string, used map[int]bool, available func(int) bool) (int, error) {
	sum := sha256.Sum256([]byte(root))
	start := int(binary.BigEndian.Uint32(sum[:4]) % PortCount)
	// A step coprime to PortCount visits every candidate exactly once.
	step := 1 + int(binary.BigEndian.Uint32(sum[4:8])%uint32(PortCount-1))
	for gcd(step, PortCount) != 1 {
		step++
		if step >= PortCount {
			step = 1
		}
	}
	for i := 0; i < PortCount; i++ {
		port := FirstPort + (start+i*step)%PortCount
		if !used[port] && available(port) {
			return port, nil
		}
	}
	return 0, fmt.Errorf("no available worktree ports in %d-%d", FirstPort, FirstPort+PortCount-1)
}
func gcd(a, b int) int {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}

// LoopbackPort recognizes only literal loopback HTTP(S) route targets.
func LoopbackPort(target string) (int, bool) {
	u, err := url.Parse(target)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return 0, false
	}
	host := u.Hostname()
	if host != "localhost" && host != "127.0.0.1" && host != "::1" {
		return 0, false
	}
	p := u.Port()
	if p == "" {
		return 0, false
	}
	n, err := strconv.Atoi(p)
	return n, err == nil && n > 0 && n <= 65535
}

// PortAvailable probes both IPv4 and IPv6 loopback listeners where supported.
// This is a best-effort check, not a reservation.
func PortAvailable(port int) bool {
	p := strconv.Itoa(port)
	for _, networkAndAddress := range [][2]string{{"tcp4", "127.0.0.1:" + p}, {"tcp6", "[::1]:" + p}} {
		ln, err := net.Listen(networkAndAddress[0], networkAndAddress[1])
		if err != nil {
			// An unsupported IPv6 stack is not a port conflict.
			if networkAndAddress[0] == "tcp6" && (strings.Contains(err.Error(), "address family not supported") || strings.Contains(err.Error(), "protocol not supported")) {
				continue
			}
			return false
		}
		_ = ln.Close()
	}
	return true
}
