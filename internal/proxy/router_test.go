package proxy

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"

	"github.com/cmdlinezero/clz-lhd/internal/config"

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

			counter := HttpRequestsTotal.WithLabelValues(host, strconv.Itoa(status))
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
	counter := HttpRequestsTotal.WithLabelValues(host, strconv.Itoa(http.StatusBadGateway))
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
