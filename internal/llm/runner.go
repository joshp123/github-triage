package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/joshp123/github-triage/internal/config"
	pi "github.com/joshp123/pi-golang"
)

const (
	promptMap    = "map.md"
	promptSweep  = "sweep.md"
	promptReduce = "reduce.md"
	promptDisc   = "discover.md"
)

type Runner struct {
	PromptDir string
	Provider  string
	Model     string
	WorkDir   string
	AgentDir  string
}

func ResolvePromptDir() (string, error) {
	start, err := os.Getwd()
	if err != nil {
		return "", err
	}

	dir := start
	for {
		candidate := filepath.Join(dir, "prompts", promptMap)
		if _, err := os.Stat(candidate); err == nil {
			return filepath.Join(dir, "prompts"), nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", errors.New("prompts directory not found; run from repo root")
}

func NewRunner(cfg config.Config, model string) (Runner, error) {
	promptDir, err := ResolvePromptDir()
	if err != nil {
		return Runner{}, err
	}
	provider, resolvedModel := resolveProviderModel(model)
	agentDir, err := ensureAgentDir(cfg)
	if err != nil {
		return Runner{}, err
	}
	return Runner{
		PromptDir: promptDir,
		Provider:  provider,
		Model:     resolvedModel,
		WorkDir:   cfg.DataRoot,
		AgentDir:  agentDir,
	}, nil
}

func resolveProviderModel(model string) (string, string) {
	provider := "openai-codex"
	resolved := "gpt-5.2"
	value := strings.TrimSpace(model)
	if value == "" {
		return provider, resolved
	}
	if strings.Contains(value, "/") {
		parts := strings.SplitN(value, "/", 2)
		p := strings.TrimSpace(parts[0])
		m := strings.TrimSpace(parts[1])
		if p != "" && m != "" {
			return p, m
		}
	}
	return provider, value
}

func (r Runner) Map(ctx context.Context, cfg config.Config, limit int, prNumbers []int, concurrency int, state string, order string, timeout time.Duration, skipExisting bool) error {
	cardDir := filepath.Join("triage", "map")
	return r.runMap(ctx, cfg, filepath.Join(r.PromptDir, promptMap), limit, prNumbers, concurrency, state, order, "high", timeout, skipExisting, true, cardDir)
}

func (r Runner) Sweep(ctx context.Context, cfg config.Config, limit int, prNumbers []int, concurrency int, state string, order string, timeout time.Duration, skipExisting bool) error {
	cardDir := filepath.Join("triage", "sweep")
	return r.runMap(ctx, cfg, filepath.Join(r.PromptDir, promptSweep), limit, prNumbers, concurrency, state, order, "low", timeout, skipExisting, false, cardDir)
}

func (r Runner) runMap(ctx context.Context, cfg config.Config, promptPath string, limit int, prNumbers []int, concurrency int, state string, order string, thinking string, timeout time.Duration, skipExisting bool, abortOnError bool, cardDir string) error {
	prs, err := listRawPRs(cfg, limit, prNumbers, state, order)
	if err != nil {
		return err
	}
	if len(prs) == 0 {
		return errors.New("no PRs found in triage/raw")
	}
	if concurrency <= 0 {
		concurrency = 1
	}

	restoreEnv := setCardDirEnv(cardDir)
	defer restoreEnv()

	cardDirAbs := filepath.Join(cfg.DataRoot, cardDir)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	jobs := make(chan int)
	errCh := make(chan error, 1)

	var errCount int64
	var successCount int64
	var skipCount int64

	worker := func() {
		for pr := range jobs {
			cardPath := filepath.Join(cardDirAbs, fmt.Sprintf("pr-%d.md", pr))
			if skipExisting {
				if _, err := os.Stat(cardPath); err == nil {
					atomic.AddInt64(&skipCount, 1)
					continue
				}
			}

			logf("start pr=%d", pr)
			var lastErr error
			for attempt := 1; attempt <= 2; attempt++ {
				if err := r.runPrompt(ctx, promptPath, strconv.Itoa(pr), thinking, timeout); err != nil {
					lastErr = err
					logf("error pr=%d attempt=%d err=%s", pr, attempt, err)
					continue
				}
				if err := validateCard(cardPath, pr); err != nil {
					lastErr = err
					logf("invalid pr=%d attempt=%d err=%s", pr, attempt, err)
					continue
				}
				lastErr = nil
				break
			}
			if lastErr != nil {
				logf("failed pr=%d err=%s", pr, lastErr)
				atomic.AddInt64(&errCount, 1)
				if abortOnError {
					select {
					case errCh <- lastErr:
					default:
					}
					cancel()
					return
				}
				continue
			}
			atomic.AddInt64(&successCount, 1)
			logf("done pr=%d", pr)
		}
	}

	for i := 0; i < concurrency; i++ {
		go worker()
	}

	for _, pr := range prs {
		select {
		case <-ctx.Done():
			break
		case jobs <- pr:
		}
	}
	close(jobs)

	select {
	case err := <-errCh:
		return err
	default:
		closeReady := countCloseReady(cardDirAbs, prs)
		logf("summary total=%d success=%d failed=%d skipped=%d close_ready=%d", len(prs), atomic.LoadInt64(&successCount), atomic.LoadInt64(&errCount), atomic.LoadInt64(&skipCount), closeReady)
		if !abortOnError {
			if atomic.LoadInt64(&successCount) == 0 && atomic.LoadInt64(&errCount) > 0 {
				return fmt.Errorf("sweep failed for all PRs (%d errors)", errCount)
			}
			if atomic.LoadInt64(&errCount) > 0 {
				logf("completed with %d errors", errCount)
			}
		}
		return nil
	}
}

func (r Runner) Reduce(ctx context.Context) error {
	promptPath := filepath.Join(r.PromptDir, promptReduce)
	inventoryPath := filepath.Join(r.WorkDir, "triage", "reduce", "current.md")

	var lastErr error
	for attempt := 1; attempt <= 2; attempt++ {
		if err := r.runPrompt(ctx, promptPath, "REDUCE", "high", 5*time.Minute); err != nil {
			lastErr = err
			continue
		}
		if _, err := os.Stat(inventoryPath); err != nil {
			lastErr = fmt.Errorf("inventory output missing (expected %s)", inventoryPath)
			continue
		}
		lastErr = nil
		break
	}
	return lastErr
}

func (r Runner) Discover(ctx context.Context) error {
	promptPath := filepath.Join(r.PromptDir, promptDisc)
	return r.runPrompt(ctx, promptPath, "DISCOVER", "high", 5*time.Minute)
}

func (r Runner) runPrompt(ctx context.Context, promptPath string, input string, thinking string, timeout time.Duration) error {
	promptBytes, err := os.ReadFile(promptPath)
	if err != nil {
		return fmt.Errorf("read prompt %s: %w", promptPath, err)
	}

	opts := pi.DefaultOneShotOptions()
	opts.AppName = "github-triage"
	opts.WorkDir = r.WorkDir
	opts.SystemPrompt = string(promptBytes)
	opts.Mode = pi.ModeDragons
	opts.Dragons = pi.DragonsOptions{
		Provider: r.Provider,
		Model:    r.Model,
		Thinking: normalizeThinking(thinking),
	}

	client, err := pi.StartOneShot(opts)
	if err != nil {
		return err
	}
	defer client.Close()

	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	_, err = client.Run(runCtx, input)
	if err != nil {
		stderr := strings.TrimSpace(client.Stderr())
		if stderr != "" {
			return fmt.Errorf("pi run failed: %w (stderr: %s)", err, stderr)
		}
		return err
	}
	return nil
}

func validateCard(path string, pr int) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("map output missing for PR %d (expected %s)", pr, path)
	}
	text := strings.TrimSpace(string(data))
	lines := strings.Split(text, "\n")
	if len(lines) < 5 {
		return fmt.Errorf("map output invalid for PR %d (too short)", pr)
	}
	if strings.TrimSpace(lines[0]) != "# PR Classification" {
		return fmt.Errorf("map output invalid for PR %d (expected '# PR Classification')", pr)
	}
	if !strings.HasPrefix(strings.TrimSpace(lines[1]), "PR: #") {
		return fmt.Errorf("map output invalid for PR %d (missing PR line)", pr)
	}
	if !strings.HasPrefix(strings.TrimSpace(lines[2]), "Author:") {
		return fmt.Errorf("map output invalid for PR %d (missing Author line)", pr)
	}
	if !strings.HasPrefix(strings.TrimSpace(lines[3]), "Maintainer:") {
		return fmt.Errorf("map output invalid for PR %d (missing Maintainer line)", pr)
	}
	if !strings.HasPrefix(strings.TrimSpace(lines[4]), "Label:") {
		return fmt.Errorf("map output invalid for PR %d (missing Label line)", pr)
	}
	return nil
}

type prInfo struct {
	Number    int    `json:"number"`
	State     string `json:"state"`
	UpdatedAt string `json:"updatedAt"`
}

func listRawPRs(cfg config.Config, limit int, prNumbers []int, state string, order string) ([]int, error) {
	normalizedOrder, err := normalizeOrder(order)
	if err != nil {
		return nil, err
	}

	infos := []prInfo{}
	if len(prNumbers) > 0 {
		for _, pr := range prNumbers {
			info, err := loadPRInfo(cfg, pr)
			if err != nil {
				return nil, err
			}
			if info.Number == 0 {
				info.Number = pr
			}
			infos = append(infos, info)
		}
		orderPRs(infos, normalizedOrder)
		return applyLimit(infos, limit), nil
	}

	entries, err := os.ReadDir(cfg.RawDir)
	if err != nil {
		return nil, err
	}

	re := regexp.MustCompile(`^pr-(\d+)\.json$`)
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
		if info.Number == 0 {
			info.Number = num
		}
		if !stateMatches(state, info.State) {
			continue
		}
		infos = append(infos, info)
	}

	orderPRs(infos, normalizedOrder)
	return applyLimit(infos, limit), nil
}

func loadPRInfo(cfg config.Config, pr int) (prInfo, error) {
	path := filepath.Join(cfg.RawDir, fmt.Sprintf("pr-%d.json", pr))
	var info prInfo
	if err := readJSON(path, &info); err != nil {
		return prInfo{}, fmt.Errorf("read PR %d: %w", pr, err)
	}
	return info, nil
}

func normalizeOrder(order string) (string, error) {
	switch strings.TrimSpace(strings.ToLower(order)) {
	case "", "updated-desc", "newest":
		return "updated-desc", nil
	case "updated-asc", "oldest":
		return "updated-asc", nil
	case "number-asc":
		return "number-asc", nil
	case "number-desc":
		return "number-desc", nil
	default:
		return "", fmt.Errorf("invalid order %q (want updated-asc|updated-desc|number-asc|number-desc)", order)
	}
}

func orderPRs(infos []prInfo, order string) {
	switch order {
	case "updated-asc":
		sort.Slice(infos, func(i, j int) bool {
			if infos[i].UpdatedAt != infos[j].UpdatedAt {
				return infos[i].UpdatedAt < infos[j].UpdatedAt
			}
			return infos[i].Number < infos[j].Number
		})
	case "updated-desc":
		sort.Slice(infos, func(i, j int) bool {
			if infos[i].UpdatedAt != infos[j].UpdatedAt {
				return infos[i].UpdatedAt > infos[j].UpdatedAt
			}
			return infos[i].Number > infos[j].Number
		})
	case "number-desc":
		sort.Slice(infos, func(i, j int) bool {
			return infos[i].Number > infos[j].Number
		})
	default:
		sort.Slice(infos, func(i, j int) bool {
			return infos[i].Number < infos[j].Number
		})
	}
}

func applyLimit(infos []prInfo, limit int) []int {
	if limit > 0 && len(infos) > limit {
		infos = infos[:limit]
	}
	prs := make([]int, 0, len(infos))
	for _, info := range infos {
		prs = append(prs, info.Number)
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

func readJSON(path string, out any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, out); err != nil {
		return err
	}
	return nil
}

func normalizeThinking(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case "low", "medium", "high":
		return strings.TrimSpace(strings.ToLower(value))
	default:
		return "high"
	}
}

func logf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
}

func setCardDirEnv(dir string) func() {
	prev, ok := os.LookupEnv("XDG_TRIAGE_CARD_DIR")
	if strings.TrimSpace(dir) == "" {
		_ = os.Unsetenv("XDG_TRIAGE_CARD_DIR")
	} else {
		_ = os.Setenv("XDG_TRIAGE_CARD_DIR", dir)
	}
	return func() {
		if ok {
			_ = os.Setenv("XDG_TRIAGE_CARD_DIR", prev)
			return
		}
		_ = os.Unsetenv("XDG_TRIAGE_CARD_DIR")
	}
}

func countCloseReady(cardDir string, prs []int) int {
	count := 0
	for _, pr := range prs {
		path := filepath.Join(cardDir, fmt.Sprintf("pr-%d.md", pr))
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		text := strings.ToLower(string(data))
		if strings.Contains(text, "close-ready: yes") {
			count++
		}
	}
	return count
}

type agentSettings struct {
	DefaultProvider      string   `json:"defaultProvider,omitempty"`
	DefaultModel         string   `json:"defaultModel,omitempty"`
	DefaultThinkingLevel string   `json:"defaultThinkingLevel,omitempty"`
	EnabledModels        []string `json:"enabledModels,omitempty"`
}

func ensureAgentDir(cfg config.Config) (string, error) {
	agentDir := filepath.Join(cfg.DataRoot, "pi-agent")
	if err := os.MkdirAll(agentDir, 0o700); err != nil {
		return "", fmt.Errorf("mkdir %s: %w", agentDir, err)
	}

	if err := seedAuth(agentDir); err != nil {
		return "", err
	}

	settingsPath := filepath.Join(agentDir, "settings.json")
	if _, err := os.Stat(settingsPath); os.IsNotExist(err) {
		settings := agentSettings{
			DefaultProvider:      "openai-codex",
			DefaultModel:         "gpt-5.2",
			DefaultThinkingLevel: "high",
			EnabledModels:        []string{"openai-codex/gpt-5.2"},
		}
		data, err := json.MarshalIndent(settings, "", "  ")
		if err != nil {
			return "", fmt.Errorf("marshal settings: %w", err)
		}
		data = append(data, '\n')
		if err := os.WriteFile(settingsPath, data, 0o600); err != nil {
			return "", fmt.Errorf("write settings: %w", err)
		}
	}

	if err := os.Setenv("PI_CODING_AGENT_DIR", agentDir); err != nil {
		return "", fmt.Errorf("set PI_CODING_AGENT_DIR: %w", err)
	}
	return agentDir, nil
}

func seedAuth(agentDir string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	sourceDir := filepath.Join(home, ".pi", "agent")
	authSource := filepath.Join(sourceDir, "auth.json")
	authDest := filepath.Join(agentDir, "auth.json")
	if fileExists(authSource) {
		if err := copyIfNewer(authSource, authDest, 0o600); err != nil {
			return err
		}
	}
	oauthSource := filepath.Join(sourceDir, "oauth.json")
	oauthDest := filepath.Join(agentDir, "oauth.json")
	if fileExists(oauthSource) {
		if err := copyIfNewer(oauthSource, oauthDest, 0o600); err != nil {
			return err
		}
	}
	return nil
}

func copyIfNewer(source string, dest string, mode os.FileMode) error {
	sourceInfo, err := os.Stat(source)
	if err != nil {
		return err
	}
	if destInfo, err := os.Stat(dest); err == nil {
		if !sourceInfo.ModTime().After(destInfo.ModTime()) {
			return nil
		}
	}
	data, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	if err := os.WriteFile(dest, data, mode); err != nil {
		return err
	}
	return nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}
