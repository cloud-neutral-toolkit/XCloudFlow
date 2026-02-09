package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/spf13/cobra"

	"xconfig/internal/xcfstore"
)

// Minimal MCP-like server for xconfig (exec engine).
// Exposes:
// - GET  /healthz
// - POST /mcp (JSON-RPC: initialize, tools/list, tools/call)
//
// This is a skeleton intended to be called by xcloud-server as an external MCP server.

type tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"inputSchema,omitempty"`
}

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "MCP server (xconfig as exec engine)",
}

var mcpServeCmd = &cobra.Command{
	Use:   "serve",
	Short: "Run MCP HTTP server (Cloud Run friendly)",
	RunE: func(cmd *cobra.Command, args []string) error {
		addr, _ := cmd.Flags().GetString("addr")
		if addr == "" {
			if p := os.Getenv("PORT"); p != "" {
				addr = ":" + p
			} else {
				addr = ":8081"
			}
		}

		var st *xcfstore.Store
		if DSN != "" {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			s, err := xcfstore.Open(ctx, DSN)
			if err != nil {
				return err
			}
			st = s
			defer st.Close()
		}

		tools := []tool{
			{
				Name:        "xconfig.ping",
				Description: "Health check tool for xconfig MCP server.",
				InputSchema: json.RawMessage(`{"type":"object","properties":{},"additionalProperties":false}`),
			},
			{
				Name:        "xconfig.playbook.run",
				Description: "Run a playbook (skeleton; returns not-implemented).",
				InputSchema: json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"},"inventory":{"type":"string"}},"required":["path"]}`),
			},
		}

		mux := http.NewServeMux()
		mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
		mux.HandleFunc("/mcp", func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			var req struct {
				JSONRPC string          `json:"jsonrpc"`
				ID      any             `json:"id"`
				Method  string          `json:"method"`
				Params  json.RawMessage `json:"params,omitempty"`
			}
			dec := json.NewDecoder(r.Body)
			dec.DisallowUnknownFields()
			if err := dec.Decode(&req); err != nil {
				_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": nil, "error": map[string]any{"code": -32700, "message": "invalid JSON"}})
				return
			}
			if req.JSONRPC == "" {
				req.JSONRPC = "2.0"
			}

			write := func(id any, result any, errMsg string) {
				w.Header().Set("Content-Type", "application/json")
				if errMsg != "" {
					_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": id, "error": map[string]any{"code": -32000, "message": errMsg}})
					return
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": id, "result": result})
			}

			switch req.Method {
			case "initialize":
				write(req.ID, map[string]any{
					"server": map[string]any{"name": "xconfig", "version": "0.1"},
					"capabilities": map[string]any{
						"tools": true,
					},
				}, "")
				return
			case "tools/list":
				write(req.ID, map[string]any{"tools": tools}, "")
				return
			case "tools/call":
				var p struct {
					Name      string          `json:"name"`
					Arguments json.RawMessage `json:"arguments"`
				}
				if err := json.Unmarshal(req.Params, &p); err != nil || p.Name == "" {
					write(req.ID, nil, "invalid params")
					return
				}

				// Closed loop: write a run record for each tool call (if DSN configured).
				var runID string
				if st != nil {
					ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
					defer cancel()
					rid, err := st.CreateRun(ctx, "xconfig", "", "mcp.tools/call", "running", "", "", []byte(fmt.Sprintf(`{"tool":%q}`, p.Name)))
					if err == nil {
						runID = rid
					}
				}

				switch p.Name {
				case "xconfig.ping":
					if st != nil && runID != "" {
						_ = st.FinishRun(r.Context(), runID, "ok", []byte(`{"ok":true}`))
					}
					write(req.ID, map[string]any{"ok": true}, "")
					return
				case "xconfig.playbook.run":
					if st != nil && runID != "" {
						_ = st.FinishRun(r.Context(), runID, "failed", []byte(`{"error":"not implemented"}`))
					}
					write(req.ID, nil, "not implemented")
					return
				default:
					if st != nil && runID != "" {
						_ = st.FinishRun(r.Context(), runID, "failed", []byte(`{"error":"unknown tool"}`))
					}
					write(req.ID, nil, "unknown tool")
					return
				}
			default:
				write(req.ID, nil, "method not found")
				return
			}
		})

		fmt.Println("xconfig mcp listening on", addr)
		return http.ListenAndServe(addr, mux)
	},
}

func init() {
	mcpServeCmd.Flags().String("addr", "", "listen addr (default :8081, or :$PORT)")
	mcpCmd.AddCommand(mcpServeCmd)
	addCommandOnce(rootCmd, mcpCmd)
}

