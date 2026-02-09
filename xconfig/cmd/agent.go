package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"xconfig/internal/xcfstore"
)

var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Agent mode (stateless worker, state/memory in PostgreSQL)",
}

var agentRunCmd = &cobra.Command{
	Use:   "run",
	Short: "Run a minimal agent heartbeat loop and write runs into xcf.runs",
	RunE: func(cmd *cobra.Command, args []string) error {
		interval, _ := cmd.Flags().GetDuration("interval")
		once, _ := cmd.Flags().GetBool("once")
		if once {
			interval = 0
		}
		if DSN == "" {
			return fmt.Errorf("missing --dsn (or set DATABASE_URL)")
		}

		ctx := context.Background()
		st, err := xcfstore.Open(ctx, DSN)
		if err != nil {
			return err
		}
		defer st.Close()

		doOnce := func() error {
			runID, err := st.CreateRun(ctx, "xconfig", "", "agent.heartbeat", "running", "", "", []byte(`{"component":"xconfig"}`))
			if err != nil {
				return err
			}
			out := map[string]any{
				"ok":        true,
				"timestamp": time.Now().UTC().Format(time.RFC3339),
				"hostname":  host(),
			}
			b, _ := json.Marshal(out)
			return st.FinishRun(ctx, runID, "ok", b)
		}

		if interval == 0 {
			return doOnce()
		}
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			if err := doOnce(); err != nil {
				fmt.Fprintln(os.Stderr, "agent run failed:", err)
			}
			<-t.C
		}
	},
}

func host() string {
	h, _ := os.Hostname()
	return h
}

func init() {
	agentRunCmd.Flags().Duration("interval", 5*time.Minute, "heartbeat interval")
	agentRunCmd.Flags().Bool("once", false, "run once and exit")
	agentCmd.AddCommand(agentRunCmd)
	addCommandOnce(rootCmd, agentCmd)
}

