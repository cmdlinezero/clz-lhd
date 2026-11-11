package cmd

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cmdlinezero/clz-lhd/internal/proxy"
	"github.com/cmdlinezero/clz-lhd/internal/status"
)

func TestSharedRuntimeRegistryAcrossHTTPAndHTTPS(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusAccepted) }))
	defer upstream.Close()
	registry := proxy.NewRouteRegistry(nil)
	live := status.New(nil)
	recorder := proxy.NewLifecycleRecorder(8)
	httpHandler := newHTTPHandlerWithRegistry(registry, live, recorder)
	httpsHandler := proxy.NewRouterWithRegistry(registry, live, recorder)
	check := func(handler http.Handler, want int) {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, "http://demo.localhost/", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != want {
			t.Fatalf("status = %d, want %d", w.Code, want)
		}
	}
	check(httpHandler, http.StatusNotFound)
	if err := registry.Create("demo.localhost=" + upstream.URL); err != nil {
		t.Fatal(err)
	}
	route, _ := registry.Lookup("demo.localhost")
	live.AddRoute(route)
	check(httpHandler, http.StatusAccepted)
	check(httpsHandler, http.StatusAccepted)
	if err := registry.Remove("demo.localhost"); err != nil {
		t.Fatal(err)
	}
	live.RemoveRoute("demo.localhost")
	check(httpHandler, http.StatusNotFound)
	check(httpsHandler, http.StatusNotFound)
}
