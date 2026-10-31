package proxy

import (
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"

	"github.com/cmdlinezero/clz-lhd/internal/config"
	"github.com/cmdlinezero/clz-lhd/internal/status"

	clientmodel "github.com/prometheus/client_model/go"
)

func TestRouterRoutesKnownHost(t *testing.T) {
	t.Parallel()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("app"))
	}))
	defer upstream.Close()

	router := NewRouter([]config.Route{testRoute(t, "app.localhost", upstream.URL)})
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "http://app.localhost/hello", nil)

	router.ServeHTTP(recorder, req)

	if got, want := recorder.Code, http.StatusOK; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
	body, err := io.ReadAll(recorder.Result().Body)
	if err != nil {
		t.Fatalf("reading response body: %v", err)
	}
	if got, want := string(body), "app"; got != want {
		t.Errorf("body = %q, want %q", got, want)
	}
}

func TestRouterRoutesMultipleHosts(t *testing.T) {
	t.Parallel()

	first := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("first"))
	}))
	defer first.Close()
	second := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("second"))
	}))
	defer second.Close()

	router := NewRouter([]config.Route{
		testRoute(t, "first.localhost", first.URL),
		testRoute(t, "second.localhost", second.URL),
	})

	for _, tc := range []struct {
		host string
		want string
	}{
		{host: "first.localhost", want: "first"},
		{host: "second.localhost", want: "second"},
	} {
		t.Run(tc.host, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "http://"+tc.host+"/", nil)
			router.ServeHTTP(recorder, req)

			if got := recorder.Body.String(); got != tc.want {
				t.Errorf("body = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestRouterIgnoresPortInHostHeader(t *testing.T) {
	t.Parallel()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer upstream.Close()

	router := NewRouter([]config.Route{testRoute(t, "app.localhost", upstream.URL)})
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "http://app.localhost:8080/", nil)

	router.ServeHTTP(recorder, req)

	if got, want := recorder.Code, http.StatusNoContent; got != want {
		t.Errorf("status = %d, want %d", got, want)
	}
}

func TestRouterRejectsUnknownHost(t *testing.T) {
	t.Parallel()

	router := NewRouter([]config.Route{testRoute(t, "app.localhost", "http://localhost:8080")})
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "http://unknown.localhost/", nil)

	router.ServeHTTP(recorder, req)

	if got, want := recorder.Code, http.StatusNotFound; got != want {
		t.Errorf("status = %d, want %d", got, want)
	}
}

func TestRouterRecordsActualUpstreamStatus(t *testing.T) {
	for _, status := range []int{http.StatusCreated, http.StatusNotFound, http.StatusInternalServerError} {
		t.Run(strconv.Itoa(status), func(t *testing.T) {
			host := "status-" + strconv.Itoa(status) + ".localhost"
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(status)
			}))
			defer upstream.Close()

			counter := HTTPRequestsTotal.WithLabelValues(host, strconv.Itoa(status))
			before := counterValue(t, counter)

			router := NewRouter([]config.Route{testRoute(t, host, upstream.URL)})
			recorder := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "http://"+host+"/", nil)
			router.ServeHTTP(recorder, req)

			if got, want := recorder.Code, status; got != want {
				t.Fatalf("status = %d, want %d", got, want)
			}
			if got, want := counterValue(t, counter), before+1; got != want {
				t.Errorf("request counter = %v, want %v", got, want)
			}
		})
	}
}

func testRoute(t *testing.T, host, target string) config.Route {
	t.Helper()
	targetURL, err := url.Parse(target)
	if err != nil {
		t.Fatalf("url.Parse(%q): %v", target, err)
	}
	return config.Route{Host: host, Target: targetURL}
}

type prometheusMetric interface {
	Write(*clientmodel.Metric) error
}

func counterValue(t *testing.T, metric prometheusMetric) float64 {
	t.Helper()

	var value clientmodel.Metric
	if err := metric.Write(&value); err != nil {
		t.Fatalf("writing metric: %v", err)
	}
	return value.GetCounter().GetValue()
}

func TestRouterRecordsBadGatewayForUnavailableUpstream(t *testing.T) {
	listener := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	target := listener.URL
	listener.Close()

	host := "unavailable.localhost"
	counter := HTTPRequestsTotal.WithLabelValues(host, strconv.Itoa(http.StatusBadGateway))
	before := counterValue(t, counter)

	router := NewRouter([]config.Route{testRoute(t, host, target)})
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "http://"+host+"/", nil)
	router.ServeHTTP(recorder, req)

	if got, want := recorder.Code, http.StatusBadGateway; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
	if got, want := counterValue(t, counter), before+1; got != want {
		t.Errorf("request counter = %v, want %v", got, want)
	}
}

func TestRouterTracksRequestsInFlight(t *testing.T) {
	host := "in-flight.localhost"
	started := make(chan struct{})
	release := make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-release
		w.WriteHeader(http.StatusNoContent)
	}))
	defer upstream.Close()

	gauge := HTTPRequestsInFlight.WithLabelValues(host)
	before := gaugeValue(t, gauge)
	router := NewRouter([]config.Route{testRoute(t, host, upstream.URL)})

	done := make(chan struct{})
	go func() {
		defer close(done)
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "http://"+host+"/", nil)
		router.ServeHTTP(recorder, req)
	}()

	<-started
	if got, want := gaugeValue(t, gauge), before+1; got != want {
		t.Errorf("in-flight gauge while proxying = %v, want %v", got, want)
	}

	close(release)
	<-done
	if got, want := gaugeValue(t, gauge), before; got != want {
		t.Errorf("in-flight gauge after request = %v, want %v", got, want)
	}
}

func TestRouterTracksUnknownHostInFlight(t *testing.T) {
	host := "unknown-in-flight.localhost"
	gauge := HTTPRequestsInFlight.WithLabelValues(host)
	before := gaugeValue(t, gauge)

	router := NewRouter(nil)
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "http://"+host+"/", nil)
	router.ServeHTTP(recorder, req)

	if got, want := gaugeValue(t, gauge), before; got != want {
		t.Errorf("in-flight gauge after unknown host = %v, want %v", got, want)
	}
}

func gaugeValue(t *testing.T, metric prometheusMetric) float64 {
	t.Helper()

	var value clientmodel.Metric
	if err := metric.Write(&value); err != nil {
		t.Fatalf("writing metric: %v", err)
	}
	return value.GetGauge().GetValue()
}

func TestRouterRecordsLiveStatusForSuccessfulRequest(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))
	defer upstream.Close()

	routes := []config.Route{testRoute(t, "app.localhost", upstream.URL)}
	liveStatus := status.New(routes)
	router := NewRouterWithStatus(routes, liveStatus)
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "http://app.localhost/", nil)

	router.ServeHTTP(recorder, req)

	snapshot := liveStatus.Snapshot()
	if got, want := snapshot.Requests, uint64(1); got != want {
		t.Fatalf("requests = %d, want %d", got, want)
	}
	if got := snapshot.InFlight; got != 0 {
		t.Fatalf("in flight = %d, want 0", got)
	}
	if len(snapshot.Routes) != 1 {
		t.Fatalf("routes = %d, want 1", len(snapshot.Routes))
	}
	route := snapshot.Routes[0]
	if got, want := route.Reachability, status.ReachabilityConnected; got != want {
		t.Errorf("reachability = %q, want %q", got, want)
	}
	if got, want := route.LastStatus, http.StatusCreated; got != want {
		t.Errorf("last status = %d, want %d", got, want)
	}
	if got, want := route.Requests, uint64(1); got != want {
		t.Errorf("route requests = %d, want %d", got, want)
	}
	if route.LastDuration <= 0 {
		t.Errorf("last duration = %v, want > 0", route.LastDuration)
	}
	if got := snapshot.Traffic[len(snapshot.Traffic)-1].Requests; got != 1 {
		t.Errorf("current traffic bucket = %d, want 1", got)
	}
}

func TestRouterRecordsRefusedUpstreamInLiveStatus(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	target := "http://" + listener.Addr().String()
	listener.Close()

	routes := []config.Route{testRoute(t, "app.localhost", target)}
	liveStatus := status.New(routes)
	router := NewRouterWithStatus(routes, liveStatus)
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "http://app.localhost/", nil)

	router.ServeHTTP(recorder, req)

	if got, want := recorder.Code, http.StatusBadGateway; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
	snapshot := liveStatus.Snapshot()
	if got, want := snapshot.Routes[0].Reachability, status.ReachabilityRefused; got != want {
		t.Errorf("reachability = %q, want %q", got, want)
	}
	if got, want := snapshot.Routes[0].LastStatus, http.StatusBadGateway; got != want {
		t.Errorf("last status = %d, want %d", got, want)
	}
}

func TestRouterTracksLiveInFlightRequest(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-release
		w.WriteHeader(http.StatusNoContent)
	}))
	defer upstream.Close()

	routes := []config.Route{testRoute(t, "app.localhost", upstream.URL)}
	liveStatus := status.New(routes)
	router := NewRouterWithStatus(routes, liveStatus)
	done := make(chan struct{})
	go func() {
		defer close(done)
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "http://app.localhost/", nil)
		router.ServeHTTP(recorder, req)
	}()

	<-started
	if got, want := liveStatus.Snapshot().InFlight, int64(1); got != want {
		t.Errorf("in flight while proxying = %d, want %d", got, want)
	}
	close(release)
	<-done
	if got := liveStatus.Snapshot().InFlight; got != 0 {
		t.Errorf("in flight after request = %d, want 0", got)
	}
}

func TestRouterRecordsUnknownHostInAggregateLiveTraffic(t *testing.T) {
	liveStatus := status.New(nil)
	router := NewRouterWithStatus(nil, liveStatus)
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "http://unknown.localhost/", nil)

	router.ServeHTTP(recorder, req)

	snapshot := liveStatus.Snapshot()
	if got, want := snapshot.Requests, uint64(1); got != want {
		t.Errorf("requests = %d, want %d", got, want)
	}
	if got := len(snapshot.Routes); got != 0 {
		t.Errorf("routes = %d, want 0", got)
	}
	if got := snapshot.Traffic[len(snapshot.Traffic)-1].Requests; got != 1 {
		t.Errorf("current traffic bucket = %d, want 1", got)
	}
}

type timeoutError struct{}

func (timeoutError) Error() string   { return "timeout" }
func (timeoutError) Timeout() bool   { return true }
func (timeoutError) Temporary() bool { return true }

func TestReachabilityForError(t *testing.T) {
	if got, want := reachabilityForError(timeoutError{}), status.ReachabilityTimeout; got != want {
		t.Errorf("timeout reachability = %q, want %q", got, want)
	}
	if got, want := reachabilityForError(net.ErrClosed), status.ReachabilityUnknown; got != want {
		t.Errorf("unknown reachability = %q, want %q", got, want)
	}
}

var _ net.Error = timeoutError{}
