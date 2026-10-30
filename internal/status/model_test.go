package status

import (
	"net/url"
	"testing"
	"time"

	"github.com/cmdlinezero/clz-lhd/internal/config"
)

func testRoute(host, target string) config.Route {
	parsed, err := url.Parse(target)
	if err != nil {
		panic(err)
	}
	return config.Route{Host: host, Target: parsed}
}

func TestNewSnapshotSortsConfiguredRoutes(t *testing.T) {
	m := New([]config.Route{
		testRoute("z.localhost", "http://localhost:3000"),
		testRoute("a.localhost", "https://localhost:4000"),
	})
	m.now = func() time.Time { return time.Unix(100, 0) }

	snapshot := m.Snapshot()
	if got, want := len(snapshot.Routes), 2; got != want {
		t.Fatalf("len(Routes) = %d, want %d", got, want)
	}
	if got, want := snapshot.Routes[0].Host, "a.localhost"; got != want {
		t.Errorf("Routes[0].Host = %q, want %q", got, want)
	}
	if got, want := snapshot.Routes[0].Target, "https://localhost:4000"; got != want {
		t.Errorf("Routes[0].Target = %q, want %q", got, want)
	}
	if got, want := snapshot.Routes[0].Reachability, ReachabilityUnknown; got != want {
		t.Errorf("Routes[0].Reachability = %q, want %q", got, want)
	}
}

func TestModelRecordsListenerAndReachabilityState(t *testing.T) {
	now := time.Unix(200, 0)
	m := New([]config.Route{testRoute("app.localhost", "http://localhost:3000")})
	m.now = func() time.Time { return now }

	m.SetListener("http", ":80", true)
	m.SetListener("https", ":8443", true)
	m.SetReachability("app.localhost", ReachabilityConnected)

	snapshot := m.Snapshot()
	if got, want := snapshot.HTTP, (Listener{Address: ":80", Listening: true}); got != want {
		t.Errorf("HTTP = %#v, want %#v", got, want)
	}
	if got, want := snapshot.HTTPS, (Listener{Address: ":8443", Listening: true}); got != want {
		t.Errorf("HTTPS = %#v, want %#v", got, want)
	}
	if got, want := snapshot.Routes[0].Reachability, ReachabilityConnected; got != want {
		t.Errorf("Reachability = %q, want %q", got, want)
	}
	if got, want := snapshot.Routes[0].LastChange, now; !got.Equal(want) {
		t.Errorf("LastChange = %v, want %v", got, want)
	}
}

func TestModelRecordsRequestActivity(t *testing.T) {
	now := time.Unix(300, 250_000_000)
	m := New([]config.Route{testRoute("app.localhost", "http://localhost:3000")})
	m.now = func() time.Time { return now }

	m.RequestStarted()
	m.RequestStarted()
	m.RequestCompleted("app.localhost", 201, 12*time.Millisecond)

	snapshot := m.Snapshot()
	if got, want := snapshot.InFlight, int64(1); got != want {
		t.Errorf("InFlight = %d, want %d", got, want)
	}
	if got, want := snapshot.Requests, uint64(1); got != want {
		t.Errorf("Requests = %d, want %d", got, want)
	}
	route := snapshot.Routes[0]
	if got, want := route.Requests, uint64(1); got != want {
		t.Errorf("route Requests = %d, want %d", got, want)
	}
	if got, want := route.LastStatus, 201; got != want {
		t.Errorf("LastStatus = %d, want %d", got, want)
	}
	if got, want := route.LastDuration, 12*time.Millisecond; got != want {
		t.Errorf("LastDuration = %v, want %v", got, want)
	}
	if got, want := route.LastRequest, now; !got.Equal(want) {
		t.Errorf("LastRequest = %v, want %v", got, want)
	}
	if got, want := snapshot.Traffic[len(snapshot.Traffic)-1].Requests, uint64(1); got != want {
		t.Errorf("current traffic bucket = %d, want %d", got, want)
	}
}

func TestTrafficWindowIsBoundedAndFillsGaps(t *testing.T) {
	now := time.Unix(1000, 0)
	m := New(nil)
	m.now = func() time.Time { return now }

	m.RequestCompleted("unknown.localhost", 404, time.Millisecond)
	now = now.Add(2 * time.Second)
	m.RequestCompleted("unknown.localhost", 404, time.Millisecond)

	snapshot := m.Snapshot()
	if got, want := len(snapshot.Traffic), 60; got != want {
		t.Fatalf("len(Traffic) = %d, want %d", got, want)
	}
	last := len(snapshot.Traffic) - 1
	if got, want := snapshot.Traffic[last].Requests, uint64(1); got != want {
		t.Errorf("latest bucket = %d, want %d", got, want)
	}
	if got, want := snapshot.Traffic[last-1].Requests, uint64(0); got != want {
		t.Errorf("gap bucket = %d, want %d", got, want)
	}
	if got, want := snapshot.Traffic[last-2].Requests, uint64(1); got != want {
		t.Errorf("earlier bucket = %d, want %d", got, want)
	}

	now = now.Add(60 * time.Second)
	snapshot = m.Snapshot()
	for _, bucket := range snapshot.Traffic {
		if bucket.Requests != 0 {
			t.Fatalf("expired traffic bucket at %v has %d requests", bucket.At, bucket.Requests)
		}
	}
}

func TestUnknownRouteDoesNotExpandConfiguredRouteState(t *testing.T) {
	m := New([]config.Route{testRoute("app.localhost", "http://localhost:3000")})
	m.now = func() time.Time { return time.Unix(400, 0) }

	m.SetReachability("unknown.localhost", ReachabilityRefused)
	m.RequestCompleted("unknown.localhost", 404, time.Millisecond)

	snapshot := m.Snapshot()
	if got, want := len(snapshot.Routes), 1; got != want {
		t.Fatalf("len(Routes) = %d, want %d", got, want)
	}
	if got, want := snapshot.Requests, uint64(1); got != want {
		t.Errorf("Requests = %d, want %d", got, want)
	}
}
