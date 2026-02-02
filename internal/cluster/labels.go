package cluster

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/joshp123/github-triage/internal/storage"
)

type Label struct {
	ID           int      `json:"id"`
	Size         int      `json:"size"`
	Label        string   `json:"label"`
	TopTokens    []string `json:"top_tokens"`
	SampleTitles []string `json:"sample_titles"`
}

type rawTitle struct {
	Title string `json:"title"`
}

func BuildLabels(clusterDir string, rawDir string, maxTitles int) ([]Label, error) {
	clustersPath := filepath.Join(clusterDir, "clusters")
	entries, err := os.ReadDir(clustersPath)
	if err != nil {
		return nil, fmt.Errorf("read clusters dir: %w", err)
	}

	labels := []Label{}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasPrefix(name, "cluster-") || !strings.HasSuffix(name, ".txt") {
			continue
		}
		id, err := parseClusterID(name)
		if err != nil {
			return nil, err
		}
		prs, err := readPRList(filepath.Join(clustersPath, name))
		if err != nil {
			return nil, err
		}
		titles := loadTitles(rawDir, prs, maxTitles)
		tokens := tokenCounts(titles)
		topTokens := topN(tokens, 3)
		label := "misc"
		if len(topTokens) > 0 {
			label = strings.Join(topTokens, "/")
		}
		labels = append(labels, Label{
			ID:           id,
			Size:         len(prs),
			Label:        label,
			TopTokens:    topTokens,
			SampleTitles: titles,
		})
	}

	sort.Slice(labels, func(i, j int) bool {
		if labels[i].Size == labels[j].Size {
			return labels[i].ID < labels[j].ID
		}
		return labels[i].Size > labels[j].Size
	})

	return labels, nil
}

func parseClusterID(name string) (int, error) {
	trimmed := strings.TrimSuffix(strings.TrimPrefix(name, "cluster-"), ".txt")
	id, err := strconv.Atoi(trimmed)
	if err != nil {
		return 0, fmt.Errorf("invalid cluster id %q", name)
	}
	return id, nil
}

func readPRList(path string) ([]int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	lines := strings.Split(string(data), "\n")
	prs := []int{}
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		num, err := strconv.Atoi(line)
		if err != nil {
			continue
		}
		prs = append(prs, num)
	}
	return prs, nil
}

func loadTitles(rawDir string, prs []int, max int) []string {
	titles := []string{}
	for _, pr := range prs {
		if max > 0 && len(titles) >= max {
			break
		}
		path := filepath.Join(rawDir, fmt.Sprintf("pr-%d.json", pr))
		var raw rawTitle
		if err := storage.ReadJSON(path, &raw); err != nil {
			continue
		}
		title := strings.TrimSpace(raw.Title)
		if title != "" {
			titles = append(titles, fmt.Sprintf("#%d %s", pr, title))
		}
	}
	return titles
}

var tokenRe = regexp.MustCompile(`[a-z0-9][a-z0-9_-]+`)

var stopwords = map[string]bool{
	"the":      true,
	"and":      true,
	"for":      true,
	"with":     true,
	"from":     true,
	"this":     true,
	"that":     true,
	"into":     true,
	"new":      true,
	"add":      true,
	"adds":     true,
	"added":    true,
	"update":   true,
	"updates":  true,
	"fix":      true,
	"fixes":    true,
	"feat":     true,
	"feature":  true,
	"chore":    true,
	"refactor": true,
	"docs":     true,
	"doc":      true,
	"readme":   true,
	"tests":    true,
}

func tokenCounts(titles []string) map[string]int {
	counts := map[string]int{}
	for _, title := range titles {
		text := strings.ToLower(title)
		for _, token := range tokenRe.FindAllString(text, -1) {
			if len(token) < 3 {
				continue
			}
			if stopwords[token] {
				continue
			}
			counts[token] += 1
		}
	}
	return counts
}

func topN(counts map[string]int, n int) []string {
	type pair struct {
		Token string
		Count int
	}
	pairs := make([]pair, 0, len(counts))
	for token, count := range counts {
		pairs = append(pairs, pair{Token: token, Count: count})
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].Count == pairs[j].Count {
			return pairs[i].Token < pairs[j].Token
		}
		return pairs[i].Count > pairs[j].Count
	})
	limit := n
	if len(pairs) < limit {
		limit = len(pairs)
	}
	out := []string{}
	for i := 0; i < limit; i++ {
		out = append(out, pairs[i].Token)
	}
	return out
}
