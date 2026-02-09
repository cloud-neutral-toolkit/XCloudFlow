package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"xcloudflow/internal/stackflow"
	"xcloudflow/internal/store"
)

func agentCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agent",
		Short: "Agent mode (stateless worker, state/memory in PostgreSQL)",
	}
	cmd.AddCommand(agentRunCmd())
	return cmd
}

func agentRunCmd() *cobra.Command {
	var configPath string
	var env string
	var interval time.Duration
	var once bool
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run validate + dns-plan in a loop and persist runs to PostgreSQL",
		RunE: func(cmd *cobra.Command, args []string) error {
			dsn, err := dsnOrErr()
			if err != nil {
				return err
			}
			if configPath == "" {
				return fmt.Errorf("missing --config")
			}
			if once {
				interval = 0
			}

			ctx := context.Background()
			st, err := store.Open(ctx, dsn)
			if err != nil {
				return err
			}
			defer st.Close()

			doOnce := func() error {
				b, err := os.ReadFile(configPath)
				if err != nil {
					return err
				}
				cfg, err := stackflow.LoadYAML(b)
				if err != nil {
					return err
				}
				stackName, err := stackflow.StackName(cfg)
				if err != nil {
					return err
				}
				if env != "" {
					cfg = stackflow.ApplyEnvOverrides(cfg, env)
				}

				runID, err := st.CreateRun(ctx, store.Run{
					Stack:     stackName,
					Env:       env,
					Phase:     "validate+dns-plan",
					Status:    "running",
					ConfigRef: configPath,
				})
				if err != nil {
					return err
				}

				val, err := stackflow.Validate(cfg)
				if err != nil {
					_ = st.FinishRun(ctx, runID, "failed", []byte(fmt.Sprintf(`{"error":%q}`, err.Error())))
					return err
				}

				plan, err := stackflow.DNSPlan(cfg, env)
				if err != nil {
					_ = st.FinishRun(ctx, runID, "failed", []byte(fmt.Sprintf(`{"error":%q}`, err.Error())))
					return err
				}

				out := map[string]any{
					"validate": val,
					"dnsPlan":  plan,
				}
				rb, _ := json.Marshal(out)
				if err := st.FinishRun(ctx, runID, "ok", rb); err != nil {
					return err
				}
				return nil
			}

			if interval == 0 {
				return doOnce()
			}
			t := time.NewTicker(interval)
			defer t.Stop()
			for {
				if err := doOnce(); err != nil {
					fmt.Fprintln(os.Stderr, "run failed:", err)
				}
				<-t.C
			}
		},
	}
	cmd.Flags().StringVar(&configPath, "config", "", "Path to StackFlow YAML file")
	cmd.Flags().StringVar(&env, "env", "", "Optional env name (global.environments.<env>)")
	cmd.Flags().DurationVar(&interval, "interval", 10*time.Minute, "Run interval (0 to run once)")
	cmd.Flags().BoolVar(&once, "once", false, "Run once and exit")
	return cmd
}

