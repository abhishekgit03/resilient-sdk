package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var dsn string

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Configure the resilient CLI with your Postgres DSN",
	Example: `  resilient init --dsn postgresql://postgres:postgres@localhost/resilient`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if dsn == "" {
			return fmt.Errorf("--dsn is required")
		}

		path := configPath()

		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return fmt.Errorf("create config dir: %w", err)
		}

		content := fmt.Sprintf("dsn = %q\napp_name = \"app\"\n", dsn)
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			return fmt.Errorf("write config: %w", err)
		}

		fmt.Printf("Config written to %s\n", path)
		return nil
	},
}

func init() {
	initCmd.Flags().StringVar(&dsn, "dsn", "", "Postgres connection string (required)")
}
