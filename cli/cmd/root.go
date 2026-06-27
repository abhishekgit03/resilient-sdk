package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/spf13/cobra"
)

type Config struct {
	DSN          string `toml:"dsn"`
	AppName      string `toml:"app_name"`
	GeminiAPIKey string `toml:"gemini_api_key"`
}

var DB *pgxpool.Pool

// Cfg is the loaded config, available to all subcommands.
var Cfg Config

var rootCmd = &cobra.Command{
	Use:   "resilient",
	Short: "Adaptive retry CLI — reports, anomalies, and AI-powered explanations",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if cmd.Name() == "init" {
			return nil
		}
		return connectDB()
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(reportCmd)
	rootCmd.AddCommand(anomaliesCmd)
	rootCmd.AddCommand(topCmd)
	rootCmd.AddCommand(explainCmd)
}

func connectDB() error {
	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}
	if cfg.DSN == "" {
		return fmt.Errorf("no DSN found — run: resilient init --dsn <connection-string>")
	}

	pool, err := pgxpool.New(context.Background(), cfg.DSN)
	if err != nil {
		return fmt.Errorf("db connect: %w", err)
	}

	DB = pool
	Cfg = cfg
	return nil
}

func loadConfig() (Config, error) {
	path := configPath()
	var cfg Config

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return cfg, nil
	}

	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return cfg, fmt.Errorf("parse %s: %w", path, err)
	}
	return cfg, nil
}

func configPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".resilient", "config.toml")
}
