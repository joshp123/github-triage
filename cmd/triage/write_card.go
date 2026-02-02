package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/joshp123/github-triage/internal/storage"
	"github.com/spf13/cobra"
)

type cardArgs struct {
	PR             int
	Author         string
	MaintainerMode string
	Label          string
	Summary        string
	Evidence       []string
	Notes          []string
}

func newWriteCardCmd() *cobra.Command {
	args := &cardArgs{}
	cmd := &cobra.Command{
		Use:          "write-card",
		Short:        "Write a PR classification card",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return writeCard(args)
		},
	}

	cmd.Flags().IntVar(&args.PR, "pr", 0, "PR number")
	cmd.Flags().StringVar(&args.Author, "author", "", "PR author login")
	cmd.Flags().StringVar(&args.MaintainerMode, "maintainer", "auto", "Maintainer mode: auto|yes|no")
	cmd.Flags().StringVar(&args.Label, "label", "", "Label: good|slop|needs-human")
	cmd.Flags().StringVar(&args.Summary, "summary", "", "One-line summary")
	cmd.Flags().StringArrayVar(&args.Evidence, "evidence", nil, "Evidence quote with source (repeatable)")
	cmd.Flags().StringArrayVar(&args.Notes, "note", nil, "Optional note (repeatable)")

	_ = cmd.MarkFlagRequired("pr")
	_ = cmd.MarkFlagRequired("author")

	return cmd
}

func writeCard(args *cardArgs) error {
	if args.PR <= 0 {
		return errors.New("--pr must be > 0")
	}
	author := strings.TrimSpace(args.Author)
	if author == "" {
		return errors.New("--author is required")
	}

	label := strings.TrimSpace(args.Label)
	summary := strings.TrimSpace(args.Summary)
	evidence := trimStrings(args.Evidence)
	notes := trimStrings(args.Notes)

	maintainer, err := resolveMaintainer(args.MaintainerMode, author)
	if err != nil {
		return err
	}

	if maintainer {
		label = "(none)"
		summary = "skipped (maintainer)"
		evidence = []string{"skipped (maintainer)"}
		notes = nil
	} else {
		if err := validateLabel(label); err != nil {
			return err
		}
		if summary == "" {
			return errors.New("--summary is required when maintainer=no")
		}
		if len(evidence) == 0 {
			return errors.New("--evidence is required when maintainer=no")
		}
	}

	body := renderCard(args.PR, author, maintainer, label, summary, evidence, notes)

	root, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working dir: %w", err)
	}
	cardDir := strings.TrimSpace(os.Getenv("XDG_TRIAGE_CARD_DIR"))
	if cardDir == "" {
		cardDir = filepath.Join("triage", "map")
	}
	var path string
	if filepath.IsAbs(cardDir) {
		path = filepath.Join(cardDir, fmt.Sprintf("pr-%d.md", args.PR))
	} else {
		path = filepath.Join(root, cardDir, fmt.Sprintf("pr-%d.md", args.PR))
	}
	return storage.WriteFileAtomic(path, []byte(body), 0o644)
}

func renderCard(pr int, author string, maintainer bool, label string, summary string, evidence []string, notes []string) string {
	var b strings.Builder
	b.WriteString("# PR Classification\n")
	b.WriteString(fmt.Sprintf("PR: #%d\n", pr))
	b.WriteString(fmt.Sprintf("Author: %s\n", author))
	if maintainer {
		b.WriteString("Maintainer: yes\n")
	} else {
		b.WriteString("Maintainer: no\n")
	}
	b.WriteString(fmt.Sprintf("Label: %s\n\n", label))

	b.WriteString("## Summary\n")
	b.WriteString(fmt.Sprintf("- %s\n\n", summary))

	b.WriteString("## Evidence\n")
	for _, item := range evidence {
		b.WriteString(fmt.Sprintf("- %s\n", item))
	}
	b.WriteString("\n")

	if len(notes) > 0 {
		b.WriteString("## Notes\n")
		for _, note := range notes {
			b.WriteString(fmt.Sprintf("- %s\n", note))
		}
		b.WriteString("\n")
	}

	return b.String()
}

func resolveMaintainer(mode string, author string) (bool, error) {
	mode = strings.TrimSpace(mode)
	switch mode {
	case "", "auto":
		return lookupMaintainer(author)
	case "yes":
		return true, nil
	case "no":
		return false, nil
	default:
		return false, fmt.Errorf("invalid --maintainer %q (want auto|yes|no)", mode)
	}
}

func lookupMaintainer(author string) (bool, error) {
	root, err := os.Getwd()
	if err != nil {
		return false, fmt.Errorf("get working dir: %w", err)
	}
	path := filepath.Join(root, "triage", "maintainers.txt")
	data, err := os.ReadFile(path)
	if err != nil {
		return false, nil
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == author {
			return true, nil
		}
	}
	return false, nil
}

func validateLabel(label string) error {
	switch label {
	case "good", "slop", "needs-human":
		return nil
	default:
		return fmt.Errorf("invalid --label %q (want good|slop|needs-human)", label)
	}
}

func trimStrings(items []string) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		value := strings.TrimSpace(item)
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}
