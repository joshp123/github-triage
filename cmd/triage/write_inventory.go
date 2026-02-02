package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/joshp123/github-triage/internal/storage"
	"github.com/spf13/cobra"
)

type inventoryItem struct {
	Label    string
	PR       int
	Summary  string
	Evidence string
}

func newWriteInventoryCmd() *cobra.Command {
	var rawItems []string
	cmd := &cobra.Command{
		Use:          "write-inventory",
		Short:        "Write inventory snapshot",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return writeInventory(rawItems)
		},
	}

	cmd.Flags().StringArrayVar(&rawItems, "item", nil, "Inventory item: label=slop|pr=123|summary=...|evidence=... (repeatable)")

	return cmd
}

func writeInventory(rawItems []string) error {
	items := make([]inventoryItem, 0, len(rawItems))
	for _, raw := range rawItems {
		item, err := parseInventoryItem(raw)
		if err != nil {
			return err
		}
		items = append(items, item)
	}

	body := renderInventory(items)
	root, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working dir: %w", err)
	}
	path := filepath.Join(root, "triage", "reduce", "current.md")
	return storage.WriteFileAtomic(path, []byte(body), 0o644)
}

func parseInventoryItem(raw string) (inventoryItem, error) {
	item := inventoryItem{}
	if strings.TrimSpace(raw) == "" {
		return item, errors.New("--item cannot be empty")
	}
	parts := strings.Split(raw, "|")
	for _, part := range parts {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			return item, fmt.Errorf("invalid --item segment %q", part)
		}
		key := strings.TrimSpace(kv[0])
		value := strings.TrimSpace(kv[1])
		switch key {
		case "label":
			item.Label = value
		case "pr":
			num, err := strconv.Atoi(value)
			if err != nil {
				return item, fmt.Errorf("invalid pr %q", value)
			}
			item.PR = num
		case "summary":
			item.Summary = value
		case "evidence":
			item.Evidence = value
		default:
			return item, fmt.Errorf("unknown --item key %q", key)
		}
	}

	if err := validateLabel(item.Label); err != nil {
		return item, err
	}
	if item.PR <= 0 {
		return item, errors.New("--item pr must be > 0")
	}
	if strings.TrimSpace(item.Summary) == "" {
		return item, errors.New("--item summary is required")
	}
	return item, nil
}

func renderInventory(items []inventoryItem) string {
	labels := []string{"good", "needs-human", "slop"}
	counts := map[string]int{"good": 0, "needs-human": 0, "slop": 0}
	grouped := map[string][]inventoryItem{"good": {}, "needs-human": {}, "slop": {}}

	for _, item := range items {
		counts[item.Label]++
		grouped[item.Label] = append(grouped[item.Label], item)
	}

	date := time.Now().UTC().Format("2006-01-02")
	var b strings.Builder
	b.WriteString(fmt.Sprintf("# Inventory Snapshot — %s\n\n", date))

	b.WriteString("## Counts\n")
	for _, label := range labels {
		b.WriteString(fmt.Sprintf("- %s: %d\n", displayLabel(label), counts[label]))
	}
	b.WriteString("\n")

	for _, label := range labels {
		b.WriteString(fmt.Sprintf("## %s\n", displayLabel(label)))
		items := grouped[label]
		if len(items) == 0 {
			b.WriteString("- (none)\n\n")
			continue
		}
		for _, item := range items {
			line := fmt.Sprintf("- #%d — %s", item.PR, item.Summary)
			if strings.TrimSpace(item.Evidence) != "" {
				line = fmt.Sprintf("%s (%s)", line, item.Evidence)
			}
			b.WriteString(line + "\n")
		}
		b.WriteString("\n")
	}

	return b.String()
}

func displayLabel(label string) string {
	if label == "slop" {
		return "low-signal"
	}
	return label
}
