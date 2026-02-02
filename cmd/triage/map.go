package main

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/joshp123/github-triage/internal/config"
	"github.com/joshp123/github-triage/internal/llm"
	"github.com/spf13/cobra"
)

func newMapCmd() *cobra.Command {
	var limit int
	var prNumbers []int
	var state string
	var order string
	var timeout time.Duration
	var skipExisting bool
	cmd := &cobra.Command{
		Use:          "map",
		Short:        "Run LLM classification over ingested PRs",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(repoFlag)
			if err != nil {
				return err
			}
			if err := cfg.EnsureDirs(); err != nil {
				return err
			}
			ensureSelfInPath()

			runner, err := llm.NewRunner(cfg, modelFlag)
			if err != nil {
				return err
			}
			return runner.Map(cmd.Context(), cfg, limit, prNumbers, concurrencyFlag, state, order, timeout, skipExisting)
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 0, "Max PRs to map from triage/raw (0 = all)")
	cmd.Flags().IntSliceVar(&prNumbers, "pr", nil, "Specific PR number to map (repeatable)")
	cmd.Flags().StringVar(&state, "state", "open", "PR state filter: open|closed|all")
	cmd.Flags().StringVar(&order, "order", "updated-desc", "Order: updated-asc|updated-desc|number-asc|number-desc")
	cmd.Flags().DurationVar(&timeout, "timeout", 5*time.Minute, "Per-PR timeout (e.g. 2m, 30s)")
	cmd.Flags().BoolVar(&skipExisting, "skip-existing", true, "Skip PRs with existing map cards")
	return cmd
}

func ensureSelfInPath() {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	_ = os.Setenv("XDG_TRIAGE_CLI", exe)

	dir := filepath.Dir(exe)
	path := os.Getenv("PATH")
	if path == "" {
		_ = os.Setenv("PATH", dir)
		return
	}
	for _, part := range strings.Split(path, string(os.PathListSeparator)) {
		if part == dir {
			return
		}
	}
	_ = os.Setenv("PATH", dir+string(os.PathListSeparator)+path)
}
