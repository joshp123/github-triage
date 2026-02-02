package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	Repo        string
	Org         string
	Name        string
	DataRoot    string
	RepoDir     string
	TriageDir   string
	RawDir      string
	MapDir      string
	SweepDir    string
	ReduceDir   string
	RubricPath  string
	Maintainers string
	StatePath   string
	SamplePath  string
	CommentsDir string
}

func Load(repo string) (Config, error) {
	repo = strings.TrimSpace(repo)
	parts := strings.Split(repo, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return Config{}, fmt.Errorf("invalid repo %q; want org/name", repo)
	}

	xdg := strings.TrimSpace(os.Getenv("XDG_DATA_HOME"))
	if xdg == "" {
		return Config{}, errors.New("XDG_DATA_HOME must be set")
	}

	dataRoot := filepath.Join(xdg, "github-triage", parts[0], parts[1])
	repoDir := filepath.Join(dataRoot, "repo")
	triageDir := filepath.Join(dataRoot, "triage")
	rawDir := filepath.Join(triageDir, "raw")
	mapDir := filepath.Join(triageDir, "map")
	sweepDir := filepath.Join(triageDir, "sweep")
	reduceDir := filepath.Join(triageDir, "reduce")
	commentsDir := filepath.Join(triageDir, "comments")

	return Config{
		Repo:        repo,
		Org:         parts[0],
		Name:        parts[1],
		DataRoot:    dataRoot,
		RepoDir:     repoDir,
		TriageDir:   triageDir,
		RawDir:      rawDir,
		MapDir:      mapDir,
		SweepDir:    sweepDir,
		ReduceDir:   reduceDir,
		RubricPath:  filepath.Join(triageDir, "rubric.md"),
		Maintainers: filepath.Join(triageDir, "maintainers.txt"),
		StatePath:   filepath.Join(triageDir, "state.json"),
		SamplePath:  filepath.Join(rawDir, "pr-sample.json"),
		CommentsDir: commentsDir,
	}, nil
}

func (c Config) EnsureDirs() error {
	dirs := []string{c.RepoDir, c.TriageDir, c.RawDir, c.MapDir, c.SweepDir, c.ReduceDir, c.CommentsDir}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("mkdir %s: %w", dir, err)
		}
	}
	return nil
}

func (c Config) RawPRPath(number int) string {
	return filepath.Join(c.RawDir, fmt.Sprintf("pr-%d.json", number))
}

func (c Config) RawPRFilesPath(number int) string {
	return filepath.Join(c.RawDir, fmt.Sprintf("pr-%d.files.json", number))
}

func (c Config) RawPRMetaPath(number int) string {
	return filepath.Join(c.RawDir, fmt.Sprintf("pr-%d.meta.json", number))
}

func (c Config) RawPRCommentsPath(number int) string {
	return filepath.Join(c.CommentsDir, fmt.Sprintf("pr-%d.comments.json", number))
}

func (c Config) RawPRReviewsPath(number int) string {
	return filepath.Join(c.CommentsDir, fmt.Sprintf("pr-%d.reviews.json", number))
}

func (c Config) RawPRReviewCommentsPath(number int) string {
	return filepath.Join(c.CommentsDir, fmt.Sprintf("pr-%d.review-comments.json", number))
}
