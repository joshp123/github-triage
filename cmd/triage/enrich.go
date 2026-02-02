package main

import (
	"github.com/joshp123/github-triage/internal/config"
	"github.com/joshp123/github-triage/internal/enrich"
	"github.com/spf13/cobra"
)

func newEnrichCmd() *cobra.Command {
	var limit int
	var prNumbers []int
	var state string
	var concurrency int
	var fullFiles bool
	var comments bool
	var reviews bool
	var reviewComments bool
	var skipExisting bool

	cmd := &cobra.Command{
		Use:          "enrich",
		Short:        "Fetch full file lists, comments, and reviews",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(repoFlag)
			if err != nil {
				return err
			}
			if err := cfg.EnsureDirs(); err != nil {
				return err
			}

			opts := enrich.Options{
				Limit:              limit,
				PRs:                prNumbers,
				State:              state,
				FullFiles:          fullFiles,
				WithComments:       comments,
				WithReviews:        reviews,
				WithReviewComments: reviewComments,
				SkipExisting:       skipExisting,
				Concurrency:        concurrency,
			}
			return enrich.Run(cmd.Context(), cfg, opts)
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 0, "Max PRs to enrich (0 = all)")
	cmd.Flags().IntSliceVar(&prNumbers, "pr", nil, "Specific PR number to enrich (repeatable)")
	cmd.Flags().StringVar(&state, "state", "open", "PR state filter: open|closed|all")
	cmd.Flags().IntVar(&concurrency, "concurrency", 4, "Concurrency for GH API calls")
	cmd.Flags().BoolVar(&fullFiles, "full-files", true, "Fetch full file list (overwrites truncated file list)")
	cmd.Flags().BoolVar(&comments, "comments", true, "Fetch issue comments")
	cmd.Flags().BoolVar(&reviews, "reviews", true, "Fetch PR reviews")
	cmd.Flags().BoolVar(&reviewComments, "review-comments", false, "Fetch PR review comments")
	cmd.Flags().BoolVar(&skipExisting, "skip-existing", true, "Skip already fetched files")

	return cmd
}
