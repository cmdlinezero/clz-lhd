// Package mcp implements a MCP server over newline-delimited
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
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const protocolVersion = "2025-03-26"
const maxMessage = 1024 * 1024

type Server struct {
	Port   string
	Client *http.Client
	// AgentToken enables restricted write tools when nonempty.
	AgentToken string
}

type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}
type response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

func failure(id json.RawMessage, code int, msg string) response {
	if len(id) == 0 {
		id = json.RawMessage("null")
	}
	return response{JSONRPC: "2.0", ID: id, Error: &rpcError{code, msg}}
}
func success(id json.RawMessage, result any) response {
	return response{JSONRPC: "2.0", ID: id, Result: result}
}

// Serve processes one JSON-RPC message per line. Notifications never receive replies.
// Malformed messages receive JSON-RPC errors; stdout contains protocol messages only.
func (s Server) Serve(ctx context.Context, in io.Reader, out io.Writer) error {
	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 4096), maxMessage)
	writer := bufio.NewWriter(out)
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		reply, emit := s.handle(ctx, line)
		if !emit {
			continue
		}
		encoded, err := json.Marshal(reply)
		if err != nil {
			return err
		}
		if _, err = writer.Write(append(encoded, '\n')); err != nil {
			return err
		}
		if err = writer.Flush(); err != nil {
			return err
		}
	}
	return scanner.Err()
}

func (s Server) handle(ctx context.Context, raw []byte) (response, bool) {
	var r request
	if err := json.Unmarshal(raw, &r); err != nil {
		return failure(nil, -32700, "parse error"), true
	}
	if r.JSONRPC != "2.0" || r.Method == "" || (len(r.ID) > 0 && !json.Valid(r.ID)) {
		return failure(r.ID, -32600, "invalid request"), true
	}
	if len(r.ID) > 0 && string(r.ID) != "null" && r.ID[0] != '"' && (r.ID[0] < '0' || r.ID[0] > '9') && r.ID[0] != '-' {
		return failure(nil, -32600, "invalid request id"), true
	}
	if len(r.ID) == 0 {
		return response{}, false
	}
	switch r.Method {
	case "initialize":
		var p struct {
			ProtocolVersion string `json:"protocolVersion"`
		}
		if len(r.Params) > 0 && json.Unmarshal(r.Params, &p) != nil {
			return failure(r.ID, -32602, "invalid initialize params"), true
		}
		return success(r.ID, map[string]any{"protocolVersion": protocolVersion, "capabilities": map[string]any{"tools": map[string]any{}}, "serverInfo": map[string]string{"name": "clz-lhd", "version": "0.7.0-dev"}}), true
	case "ping":
		return success(r.ID, map[string]any{}), true
	case "tools/list":
		return success(r.ID, map[string]any{"tools": s.tools()}), true
	case "tools/call":
		var p struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if err := json.Unmarshal(r.Params, &p); err != nil || p.Name == "" {
			return failure(r.ID, -32602, "invalid tool parameters"), true
		}
		if p.Name == "lhd_create_route" || p.Name == "lhd_update_route" || p.Name == "lhd_remove_route" {
			if s.AgentToken == "" {
				return failure(r.ID, -32602, "unknown tool"), true
			}
			return s.writeTool(ctx, r.ID, p.Name, p.Arguments)
		}
		var args map[string]json.RawMessage
		if len(p.Arguments) > 0 {
			if err := json.Unmarshal(p.Arguments, &args); err != nil || args == nil || len(args) > 0 {
				return failure(r.ID, -32602, "tools take no arguments"), true
			}
		}
		var path string
		switch p.Name {
		case "lhd_list_routes":
			path = "/v1/routes"
		case "lhd_get_status":
			path = "/v1/status"
		default:
			return failure(r.ID, -32602, "unknown tool"), true
		}
		data, err := s.readAPI(ctx, path)
		if err != nil {
			return toolError(r.ID, err.Error()), true
		}
		return success(r.ID, map[string]any{"content": []any{map[string]string{"type": "text", "text": string(data)}}}), true

	default:
		return failure(r.ID, -32601, "method not found"), true
	}
}

func (s Server) tools() []any {
	tools := []any{
		map[string]any{"name": "lhd_list_routes", "description": "List current LHD routes and their ownership (read only)", "inputSchema": map[string]any{"type": "object", "properties": map[string]any{}, "additionalProperties": false}},
		map[string]any{"name": "lhd_get_status", "description": "Get current LHD listener, route health, and traffic status (read only)", "inputSchema": map[string]any{"type": "object", "properties": map[string]any{}, "additionalProperties": false}},
	}
	if s.AgentToken != "" {
		routeProps := map[string]any{"host": map[string]any{"type": "string"}, "target": map[string]any{"type": "string"}}
		for _, entry := range []struct{ name, description string }{{"lhd_create_route", "Create an agent-owned localhost route"}, {"lhd_update_route", "Update an agent-owned localhost route"}} {
			tools = append(tools, map[string]any{"name": entry.name, "description": entry.description, "inputSchema": map[string]any{"type": "object", "properties": routeProps, "required": []string{"host", "target"}, "additionalProperties": false}})
		}
		tools = append(tools, map[string]any{"name": "lhd_remove_route", "description": "Remove an agent-owned localhost route", "inputSchema": map[string]any{"type": "object", "properties": map[string]any{"host": map[string]any{"type": "string"}}, "required": []string{"host"}, "additionalProperties": false}})
	}
	return tools
}

func toolError(id json.RawMessage, message string) response {
	return success(id, map[string]any{"content": []any{map[string]string{"type": "text", "text": message}}, "isError": true})
}

func (s Server) writeTool(ctx context.Context, id json.RawMessage, name string, raw json.RawMessage) (response, bool) {
	var args map[string]json.RawMessage
	if len(raw) == 0 || json.Unmarshal(raw, &args) != nil || args == nil {
		return failure(id, -32602, "invalid tool arguments"), true
	}
	expected := 2
	if name == "lhd_remove_route" {
		expected = 1
	}
	if len(args) != expected {
		return failure(id, -32602, "unexpected tool arguments"), true
	}
	var host, target string
	if json.Unmarshal(args["host"], &host) != nil || host == "" {
		return failure(id, -32602, "host must be a nonempty string"), true
	}
	if expected == 2 && (json.Unmarshal(args["target"], &target) != nil || target == "") {
		return failure(id, -32602, "target must be a nonempty string"), true
	}
	var method, path string
	var body []byte
	switch name {
	case "lhd_create_route":
		method, path = http.MethodPost, "/v1/routes"
	case "lhd_update_route":
		method, path = http.MethodPut, "/v1/routes"
	case "lhd_remove_route":
		method, path = http.MethodDelete, "/v1/routes/"+url.PathEscape(host)+"?owner=agent"
	}
	if expected == 2 {
		body, _ = json.Marshal(map[string]string{"host": host, "target": target, "owner": "agent"})
	}
	data, err := s.callAPI(ctx, method, path, body, s.AgentToken)
	if err != nil {
		return toolError(id, err.Error()), true
	}
	_ = data
	return success(id, map[string]any{"content": []any{map[string]string{"type": "text", "text": "agent route mutation completed"}}}), true
}

func (s Server) readAPI(ctx context.Context, path string) ([]byte, error) {
	return s.callAPI(ctx, http.MethodGet, path, nil, "")
}

func (s Server) callAPI(ctx context.Context, method, path string, body []byte, token string) ([]byte, error) {
	port, err := strconv.Atoi(s.Port)
	if err != nil || port < 1 || port > 65535 || strings.Trim(s.Port, "0123456789") != "" {
		return nil, errors.New("invalid status port")
	}
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, fmt.Sprintf("http://127.0.0.1:%d%s", port, path), reader)
	if err != nil {
		return nil, err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
	}
	client := s.Client
	if client == nil {
		dialer := &net.Dialer{Timeout: 5 * time.Second}
		client = &http.Client{Timeout: 5 * time.Second, Transport: &http.Transport{Proxy: nil, DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			return dialer.DialContext(ctx, "tcp", fmt.Sprintf("127.0.0.1:%d", port))
		}}}
	}
	restricted := *client
	restricted.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := restricted.Do(req)
	if err != nil {
		return nil, fmt.Errorf("LHD status API unavailable: %w", err)
	}
	defer resp.Body.Close()
	if method != http.MethodGet && resp.StatusCode == http.StatusNoContent {
		return nil, nil
	}
	if method == http.MethodGet && resp.StatusCode != http.StatusOK || method != http.MethodGet {
		return nil, fmt.Errorf("LHD status API returned %s", resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxMessage+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxMessage {
		return nil, errors.New("LHD status response too large")
	}
	if !json.Valid(data) {
		return nil, errors.New("LHD status API returned invalid JSON")
	}
	return data, nil
}
