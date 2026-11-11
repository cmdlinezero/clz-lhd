package mcp

import (
 "context"
 "encoding/json"
 "fmt"
 "net"
 "net/http"
 "net/http/httptest"
 "strings"
 "bytes"
 "testing"
)

func run(t *testing.T,s Server,input string) []map[string]any {
 t.Helper();var output bytes.Buffer
 if err:=s.Serve(context.Background(),strings.NewReader(input),&output);err!=nil {t.Fatal(err)}
 var result []map[string]any
 for _,line:=range strings.Split(strings.TrimSpace(output.String()),"\n") {if line=="" {continue};var item map[string]any;if err:=json.Unmarshal([]byte(line),&item);err!=nil {t.Fatal(err)};result=append(result,item)}
 return result
}

func TestProtocol(t *testing.T) {
 s:=Server{Port:"9099"}
 results:=run(t,s,"{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"initialize\",\"params\":{\"protocolVersion\":\"2025-03-26\"}}\n"+
 "{\"jsonrpc\":\"2.0\",\"method\":\"notifications/initialized\"}\n"+
 "{\"jsonrpc\":\"2.0\",\"id\":2,\"method\":\"tools/list\"}\n"+
 "{\"jsonrpc\":\"2.0\",\"id\":3,\"method\":\"tools/call\",\"params\":{\"name\":\"lhd_remove_route\"}}\n"+
 "garbage\n")
 if len(results)!=4 {t.Fatalf("got %d replies",len(results))}
 if results[0]["result"].(map[string]any)["protocolVersion"]!=protocolVersion {t.Fatal(results[0])}
 tools:=results[1]["result"].(map[string]any)["tools"].([]any)
 if len(tools)!=2 {t.Fatalf("unexpected tools: %v",tools)}
 if results[2]["error"].(map[string]any)["code"]!=float64(-32602) {t.Fatal(results[2])}
 if results[3]["error"].(map[string]any)["code"]!=float64(-32700) {t.Fatal(results[3])}
}

func TestReadOnlyAPICalls(t *testing.T) {
 methods:=[]string{}
 api:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){methods=append(methods,r.Method+" "+r.URL.Path);w.Header().Set("Content-Type","application/json");fmt.Fprint(w,`{"routes":[{"host":"demo.localhost","owner":"worktree"}]}`)}))
 defer api.Close()
 addr:=api.Listener.Addr().(*net.TCPAddr)
 s:=Server{Port:fmt.Sprint(addr.Port)}
 results:=run(t,s,`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"lhd_list_routes","arguments":{}}}`+"\n"+`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"lhd_get_status"}}`+"\n")
 if len(results)!=2 || len(methods)!=2 || methods[0]!="GET /v1/routes" || methods[1]!="GET /v1/status" {t.Fatalf("results %v methods %v",results,methods)}
 text:=results[0]["result"].(map[string]any)["content"].([]any)[0].(map[string]any)["text"].(string)
 if !strings.Contains(text,`"owner":"worktree"`) {t.Fatal(text)}
}

func TestFailureAndValidation(t *testing.T) {
 results:=run(t,Server{Port:"not-a-port"},`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"lhd_get_status"}}`+"\n"+`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"lhd_get_status","arguments":{"unexpected":true}}}`+"\n")
 if results[0]["result"].(map[string]any)["isError"]!=true {t.Fatal(results[0])}
 if results[1]["error"].(map[string]any)["code"]!=float64(-32602) {t.Fatal(results[1])}
}
