package mcp

import (
	"encoding/json"
	"net/http"
	"time"

	"xcloudflow/internal/store"
)

// Minimal JSON-RPC 2.0 handler that supports:
// - initialize
// - tools/list
// - tools/call
//
// This is intentionally small: enough to act as an MCP-like server on Cloud Run.

type ServerOptions struct {
	Store *store.Store
}

type Server struct {
	store *store.Store
	tools []Tool
}

type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"inputSchema,omitempty"`
}

func NewServer(opts ServerOptions) *Server {
	tools := []Tool{
		{
			Name:        "stackflow.validate",
			Description: "Validate StackFlow config (schema + constraints).",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"config_yaml":{"type":"string"},"env":{"type":"string"}},"required":["config_yaml"]}`),
		},
		{
			Name:        "stackflow.plan.dns",
			Description: "Generate DNS plan from StackFlow config.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"config_yaml":{"type":"string"},"env":{"type":"string"}},"required":["config_yaml"]}`),
		},
	}
	return &Server{store: opts.Store, tools: tools}
}

type rpcReq struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcResp struct {
	JSONRPC string  `json:"jsonrpc"`
	ID      any     `json:"id"`
	Result  any     `json:"result,omitempty"`
	Error   *rpcErr `json:"error,omitempty"`
}

type rpcErr struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var req rpcReq
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeJSON(w, rpcResp{JSONRPC: "2.0", ID: nil, Error: &rpcErr{Code: -32700, Message: "invalid JSON"}})
		return
	}
	if req.JSONRPC == "" {
		req.JSONRPC = "2.0"
	}

	switch req.Method {
	case "initialize":
		writeJSON(w, rpcResp{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{
			"server": map[string]any{
				"name":    "xcloudflow",
				"version": "0.1",
			},
			"capabilities": map[string]any{
				"tools": true,
			},
			"time": time.Now().UTC().Format(time.RFC3339),
		}})
		return

	case "tools/list":
		writeJSON(w, rpcResp{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{"tools": s.tools}})
		return

	case "tools/call":
		var p struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &p); err != nil || p.Name == "" {
			writeJSON(w, rpcResp{JSONRPC: "2.0", ID: req.ID, Error: &rpcErr{Code: -32602, Message: "invalid params"}})
			return
		}

		res, err := s.callTool(r.Context(), p.Name, p.Arguments)
		if err != nil {
			writeJSON(w, rpcResp{JSONRPC: "2.0", ID: req.ID, Error: &rpcErr{Code: -32000, Message: err.Error()}})
			return
		}
		writeJSON(w, rpcResp{JSONRPC: "2.0", ID: req.ID, Result: res})
		return
	default:
		writeJSON(w, rpcResp{JSONRPC: "2.0", ID: req.ID, Error: &rpcErr{Code: -32601, Message: "method not found"}})
		return
	}
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

