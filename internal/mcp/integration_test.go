package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http/httptest"
	"testing"

	"github.com/cmdlinezero/clz-lhd/internal/proxy"
	"github.com/cmdlinezero/clz-lhd/internal/status"
	"github.com/cmdlinezero/clz-lhd/internal/statushttp"
)

// Exercises MCP -> real management handler -> registry -> GET /v1/routes.
// Unlike a mocked mutation endpoint, this detects missing persistence and
// incorrect owner/source serialization.
func TestAgentMutationLifecycleThroughManagementAPI(t *testing.T) {
	registry := proxy.NewRouteRegistry(nil)
	if err := registry.CreateOwned("manual.localhost=http://127.0.0.1:23001", proxy.OwnerManual); err != nil {
		t.Fatal(err)
	}
	if err := registry.CreateOwned("work.localhost=http://127.0.0.1:23002", proxy.OwnerWorktree); err != nil {
		t.Fatal(err)
	}
	api := httptest.NewServer(statushttp.NewScopedManagementHandler(status.New(nil), proxy.NewLifecycleRecorder(8), registry, "admin-secret", "agent-secret"))
	defer api.Close()
	port := fmt.Sprint(api.Listener.Addr().(*net.TCPAddr).Port)
	agent := Server{Port: port, AgentToken: "agent-secret"}
	call := func(name, arguments string, wantError bool) {
		t.Helper()
		input := fmt.Sprintf(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":%q,"arguments":%s}}`, name, arguments) + "\n"
		replies := run(t, agent, input)
		if len(replies) != 1 {
			t.Fatalf("%s replies: %v", name, replies)
		}
		if replies[0]["error"] != nil {
			t.Fatalf("%s protocol error: %v", name, replies[0])
		}
		result := replies[0]["result"].(map[string]any)
		if (result["isError"] == true) != wantError {
			t.Fatalf("%s error mismatch: %v", name, result)
		}
	}
	check := func(target string, present bool) {
		t.Helper()
		data, err := agent.readAPI(context.Background(), "/v1/routes")
		if err != nil {
			t.Fatal(err)
		}
		var result struct {
			Routes []struct{ Host, Target, Source, Owner string } `json:"routes"`
		}
		if err := json.Unmarshal(data, &result); err != nil {
			t.Fatal(err)
		}
		found := false
		for _, route := range result.Routes {
			if route.Host == "mcp-test.localhost" {
				found = true
				if route.Target != target || route.Source != "runtime" || route.Owner != "agent" {
					t.Fatalf("incorrect agent route: %+v", route)
				}
			}
		}
		if found != present {
			t.Fatalf("agent route presence=%v want=%v: %s", found, present, data)
		}
	}
	call("lhd_create_route", `{"host":"mcp-test.localhost","target":"http://127.0.0.1:34567"}`, false)
	check("http://127.0.0.1:34567", true)
	call("lhd_update_route", `{"host":"mcp-test.localhost","target":"http://127.0.0.1:34568"}`, false)
	check("http://127.0.0.1:34568", true)
	for _, host := range []string{"manual.localhost", "work.localhost"} {
		call("lhd_update_route", fmt.Sprintf(`{"host":%q,"target":"http://127.0.0.1:34568"}`, host), true)
		call("lhd_remove_route", fmt.Sprintf(`{"host":%q}`, host), true)
		if _, ok := registry.Lookup(host); !ok {
			t.Fatalf("cross-owner route removed: %s", host)
		}
	}
	call("lhd_create_route", `{"host":"remote.localhost","target":"http://example.com:3000"}`, true)
	call("lhd_remove_route", `{"host":"mcp-test.localhost"}`, false)
	check("", false)
	wrong := Server{Port: port, AgentToken: "wrong-token"}
	replies := run(t, wrong, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"lhd_create_route","arguments":{"host":"denied.localhost","target":"http://127.0.0.1:34567"}}}`+"\n")
	if replies[0]["result"].(map[string]any)["isError"] != true {
		t.Fatal(replies)
	}
	if _, ok := registry.Lookup("denied.localhost"); ok {
		t.Fatal("unauthorized route created")
	}
	// Read-only MCP must not advertise write tools, even with a running API.
	readOnly := run(t, Server{Port: port}, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`+"\n")
	if got := len(readOnly[0]["result"].(map[string]any)["tools"].([]any)); got != 2 {
		t.Fatalf("read-only tools = %d", got)
	}
}
