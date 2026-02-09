package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	"xcloudflow/internal/mcp"
	"xcloudflow/internal/store"
)

// xcloud-server is the stateless control plane entrypoint intended for Cloud Run.
//
// Endpoints:
// - GET  /healthz
// - POST /mcp   (minimal JSON-RPC handler)
//
// State/memory is persisted in PostgreSQL (postgresql.svc.plus) when DATABASE_URL is provided.
func main() {
	var addr string
	flag.StringVar(&addr, "addr", "", "listen address (default :$PORT or :8080)")
	flag.Parse()

	if addr == "" {
		if p := os.Getenv("PORT"); p != "" {
			addr = ":" + p
		} else {
			addr = ":8080"
		}
	}

	dsn := os.Getenv("DATABASE_URL")
	var st *store.Store
	if dsn != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		s, err := store.Open(ctx, dsn)
		if err != nil {
			fmt.Fprintln(os.Stderr, "db connect:", err)
			os.Exit(1)
		}
		st = s
		defer st.Close()
	}

	srv := mcp.NewServer(mcp.ServerOptions{Store: st})

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.Handle("/mcp", srv)

	fmt.Println("listening on", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

