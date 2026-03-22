package proxy

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	_ "strings"
	_ "time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	HttpRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "devproxy_requests_total",
		Help: "Total requests by host",
	}, []string{"host", "status"})

	HttpRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "devproxy_duration_seconds",
		Help:    "Latency in seconds",
		Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5},
	}, []string{"host"})
)

func NewReverseProxy(target string) (*httputil.ReverseProxy, error) {
	url, err := url.Parse(target)
	if err != nil {
		return nil, err
	}
	proxy := httputil.NewSingleHostReverseProxy(url)
	
	// Optional: Customize transport for local dev performance
	proxy.Transport = &http.Transport{
		MaxIdleConnsPerHost: 100,
	}
	return proxy, nil
}
