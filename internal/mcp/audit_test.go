package mcp

import (
 "context"
 "encoding/json"
 "fmt"
 "net"
 "net/http"
 "net/http/httptest"
 "strings"
 "sync"
 "testing"

 "github.com/cmdlinezero/clz-lhd/internal/proxy"
 "github.com/cmdlinezero/clz-lhd/internal/status"
 "github.com/cmdlinezero/clz-lhd/internal/statushttp"
)

// Invalid tool arguments must fail at the MCP boundary, before an HTTP request.
func TestAuditMalformedWriteCallsNeverReachAPI(t *testing.T) {
 var mu sync.Mutex
 hits := 0
 api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
  mu.Lock(); hits++; mu.Unlock()
  w.WriteHeader(http.StatusNoContent)
 }))
 defer api.Close()
 port := fmt.Sprint(api.Listener.Addr().(*net.TCPAddr).Port)
 inputs := []string{
  `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"lhd_create_route","arguments":{}}}`,
  `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"lhd_create_route","arguments":{"host":"x.localhost","target":123}}}`,
  `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"lhd_remove_route","arguments":{"host":"x.localhost","owner":"manual"}}}`,
  `{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"lhd_update_route","arguments":null}}`,
  `{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"lhd_create_route","arguments":{"host":"","target":"http://127.0.0.1:23000"}}}`,
 }
 replies := run(t, Server{Port:port, AgentToken:"agent-secret"}, strings.Join(inputs,"\n")+"\n")
 if len(replies)!=len(inputs) { t.Fatalf("got %d replies",len(replies)) }
 for i, reply := range replies {
  e,ok := reply["error"].(map[string]any)
  if !ok || e["code"] != float64(-32602) { t.Fatalf("case %d: %v",i,reply) }
 }
 mu.Lock(); defer mu.Unlock()
 if hits!=0 { t.Fatalf("malformed calls reached API %d times",hits) }
}

func TestAuditReadOnlyRejectsWriteEvenWithLiveAPI(t *testing.T) {
 registry := proxy.NewRouteRegistry(nil)
 api := httptest.NewServer(statushttp.NewScopedManagementHandler(status.New(nil),proxy.NewLifecycleRecorder(8),registry,"admin-secret","agent-secret"))
 defer api.Close()
 port := fmt.Sprint(api.Listener.Addr().(*net.TCPAddr).Port)
 s := Server{Port:port}
 replies := run(t,s,`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"lhd_create_route","arguments":{"host":"blocked.localhost","target":"http://127.0.0.1:23000"}}}`+"\n")
 if len(replies)!=1 || replies[0]["error"]==nil { t.Fatalf("write not rejected: %v",replies) }
 if _,ok:=registry.Lookup("blocked.localhost");ok { t.Fatal("read-only MCP created a route") }
}

func TestAuditConcurrentAgentCreateSingleOwner(t *testing.T) {
 registry := proxy.NewRouteRegistry(nil)
 api := httptest.NewServer(statushttp.NewScopedManagementHandler(status.New(nil),proxy.NewLifecycleRecorder(8),registry,"admin-secret","agent-secret"))
 defer api.Close()
 port := fmt.Sprint(api.Listener.Addr().(*net.TCPAddr).Port)
 s := Server{Port:port,AgentToken:"agent-secret"}
 const n=12
 var wg sync.WaitGroup
 var mu sync.Mutex
 successes:=0
 for i:=0;i<n;i++ {
  wg.Add(1)
  go func(i int) {
   defer wg.Done()
   host:="audit-race.localhost"
   target:=fmt.Sprintf("http://127.0.0.1:%d",23000+i)
   payload,_:=json.Marshal(map[string]any{"jsonrpc":"2.0","id":i+1,"method":"tools/call","params":map[string]any{"name":"lhd_create_route","arguments":map[string]string{"host":host,"target":target}}})
   reply,emit:=s.handle(context.Background(),payload)
   if !emit { t.Errorf("missing reply for %d",i);return }
   result,ok:=reply.Result.(map[string]any)
   if !ok { t.Errorf("unexpected response %d: %+v",i,reply);return }
   if result["isError"]!=true { mu.Lock();successes++;mu.Unlock() }
  }(i)
 }
 wg.Wait()
 if successes!=1 { t.Fatalf("concurrent creates succeeded %d times, want exactly one",successes) }
 entry,ok:=registry.Lookup("audit-race.localhost")
 if !ok || entry.Target==nil { t.Fatalf("missing route after concurrent creates: %+v",entry) }
 data,err:=s.readAPI(context.Background(),"/v1/routes")
 if err!=nil || !strings.Contains(string(data),`"owner":"agent"`) { t.Fatalf("API ownership: %s %v",data,err) }
}
