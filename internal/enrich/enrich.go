package enrich

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/joshp123/github-triage/internal/config"
	"github.com/joshp123/github-triage/internal/gh"
	"github.com/joshp123/github-triage/internal/storage"
)

type Options struct {
	Limit              int
	PRs                []int
	State              string
	FullFiles          bool
	WithComments       bool
	WithReviews        bool
	WithReviewComments bool
	SkipExisting       bool
	Concurrency        int
}

type prInfo struct {
	Number    int    `json:"number"`
	State     string `json:"state"`
	UpdatedAt string `json:"updatedAt"`
}

type prFiles struct {
	TotalCount int      `json:"total_count"`
	Truncated  bool     `json:"truncated"`
	Files      []string `json:"files"`
}

type ghFile struct {
	Filename string `json:"filename"`
}

func Run(ctx context.Context, cfg config.Config, opts Options) error {
	prs, err := listRawPRs(cfg, opts)
	if err != nil {
		return err
	}
	if len(prs) == 0 {
		return fmt.Errorf("no PRs found to enrich")
	}
	if opts.Concurrency <= 0 {
		opts.Concurrency = 4
	}

	jobs := make(chan int)
	var wg sync.WaitGroup
	var errCount int64

	worker := func() {
		defer wg.Done()
		for pr := range jobs {
			if opts.FullFiles {
				if err := ensureFullFiles(ctx, cfg, pr, opts.SkipExisting); err != nil {
					logf("files pr=%d err=%s", pr, err)
					atomic.AddInt64(&errCount, 1)
				}
			}
			if opts.WithComments {
				if err := ensureComments(ctx, cfg, pr, opts.SkipExisting); err != nil {
					logf("comments pr=%d err=%s", pr, err)
					atomic.AddInt64(&errCount, 1)
				}
			}
			if opts.WithReviews {
				if err := ensureReviews(ctx, cfg, pr, opts.SkipExisting); err != nil {
					logf("reviews pr=%d err=%s", pr, err)
					atomic.AddInt64(&errCount, 1)
				}
			}
			if opts.WithReviewComments {
				if err := ensureReviewComments(ctx, cfg, pr, opts.SkipExisting); err != nil {
					logf("review-comments pr=%d err=%s", pr, err)
					atomic.AddInt64(&errCount, 1)
				}
			}
		}
	}

	for i := 0; i < opts.Concurrency; i++ {
		wg.Add(1)
		go worker()
	}

	for _, pr := range prs {
		jobs <- pr
	}
	close(jobs)
	wg.Wait()

	if errCount > 0 {
		return fmt.Errorf("enrich completed with %d errors", errCount)
	}
	return nil
}

func ensureFullFiles(ctx context.Context, cfg config.Config, pr int, skip bool) error {
	path := cfg.RawPRFilesPath(pr)
	if skip {
		if data, err := os.ReadFile(path); err == nil {
			var files prFiles
			if json.Unmarshal(data, &files) == nil && !files.Truncated && len(files.Files) > 0 {
				return nil
			}
		}
	}

	out, err := gh.Run(ctx, "api", fmt.Sprintf("/repos/%s/pulls/%d/files", cfg.Repo, pr), "--paginate")
	if err != nil {
		return err
	}
	var items []ghFile
	if err := json.Unmarshal(out, &items); err != nil {
		return fmt.Errorf("parse files for %d: %w", pr, err)
	}
	files := make([]string, 0, len(items))
	for _, item := range items {
		if strings.TrimSpace(item.Filename) != "" {
			files = append(files, item.Filename)
		}
	}
	payload := prFiles{TotalCount: len(files), Truncated: false, Files: files}
	return storage.WriteJSONAtomic(path, payload)
}

func ensureComments(ctx context.Context, cfg config.Config, pr int, skip bool) error {
	path := cfg.RawPRCommentsPath(pr)
	if skip {
		if _, err := os.Stat(path); err == nil {
			return nil
		}
	}
	out, err := gh.Run(ctx, "api", fmt.Sprintf("/repos/%s/issues/%d/comments", cfg.Repo, pr), "--paginate")
	if err != nil {
		return err
	}
	return storage.WriteFileAtomic(path, out, 0o644)
}

func ensureReviews(ctx context.Context, cfg config.Config, pr int, skip bool) error {
	path := cfg.RawPRReviewsPath(pr)
	if skip {
		if _, err := os.Stat(path); err == nil {
			return nil
		}
	}
	out, err := gh.Run(ctx, "api", fmt.Sprintf("/repos/%s/pulls/%d/reviews", cfg.Repo, pr), "--paginate")
	if err != nil {
		return err
	}
	return storage.WriteFileAtomic(path, out, 0o644)
}

func ensureReviewComments(ctx context.Context, cfg config.Config, pr int, skip bool) error {
	path := cfg.RawPRReviewCommentsPath(pr)
	if skip {
		if _, err := os.Stat(path); err == nil {
			return nil
		}
	}
	out, err := gh.Run(ctx, "api", fmt.Sprintf("/repos/%s/pulls/%d/comments", cfg.Repo, pr), "--paginate")
	if err != nil {
		return err
	}
	return storage.WriteFileAtomic(path, out, 0o644)
}

func listRawPRs(cfg config.Config, opts Options) ([]int, error) {
	if len(opts.PRs) > 0 {
		filtered := make([]int, 0, len(opts.PRs))
		for _, pr := range opts.PRs {
			path := filepath.Join(cfg.RawDir, fmt.Sprintf("pr-%d.json", pr))
			if _, err := os.Stat(path); err != nil {
				return nil, fmt.Errorf("missing raw PR file for %d", pr)
			}
			filtered = append(filtered, pr)
		}
		sort.Ints(filtered)
		return applyLimit(filtered, opts.Limit), nil
	}

	entries, err := os.ReadDir(cfg.RawDir)
	if err != nil {
		return nil, err
	}
	re := regexp.MustCompile(`^pr-(\d+)\.json$`)
	prs := []int{}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		match := re.FindStringSubmatch(entry.Name())
		if len(match) != 2 {
			continue
		}
		num, err := strconv.Atoi(match[1])
		if err != nil {
			continue
		}
		info, err := loadPRInfo(cfg, num)
		if err != nil {
			return nil, err
		}
		if !stateMatches(opts.State, info.State) {
			continue
		}
		prs = append(prs, num)
	}
	return applyLimit(prs, opts.Limit), nil
}

func loadPRInfo(cfg config.Config, pr int) (prInfo, error) {
	path := filepath.Join(cfg.RawDir, fmt.Sprintf("pr-%d.json", pr))
	var info prInfo
	if err := storage.ReadJSON(path, &info); err != nil {
		return prInfo{}, fmt.Errorf("read PR %d: %w", pr, err)
	}
	if info.Number == 0 {
		info.Number = pr
	}
	return info, nil
}

func applyLimit(prs []int, limit int) []int {
	sort.Ints(prs)
	if limit > 0 && len(prs) > limit {
		prs = prs[:limit]
	}
	return prs
}

func stateMatches(filter string, value string) bool {
	filter = strings.TrimSpace(strings.ToLower(filter))
	if filter == "" || filter == "open" {
		filter = "open"
	}
	if filter == "all" {
		return true
	}
	value = strings.TrimSpace(strings.ToLower(value))
	switch filter {
	case "open":
		return value == "open"
	case "closed":
		return value == "closed" || value == "merged"
	default:
		return false
	}
}

func logf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
}
