package main

import (
	"os"

	"github.com/joshp123/github-triage/internal/config"
	"github.com/joshp123/github-triage/internal/ingest"
	"github.com/spf13/cobra"
)

var (
	repoFlag        string
	modelFlag       string
	concurrencyFlag int
)

func main() {
	root := &cobra.Command{
		Use:          "triage",
		Short:        "github-triage CLI",
		SilenceUsage: true,
	}

	root.PersistentFlags().StringVar(&repoFlag, "repo", "openclaw/openclaw", "GitHub repo (org/name)")
	root.PersistentFlags().StringVar(&modelFlag, "model", "openai-codex/gpt-5.2", "LLM model or provider/model (e.g. openai-codex/gpt-5.2)")
	root.PersistentFlags().IntVar(&concurrencyFlag, "concurrency", 8, "LLM concurrency (reserved)")

	root.AddCommand(newDiscoverCmd())
	root.AddCommand(newRunCmd())
	root.AddCommand(newMapCmd())
	root.AddCommand(newSweepCmd())
	root.AddCommand(newCloseQueueCmd())
	root.AddCommand(newReduceCmd())
	root.AddCommand(newEnrichCmd())
	root.AddCommand(newClusterExportCmd())
	root.AddCommand(newClusterLabelsCmd())
	root.AddCommand(newWriteCardCmd())
	root.AddCommand(newWriteInventoryCmd())

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func newDiscoverCmd() *cobra.Command {
	var limit int
	var state string
	cmd := &cobra.Command{
		Use:          "discover",
		Short:        "Build a classification rubric from a corpus sample",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(repoFlag)
			if err != nil {
				return err
			}
			return ingest.Discover(cmd.Context(), cfg, limit, state)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 0, "Max PRs to sample from (0 = all)")
	cmd.Flags().StringVar(&state, "state", "open", "PR state filter: open|closed|all")
	return cmd
}

func newRunCmd() *cobra.Command {
	var limit int
	var state string
	cmd := &cobra.Command{
		Use:          "run",
		Short:        "Ingest PRs and prep for map/inventory",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(repoFlag)
			if err != nil {
				return err
			}
			return ingest.Run(cmd.Context(), cfg, limit, state)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 0, "Max PRs to ingest (0 = all)")
	cmd.Flags().StringVar(&state, "state", "open", "PR state filter: open|closed|all")
	return cmd
}
