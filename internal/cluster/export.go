package cluster

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/joshp123/github-triage/internal/config"
	"github.com/joshp123/github-triage/internal/storage"
)

type PRSnapshot struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	Body   string `json:"body"`
	URL    string `json:"url"`
	State  string `json:"state"`
}

type PRFiles struct {
	TotalCount int      `json:"total_count"`
	Truncated  bool     `json:"truncated"`
	Files      []string `json:"files"`
}

type Item struct {
	URL    string   `json:"url"`
	Number int      `json:"number"`
	Title  string   `json:"title"`
	Body   string   `json:"body"`
	State  string   `json:"state"`
	Type   string   `json:"type"`
	Files  []string `json:"files,omitempty"`
}

type comment struct {
	Body string `json:"body"`
}

type review struct {
	Body  string `json:"body"`
	State string `json:"state"`
}

func Export(cfg config.Config, outputPath string, stateFilter string) error {
	entries, err := os.ReadDir(cfg.RawDir)
	if err != nil {
		return fmt.Errorf("read raw dir: %w", err)
	}

	items := make([]Item, 0, len(entries))

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if name == "pr-sample.json" {
			continue
		}
		if !strings.HasPrefix(name, "pr-") || !strings.HasSuffix(name, ".json") {
			continue
		}
		if strings.HasSuffix(name, ".files.json") || strings.HasSuffix(name, ".meta.json") {
			continue
		}

		prNumber, ok := parsePRNumber(name)
		if !ok {
			continue
		}

		var pr PRSnapshot
		if err := storage.ReadJSON(filepath.Join(cfg.RawDir, name), &pr); err != nil {
			return fmt.Errorf("read pr json %s: %w", name, err)
		}

		if pr.Number == 0 {
			pr.Number = prNumber
		}

		state := normalizeState(pr.State)
		if !stateMatches(stateFilter, state) {
			continue
		}

		filesPath := cfg.RawPRFilesPath(prNumber)
		var files PRFiles
		if err := storage.ReadJSON(filesPath, &files); err != nil {
			return fmt.Errorf("read pr files %s: %w", filesPath, err)
		}

		body := strings.TrimSpace(pr.Body)
		commentsText, _ := loadCommentBodies(cfg.RawPRCommentsPath(prNumber))
		reviewsText, _ := loadReviewBodies(cfg.RawPRReviewsPath(prNumber))
		reviewCommentsText, _ := loadCommentBodies(cfg.RawPRReviewCommentsPath(prNumber))
		body = appendText(body, "Comments", commentsText)
		body = appendText(body, "Reviews", reviewsText)
		body = appendText(body, "Review comments", reviewCommentsText)

		items = append(items, Item{
			URL:    pr.URL,
			Number: prNumber,
			Title:  strings.TrimSpace(pr.Title),
			Body:   body,
			State:  state,
			Type:   "pr",
			Files:  files.Files,
		})
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].Number < items[j].Number
	})

	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(outputPath), err)
	}

	return storage.WriteJSONAtomic(outputPath, items)
}

func parsePRNumber(name string) (int, bool) {
	trimmed := strings.TrimPrefix(name, "pr-")
	trimmed = strings.TrimSuffix(trimmed, ".json")
	if trimmed == "" {
		return 0, false
	}
	num, err := strconv.Atoi(trimmed)
	if err != nil {
		return 0, false
	}
	return num, true
}

func normalizeState(state string) string {
	return strings.ToLower(strings.TrimSpace(state))
}

func stateMatches(filter string, state string) bool {
	switch strings.ToLower(strings.TrimSpace(filter)) {
	case "", "open":
		return state == "open"
	case "closed":
		return state == "closed" || state == "merged"
	case "all":
		return true
	default:
		return false
	}
}

func loadCommentBodies(path string) ([]string, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, err
	}
	var items []comment
	if err := storage.ReadJSON(path, &items); err != nil {
		return nil, err
	}
	bodies := []string{}
	for _, item := range items {
		text := strings.TrimSpace(item.Body)
		if text != "" {
			bodies = append(bodies, text)
		}
	}
	return bodies, nil
}

func loadReviewBodies(path string) ([]string, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, err
	}
	var items []review
	if err := storage.ReadJSON(path, &items); err != nil {
		return nil, err
	}
	bodies := []string{}
	for _, item := range items {
		text := strings.TrimSpace(item.Body)
		if text != "" {
			if item.State != "" {
				bodies = append(bodies, fmt.Sprintf("%s: %s", strings.TrimSpace(item.State), text))
			} else {
				bodies = append(bodies, text)
			}
		}
	}
	return bodies, nil
}

func appendText(body string, label string, parts []string) string {
	if len(parts) == 0 {
		return body
	}
	var b strings.Builder
	if strings.TrimSpace(body) != "" {
		b.WriteString(strings.TrimSpace(body))
		b.WriteString("\n\n")
	}
	b.WriteString(label)
	b.WriteString(":\n")
	for _, part := range parts {
		b.WriteString("- ")
		b.WriteString(part)
		b.WriteString("\n")
	}
	return strings.TrimSpace(b.String())
}
