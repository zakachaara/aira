// Package main is the AIRA CLI entrypoint.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/aira/aira/internal/config"
	"github.com/aira/aira/internal/delivery"
	"github.com/aira/aira/internal/logger"
	"github.com/aira/aira/internal/storage"
)

// appCtx holds shared dependencies injected into cobra sub-commands via context.
type appCtx struct {
	cfg  *config.Config
	repo storage.Repository
}

type ctxKey struct{}

// ─────────────────────────────────────────────
//  Global flags
// ─────────────────────────────────────────────

var (
	cfgFile    string
	logLevel   string
	logPretty  bool
	showBanner bool
)

// ─────────────────────────────────────────────
//  Root command
// ─────────────────────────────────────────────

var rootCmd = &cobra.Command{
	Use:   "aira",
	Short: "AIRA – AI Research Aggregator CLI",
	Long: `AIRA is a modular intelligence platform for monitoring AI research,
model releases, and cloud-native ecosystem updates via RSS feeds.

Pipeline:
  collect → parse → classify → signals → trends → digest

Sources: arXiv • Papers with Code • CNCF • OpenAI • HuggingFace • and more`,
	SilenceUsage: true,
	PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
		if cmd.Name() == "completion" || cmd.Name() == "help" {
			return nil
		}
		return bootstrap(cmd)
	},
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "",
		"config file (default: ~/.aira/config.yaml)")
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "info",
		"log level: debug|info|warn|error")
	rootCmd.PersistentFlags().BoolVar(&logPretty, "log-pretty", true,
		"human-readable console log output")
	rootCmd.PersistentFlags().BoolVar(&showBanner, "banner", false,
		"show AIRA ASCII banner on startup")

	rootCmd.AddCommand(
		collectCmd(),
		parseCmd(),
		classifyCmd(),
		signalsCmd(),
		trendsCmd(),
		digestCmd(),
		sourcesCmd(),
		scheduleCmd(),
		initCmd(),
		versionCmd(),
	)
}

// ─────────────────────────────────────────────
//  Bootstrap
// ─────────────────────────────────────────────

// bootstrap initialises the logger, loads config, opens the DB, and injects
// an appCtx into the cobra command's context.
func bootstrap(cmd *cobra.Command) error {
	logger.Init(logLevel, logPretty)

	if showBanner {
		delivery.PrintBanner()
	}

	if cfgFile != "" {
		_ = os.Setenv("AIRA_CONFIG", cfgFile)
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	repo, err := openRepo(cfg)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}

	if err := repo.Migrate(context.Background()); err != nil {
		return fmt.Errorf("running migrations: %w", err)
	}

	cmd.SetContext(context.WithValue(cmd.Context(), ctxKey{}, &appCtx{cfg: cfg, repo: repo}))
	return nil
}

// openRepo constructs a Repository from the driver config.
func openRepo(cfg *config.Config) (storage.Repository, error) {
	switch cfg.Database.Driver {
	case "sqlite3", "sqlite", "":
		return storage.NewSQLite(cfg.Database.DSN)
	default:
		return nil, fmt.Errorf("unsupported db driver %q (supported: sqlite3)", cfg.Database.Driver)
	}
}

// getApp extracts the appCtx from the command context or exits on failure.
func getApp(cmd *cobra.Command) *appCtx {
	v := cmd.Context().Value(ctxKey{})
	if v == nil {
		fmt.Fprintln(os.Stderr, "fatal: app context not initialised (bootstrap failed)")
		os.Exit(1)
	}
	return v.(*appCtx)
}

// ─────────────────────────────────────────────
//  Entrypoint
// ─────────────────────────────────────────────

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
