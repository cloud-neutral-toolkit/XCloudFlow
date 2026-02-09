package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

type rootFlags struct {
	DSN string
}

var rf rootFlags

func Execute() error {
	rootCmd := &cobra.Command{
		Use:   "xcloudflow",
		Short: "XCloudFlow control plane (MCP/Agent/Skills/State)",
	}

	rootCmd.PersistentFlags().StringVar(&rf.DSN, "dsn", os.Getenv("DATABASE_URL"), "PostgreSQL DSN (defaults to DATABASE_URL)")

	rootCmd.AddCommand(dbCmd())
	rootCmd.AddCommand(mcpCmd())
	rootCmd.AddCommand(skillsCmd())
	rootCmd.AddCommand(agentCmd())

	return rootCmd.Execute()
}

func dsnOrErr() (string, error) {
	if rf.DSN == "" {
		return "", fmt.Errorf("missing --dsn (or set DATABASE_URL)")
	}
	return rf.DSN, nil
}

