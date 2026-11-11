package worktree

import (
	"net"
	"testing"
)

func TestCandidatePorts(t *testing.T) {
	first, err := CandidatePorts("/tmp/feature", nil, func(int) bool { return true })
	if err != nil {
		t.Fatal(err)
	}
	again, err := CandidatePorts("/tmp/feature", nil, func(int) bool { return true })
	if err != nil || first != again {
		t.Fatalf("unstable %d %d %v", first, again, err)
	}
	next, err := CandidatePorts("/tmp/feature", map[int]bool{first: true}, func(int) bool { return true })
	if err != nil || next == first {
		t.Fatalf("collision %d %v", next, err)
	}
	next2, err := CandidatePorts("/tmp/feature", nil, func(p int) bool { return p != first })
	if err != nil || next2 == first {
		t.Fatalf("busy port selected %d %v", next2, err)
	}
}
func TestLoopbackPort(t *testing.T) {
	for _, tc := range []struct {
		url  string
		port int
		ok   bool
	}{{"http://127.0.0.1:3456", 3456, true}, {"https://localhost:4433", 4433, true}, {"http://[::1]:9876", 9876, true}, {"http://192.168.1.2:1234", 0, false}, {"http://localhost", 0, false}, {"https://example.com:42", 0, false}} {
		p, ok := LoopbackPort(tc.url)
		if p != tc.port || ok != tc.ok {
			t.Errorf("%s => %d %v", tc.url, p, ok)
		}
	}
}
func TestPortAvailableBusy(t *testing.T) {
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	p := ln.Addr().(*net.TCPAddr).Port
	if PortAvailable(p) {
		t.Fatal("occupied port reported free")
	}
}
