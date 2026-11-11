// Package mcp implements a minimal read-only MCP server over newline-delimited
// JSON-RPC 2.0 messages on standard input and output.
package mcp

import (
 "bufio"
 "bytes"
 "context"
 "encoding/json"
 "errors"
 "fmt"
 "io"
 "net/http"
 "strconv"
 "strings"
 "time"
)

const protocolVersion = "2025-03-26"
const maxMessage = 1024 * 1024

type Server struct {
 Port string
 Client *http.Client
}

type request struct {
 JSONRPC string `json:"jsonrpc"`
 ID json.RawMessage `json:"id"`
 Method string `json:"method"`
 Params json.RawMessage `json:"params"`
}

type rpcError struct { Code int `json:"code"`; Message string `json:"message"` }
type response struct { JSONRPC string `json:"jsonrpc"`; ID json.RawMessage `json:"id"`; Result any `json:"result,omitempty"`; Error *rpcError `json:"error,omitempty"` }

func failure(id json.RawMessage, code int, msg string) response { if len(id)==0 {id=json.RawMessage("null")}; return response{JSONRPC:"2.0", ID:id, Error:&rpcError{code,msg}} }
func success(id json.RawMessage, result any) response { return response{JSONRPC:"2.0",ID:id,Result:result} }

// Serve processes one JSON-RPC message per line. Notifications never receive replies.
// Malformed messages receive JSON-RPC errors; stdout contains protocol messages only.
func (s Server) Serve(ctx context.Context, in io.Reader, out io.Writer) error {
 scanner:=bufio.NewScanner(in)
 scanner.Buffer(make([]byte,4096),maxMessage)
 writer:=bufio.NewWriter(out)
 for scanner.Scan() {
  line:=bytes.TrimSpace(scanner.Bytes()); if len(line)==0 {continue}
  reply, emit:=s.handle(ctx,line)
  if !emit {continue}
  encoded,err:=json.Marshal(reply); if err!=nil {return err}
  if _,err=writer.Write(append(encoded,'\n'));err!=nil{return err}
  if err=writer.Flush();err!=nil{return err}
 }
 return scanner.Err()
}

func (s Server) handle(ctx context.Context, raw []byte) (response,bool) {
 var r request
 if err:=json.Unmarshal(raw,&r);err!=nil {return failure(nil,-32700,"parse error"),true}
 if r.JSONRPC!="2.0" || r.Method=="" || (len(r.ID)>0 && !json.Valid(r.ID)) {return failure(r.ID,-32600,"invalid request"),true}
 if len(r.ID)>0 && string(r.ID)!="null" && r.ID[0]!='"' && (r.ID[0]<'0'||r.ID[0]>'9') && r.ID[0]!='-' {return failure(nil,-32600,"invalid request id"),true}
 if len(r.ID)==0 {return response{},false}
 switch r.Method {
 case "initialize":
  var p struct { ProtocolVersion string `json:"protocolVersion"` }
  if len(r.Params)>0 && json.Unmarshal(r.Params,&p)!=nil {return failure(r.ID,-32602,"invalid initialize params"),true}
  return success(r.ID,map[string]any{"protocolVersion":protocolVersion,"capabilities":map[string]any{"tools":map[string]any{}},"serverInfo":map[string]string{"name":"clz-lhd","version":"0.7.0-dev"}}),true
 case "ping": return success(r.ID,map[string]any{}),true
 case "tools/list": return success(r.ID,map[string]any{"tools":[]any{
  map[string]any{"name":"lhd_list_routes","description":"List current LHD routes and their ownership (read only)","inputSchema":map[string]any{"type":"object","properties":map[string]any{},"additionalProperties":false}},
  map[string]any{"name":"lhd_get_status","description":"Get current LHD listener, route health, and traffic status (read only)","inputSchema":map[string]any{"type":"object","properties":map[string]any{},"additionalProperties":false}},
 }}),true
 case "tools/call":
  var p struct {Name string `json:"name"`; Arguments json.RawMessage `json:"arguments"`}
  if err:=json.Unmarshal(r.Params,&p);err!=nil || p.Name=="" {return failure(r.ID,-32602,"invalid tool parameters"),true}
  var args map[string]json.RawMessage
  if len(p.Arguments)>0 {if err:=json.Unmarshal(p.Arguments,&args);err!=nil || len(args)>0 {return failure(r.ID,-32602,"tools take no arguments"),true}}
  var path string
  switch p.Name {case "lhd_list_routes":path="/v1/routes";case "lhd_get_status":path="/v1/status";default:return failure(r.ID,-32602,"unknown tool"),true}
  data,err:=s.readAPI(ctx,path)
  if err!=nil {return success(r.ID,map[string]any{"content":[]any{map[string]string{"type":"text","text":err.Error()}},"isError":true}),true}
  return success(r.ID,map[string]any{"content":[]any{map[string]string{"type":"text","text":string(data)}}}),true
 default: return failure(r.ID,-32601,"method not found"),true
 }
}

func (s Server) readAPI(ctx context.Context,path string) ([]byte,error) {
 port,err:=strconv.Atoi(s.Port);if err!=nil || port<1 || port>65535 || strings.Trim(s.Port,"0123456789")!="" {return nil,errors.New("invalid status port")}
 req,err:=http.NewRequestWithContext(ctx,http.MethodGet,fmt.Sprintf("http://127.0.0.1:%d%s",port,path),nil);if err!=nil{return nil,err}
 client:=s.Client;if client==nil {client=&http.Client{Timeout:5*time.Second}}
 resp,err:=client.Do(req);if err!=nil{return nil,fmt.Errorf("LHD status API unavailable: %w",err)}
 defer resp.Body.Close()
 if resp.StatusCode!=http.StatusOK {return nil,fmt.Errorf("LHD status API returned %s",resp.Status)}
 body,err:=io.ReadAll(io.LimitReader(resp.Body,maxMessage+1));if err!=nil{return nil,err};if len(body)>maxMessage{return nil,errors.New("LHD status response too large")}
 if !json.Valid(body) {return nil,errors.New("LHD status API returned invalid JSON")}
 return body,nil
}
