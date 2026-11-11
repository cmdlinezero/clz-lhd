package proxy

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/cmdlinezero/clz-lhd/internal/config"
)

func TestRegistryOwnershipAndValidation(t *testing.T) {
	configured, err := config.ParseRoute("app.localhost=http://localhost:8080")
	if err != nil {
		t.Fatal(err)
	}
	r := NewRouteRegistry([]config.Route{configured})
	if err := r.Create("APP.LOCALHOST=http://localhost:9000"); !errors.Is(err, ErrConfiguredRoute) {
		t.Fatalf("create configured: %v", err)
	}
	if err := r.Update("app.localhost=http://localhost:9000"); !errors.Is(err, ErrConfiguredRoute) {
		t.Fatalf("update configured: %v", err)
	}
	if err := r.Remove("app.localhost"); !errors.Is(err, ErrConfiguredRoute) {
		t.Fatalf("remove configured: %v", err)
	}
	if err := r.Create("other.example.com=http://localhost:9000"); err != nil {
		t.Fatal(err)
	}
	if err := r.Create("other.example.com=http://localhost:9001"); !errors.Is(err, ErrRouteExists) {
		t.Fatalf("duplicate: %v", err)
	}
	if err := r.Update("other.example.com=http://localhost:9001"); err != nil {
		t.Fatal(err)
	}
	route, ok := r.Lookup("other.example.com")
	if !ok || route.Target.String() != "http://localhost:9001" {
		t.Fatalf("updated route: %+v %v", route, ok)
	}
	route.Target.Host = "modified"
	again, _ := r.Lookup("other.example.com")
	if again.Target.Host == "modified" {
		t.Fatal("lookup leaked mutable URL")
	}
	if err := r.Create("bad_host=http://localhost"); err == nil {
		t.Fatal("invalid hostname accepted")
	}
	if err := r.Remove("OTHER.EXAMPLE.COM."); err != nil {
		t.Fatal(err)
	}
	if _, ok := r.Lookup("other.example.com"); ok {
		t.Fatal("removed route still exists")
	}
	if err := r.Remove("other.example.com"); !errors.Is(err, ErrRouteNotFound) {
		t.Fatalf("missing route: %v", err)
	}
	if len(r.Snapshot()) != 1 {
		t.Fatal("unexpected snapshot size")
	}
}

func TestRegistrySharedRoutersAndInFlight(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		close(entered)
		<-release
		_, _ = io.WriteString(w, "old")
	}))
	defer upstream.Close()
	r := NewRouteRegistry(nil)
	if err := r.Create("work.localhost=" + upstream.URL); err != nil {
		t.Fatal(err)
	}
	router := NewRouterWithRegistry(r, nil, nil)
	done := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "http://work.localhost/", nil))
		done <- w
	}()
	<-entered
	if err := r.Remove("work.localhost"); err != nil {
		t.Fatal(err)
	}
	missing := httptest.NewRecorder()
	router.ServeHTTP(missing, httptest.NewRequest(http.MethodGet, "http://work.localhost/", nil))
	if missing.Code != http.StatusNotFound {
		t.Fatalf("after removal: %d", missing.Code)
	}
	close(release)
	finished := <-done
	if finished.Code != http.StatusOK || finished.Body.String() != "old" {
		t.Fatalf("in flight: %d %q", finished.Code, finished.Body.String())
	}
}

func TestRegistryConcurrentAccess(t *testing.T) {
	r := NewRouteRegistry(nil)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				_ = r.Create("work.localhost=http://localhost:3000")
				_ = r.Update("work.localhost=http://localhost:3001")
				_, _ = r.Lookup("work.localhost")
				_ = r.Snapshot()
				_ = r.Remove("work.localhost")
			}
		}()
	}
	wg.Wait()
}

func TestRegistryExplicitOwnership(t *testing.T) {
	r := NewRouteRegistry(nil)
	const spec = "feature.localhost=http://127.0.0.1:23456"
	if err := r.CreateOwned(spec, OwnerWorktree); err != nil {
		t.Fatal(err)
	}
	if err := r.Update(spec); !errors.Is(err, ErrRouteOwner) {
		t.Fatalf("manual update: %v", err)
	}
	if err := r.Remove("feature.localhost"); !errors.Is(err, ErrRouteOwner) {
		t.Fatalf("manual remove: %v", err)
	}
	entries := r.SnapshotOwned()
	if len(entries) != 1 || entries[0].Owner != OwnerWorktree {
		t.Fatalf("snapshot: %+v", entries)
	}
	if err := r.RemoveOwned("feature.localhost", OwnerWorktree); err != nil {
		t.Fatal(err)
	}
	if err := r.CreateOwned(spec, "unknown"); !errors.Is(err, ErrInvalidOwner) {
		t.Fatalf("invalid owner: %v", err)
	}
}
