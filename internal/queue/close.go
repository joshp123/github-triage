package queue

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Card struct {
	PR         int
	Author     string
	Maintainer bool
	Label      string
	Summary    string
	Evidence   []string
	Notes      []string
}

type CloseQueue struct {
	GeneratedAt time.Time
	Cards       []Card
	Total       int
	CloseReady  int
}

func BuildCloseQueue(mapDir string) (CloseQueue, error) {
	entries, err := os.ReadDir(mapDir)
	if err != nil {
		return CloseQueue{}, fmt.Errorf("read map dir: %w", err)
	}

	cards := []Card{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "pr-") || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		path := filepath.Join(mapDir, entry.Name())
		card, err := parseCard(path)
		if err != nil {
			return CloseQueue{}, err
		}
		if strings.ToLower(card.Label) != "slop" {
			continue
		}
		if !hasCloseReadyYes(card.Notes) {
			continue
		}
		cards = append(cards, card)
	}

	sort.Slice(cards, func(i, j int) bool {
		return cards[i].PR < cards[j].PR
	})

	return CloseQueue{
		GeneratedAt: time.Now().UTC(),
		Cards:       cards,
		Total:       len(entries),
		CloseReady:  len(cards),
	}, nil
}

func WriteCloseQueue(path string, queue CloseQueue) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(path), err)
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("# Close Queue — %s\n\n", queue.GeneratedAt.Format("2006-01-02 15:04:05 MST")))
	b.WriteString(fmt.Sprintf("- close-ready: %d\n\n", queue.CloseReady))

	for _, card := range queue.Cards {
		b.WriteString(fmt.Sprintf("- #%d — %s (author: %s)\n", card.PR, card.Summary, card.Author))
		for _, note := range card.Notes {
			b.WriteString(fmt.Sprintf("  - note: %s\n", note))
		}
		for _, ev := range card.Evidence {
			b.WriteString(fmt.Sprintf("  - evidence: %s\n", ev))
		}
		b.WriteString("\n")
	}

	return os.WriteFile(path, []byte(b.String()), 0o644)
}

func parseCard(path string) (Card, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Card{}, fmt.Errorf("read card %s: %w", path, err)
	}

	lines := strings.Split(string(data), "\n")
	card := Card{}
	section := ""

	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		switch {
		case strings.HasPrefix(line, "PR: #"):
			value := strings.TrimPrefix(line, "PR: #")
			if num, err := strconv.Atoi(strings.TrimSpace(value)); err == nil {
				card.PR = num
			}
		case strings.HasPrefix(line, "Author:"):
			card.Author = strings.TrimSpace(strings.TrimPrefix(line, "Author:"))
		case strings.HasPrefix(line, "Maintainer:"):
			card.Maintainer = strings.TrimSpace(strings.TrimPrefix(line, "Maintainer:")) == "yes"
		case strings.HasPrefix(line, "Label:"):
			card.Label = strings.TrimSpace(strings.TrimPrefix(line, "Label:"))
		case line == "## Summary":
			section = "summary"
		case line == "## Evidence":
			section = "evidence"
		case line == "## Notes":
			section = "notes"
		case strings.HasPrefix(line, "## "):
			section = ""
		default:
			switch section {
			case "summary":
				if strings.HasPrefix(line, "-") {
					card.Summary = strings.TrimSpace(strings.TrimPrefix(line, "-"))
				}
			case "evidence":
				if strings.HasPrefix(line, "-") {
					card.Evidence = append(card.Evidence, strings.TrimSpace(strings.TrimPrefix(line, "-")))
				}
			case "notes":
				if strings.HasPrefix(line, "-") {
					card.Notes = append(card.Notes, strings.TrimSpace(strings.TrimPrefix(line, "-")))
				}
			}
		}
	}

	if card.PR == 0 {
		return Card{}, fmt.Errorf("missing PR number in %s", path)
	}

	return card, nil
}

func hasCloseReadyYes(notes []string) bool {
	for _, note := range notes {
		value := strings.ToLower(strings.TrimSpace(note))
		if strings.HasPrefix(value, "close-ready: yes") {
			return true
		}
	}
	return false
}
