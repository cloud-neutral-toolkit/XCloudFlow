package cli

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"xcloudflow/internal/store"
)

func dbCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "db",
		Short: "Database utilities (schema init/migrate)",
	}
	cmd.AddCommand(dbInitCmd())
	return cmd
}

func dbInitCmd() *cobra.Command {
	var schemaPath string
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize schema (sql/schema.sql) in PostgreSQL",
		RunE: func(cmd *cobra.Command, args []string) error {
			dsn, err := dsnOrErr()
			if err != nil {
				return err
			}
			if schemaPath == "" {
				schemaPath = "sql/schema.sql"
			}
			b, err := os.ReadFile(schemaPath)
			if err != nil {
				return fmt.Errorf("read schema: %w", err)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			st, err := store.Open(ctx, dsn)
			if err != nil {
				return err
			}
			defer st.Close()

			if err := st.ExecSQL(ctx, string(b)); err != nil {
				return fmt.Errorf("apply schema: %w", err)
			}
			fmt.Println("ok: schema applied")
			return nil
		},
	}
	cmd.Flags().StringVar(&schemaPath, "schema", "sql/schema.sql", "Path to schema SQL file")
	return cmd
}

