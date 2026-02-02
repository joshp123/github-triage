package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/joshp123/github-triage/internal/cluster"
	"github.com/joshp123/github-triage/internal/config"
	"github.com/spf13/cobra"
)

func newClusterLabelsCmd() *cobra.Command {
	var clustersDir string
	var output string
	var outputJSON string
	var maxTitles int

	cmd := &cobra.Command{
		Use:          "cluster-labels",
		Short:        "Generate human-readable labels for clusters",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(repoFlag)
			if err != nil {
				return err
			}
			if clustersDir == "" {
				clustersDir = filepath.Join(cfg.TriageDir, "cluster", "hdbscan-enriched")
			}
			if output == "" {
				output = filepath.Join(clustersDir, "labels.md")
			}
			if outputJSON == "" {
				outputJSON = filepath.Join(clustersDir, "labels.json")
			}
			labels, err := cluster.BuildLabels(clustersDir, cfg.RawDir, maxTitles)
			if err != nil {
				return err
			}
			if err := writeLabelsMarkdown(output, labels); err != nil {
				return err
			}
			if err := writeLabelsJSON(outputJSON, labels); err != nil {
				return err
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&clustersDir, "clusters", "", "Cluster directory (default: <data-root>/triage/cluster/hdbscan-enriched)")
	cmd.Flags().StringVar(&output, "output", "", "Markdown output path (default: <clusters>/labels.md)")
	cmd.Flags().StringVar(&outputJSON, "output-json", "", "JSON output path (default: <clusters>/labels.json)")
	cmd.Flags().IntVar(&maxTitles, "max-titles", 8, "Max titles per cluster")
	return cmd
}

func writeLabelsMarkdown(path string, labels []cluster.Label) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(path), err)
	}
	var b strings.Builder
	b.WriteString("# Cluster Labels\n\n")
	for _, label := range labels {
		b.WriteString(fmt.Sprintf("## cluster-%04d (size %d)\n", label.ID, label.Size))
		b.WriteString(fmt.Sprintf("- label: %s\n", label.Label))
		if len(label.TopTokens) > 0 {
			b.WriteString(fmt.Sprintf("- top tokens: %s\n", strings.Join(label.TopTokens, ", ")))
		}
		if len(label.SampleTitles) > 0 {
			b.WriteString("- sample titles:\n")
			for _, title := range label.SampleTitles {
				b.WriteString(fmt.Sprintf("  - %s\n", title))
			}
		}
		b.WriteString("\n")
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

func writeLabelsJSON(path string, labels []cluster.Label) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(path), err)
	}
	data, err := json.MarshalIndent(labels, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}
