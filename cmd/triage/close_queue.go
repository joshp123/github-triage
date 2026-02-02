package main

import (
	"path/filepath"

	"github.com/joshp123/github-triage/internal/config"
	"github.com/joshp123/github-triage/internal/queue"
	"github.com/spf13/cobra"
)

func newCloseQueueCmd() *cobra.Command {
	var output string
	var cardDir string
	cmd := &cobra.Command{
		Use:          "close-queue",
		Short:        "Build close-ready queue from sweep cards",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(repoFlag)
			if err != nil {
				return err
			}
			if output == "" {
				output = filepath.Join(cfg.TriageDir, "close", "queue.md")
			}
			if cardDir == "" {
				cardDir = cfg.SweepDir
			} else if !filepath.IsAbs(cardDir) {
				cardDir = filepath.Join(cfg.DataRoot, cardDir)
			}
			queueData, err := queue.BuildCloseQueue(cardDir)
			if err != nil {
				return err
			}
			return queue.WriteCloseQueue(output, queueData)
		},
	}

	cmd.Flags().StringVar(&output, "output", "", "Output path (default: <data-root>/triage/close/queue.md)")
	cmd.Flags().StringVar(&cardDir, "cards", "", "Cards directory (default: <data-root>/triage/sweep)")
	return cmd
}
