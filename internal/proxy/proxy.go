package proxy

import (
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	HTTPRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "lhd_requests_total",
		Help: "Total requests by host",
	}, []string{"host", "status"})

	HTTPRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "lhd_request_duration_seconds",
		Help:    "HTTP request duration in seconds",
		Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5},
	}, []string{"host"})

	HTTPRequestsInFlight = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "lhd_requests_in_flight",
		Help: "Current HTTP requests by host",
	}, []string{"host"})
)

func NewReverseProxy(target string) (*httputil.ReverseProxy, error) {
	url, err := url.Parse(target)
	if err != nil {
		return nil, err
	}
	proxy := httputil.NewSingleHostReverseProxy(url)
	director := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalHost := req.Host
		originalProto := requestScheme(req)
		director(req)
		req.Header.Set("X-Forwarded-Host", originalHost)
		req.Header.Set("X-Forwarded-Proto", originalProto)
	}
	proxy.ErrorHandler = func(w http.ResponseWriter, _ *http.Request, _ error) {
		http.Error(w, "Proxy: upstream unavailable", http.StatusBadGateway)
	}

	// Optional: Customize transport for local dev performance
	proxy.Transport = &http.Transport{
		MaxIdleConnsPerHost: 100,
	}
	return proxy, nil
}

func requestScheme(req *http.Request) string {
	if req.TLS != nil {
		return "https"
	}
	return "http"
}
