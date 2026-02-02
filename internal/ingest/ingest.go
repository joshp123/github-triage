package ingest

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/joshp123/github-triage/internal/config"
	"github.com/joshp123/github-triage/internal/gh"
	"github.com/joshp123/github-triage/internal/storage"
)

type PRState struct {
	UpdatedAt string `json:"updated_at"`
	State     string `json:"state"`
}

type State struct {
	PRs map[string]PRState `json:"prs"`
}

type PRMeta struct {
	Reopened      bool   `json:"reopened"`
	PreviousState string `json:"previous_state"`
}

type PRListItem struct {
	Number    int
	UpdatedAt string
}

type PRFiles struct {
	TotalCount int      `json:"total_count"`
	Truncated  bool     `json:"truncated"`
	Files      []string `json:"files"`
}

type graphQLPR struct {
	Number            int    `json:"number"`
	Title             string `json:"title"`
	Body              string `json:"body"`
	URL               string `json:"url"`
	State             string `json:"state"`
	UpdatedAt         string `json:"updatedAt"`
	AuthorAssociation string `json:"authorAssociation"`
	IsDraft           bool   `json:"isDraft"`
	Additions         int    `json:"additions"`
	Deletions         int    `json:"deletions"`
	ChangedFiles      int    `json:"changedFiles"`
	Author            struct {
		Login string `json:"login"`
	} `json:"author"`
	Labels struct {
		Nodes []struct {
			Name string `json:"name"`
		} `json:"nodes"`
	} `json:"labels"`
	Files struct {
		TotalCount int `json:"totalCount"`
		Nodes      []struct {
			Path string `json:"path"`
		} `json:"nodes"`
	} `json:"files"`
}

type graphQLResponse struct {
	Data struct {
		Repository struct {
			PullRequests struct {
				PageInfo struct {
					HasNextPage bool   `json:"hasNextPage"`
					EndCursor   string `json:"endCursor"`
				} `json:"pageInfo"`
				Nodes []graphQLPR `json:"nodes"`
			} `json:"pullRequests"`
		} `json:"repository"`
	} `json:"data"`
}

func Discover(ctx context.Context, cfg config.Config, limit int, state string) error {
	if err := cfg.EnsureDirs(); err != nil {
		return err
	}
	if err := prewarmMaintainers(ctx, cfg); err != nil {
		return err
	}
	return writeSamplePR(ctx, cfg, limit, state)
}

func Run(ctx context.Context, cfg config.Config, limit int, state string) error {
	if err := cfg.EnsureDirs(); err != nil {
		return err
	}
	if err := prewarmMaintainers(ctx, cfg); err != nil {
		return err
	}
	return ingestPRs(ctx, cfg, limit, state)
}

func prewarmMaintainers(ctx context.Context, cfg config.Config) error {
	out, err := gh.Run(ctx, "api", fmt.Sprintf("/orgs/%s/members", cfg.Org), "--paginate", "--jq", ".[].login")
	if err != nil {
		return err
	}
	out = bytes.TrimSpace(out)
	return storage.WriteFileAtomic(cfg.Maintainers, out, 0o644)
}

func writeSamplePR(ctx context.Context, cfg config.Config, limit int, state string) error {
	prs, err := listPRs(ctx, cfg, limit, state)
	if err != nil {
		return err
	}
	if len(prs) == 0 {
		return fmt.Errorf("no PRs found for %s", cfg.Repo)
	}
	return writePRSnapshot(cfg, cfg.SamplePath, prs[0])
}

func listPRs(ctx context.Context, cfg config.Config, limit int, state string) ([]graphQLPR, error) {
	statesClause, err := graphqlStates(state)
	if err != nil {
		return nil, err
	}
	if limit < 0 {
		limit = 0
	}

	const pageSize = 100
	const fileLimit = 50

	query := fmt.Sprintf(`
query($owner: String!, $name: String!, $first: Int!, $endCursor: String, $filesFirst: Int!) {
  repository(owner: $owner, name: $name) {
    pullRequests(first: $first, after: $endCursor, states: %s, orderBy: {field: UPDATED_AT, direction: DESC}) {
      pageInfo {
        hasNextPage
        endCursor
      }
      nodes {
        number
        title
        body
        url
        state
        updatedAt
        authorAssociation
        isDraft
        additions
        deletions
        changedFiles
        author {
          login
        }
        labels(first: 20) {
          nodes {
            name
          }
        }
        files(first: $filesFirst) {
          totalCount
          nodes {
            path
          }
        }
      }
    }
  }
}
`, statesClause)

	items := []graphQLPR{}
	endCursor := ""
	for {
		if limit > 0 && len(items) >= limit {
			break
		}
		first := pageSize
		if limit > 0 {
			remaining := limit - len(items)
			if remaining < first {
				first = remaining
			}
		}
		args := []string{
			"api",
			"graphql",
			"-f",
			fmt.Sprintf("query=%s", query),
			"-F",
			fmt.Sprintf("owner=%s", cfg.Org),
			"-F",
			fmt.Sprintf("name=%s", cfg.Name),
			"-F",
			fmt.Sprintf("first=%d", first),
			"-F",
			fmt.Sprintf("filesFirst=%d", fileLimit),
		}
		if endCursor != "" {
			args = append(args, "-F", fmt.Sprintf("endCursor=%s", endCursor))
		}
		out, err := gh.Run(ctx, args...)
		if err != nil {
			return nil, err
		}
		var resp graphQLResponse
		if err := json.Unmarshal(out, &resp); err != nil {
			return nil, fmt.Errorf("parse graphql response: %w", err)
		}
		batch := resp.Data.Repository.PullRequests.Nodes
		if len(batch) == 0 {
			break
		}
		items = append(items, batch...)
		pageInfo := resp.Data.Repository.PullRequests.PageInfo
		if !pageInfo.HasNextPage {
			break
		}
		endCursor = pageInfo.EndCursor
	}

	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}

func graphqlStates(state string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(state)) {
	case "", "open":
		return "[OPEN]", nil
	case "closed":
		return "[CLOSED, MERGED]", nil
	case "all":
		return "[OPEN, CLOSED, MERGED]", nil
	default:
		return "", fmt.Errorf("invalid state %q (want open|closed|all)", state)
	}
}

func normalizeState(state string) string {
	return strings.ToLower(strings.TrimSpace(state))
}

func ingestPRs(ctx context.Context, cfg config.Config, limit int, stateFilter string) error {
	prs, err := listPRs(ctx, cfg, limit, stateFilter)
	if err != nil {
		return err
	}

	state, err := loadState(cfg.StatePath)
	if err != nil {
		return err
	}

	openSet := map[string]PRListItem{}
	if stateFilter == "open" {
		openSet = make(map[string]PRListItem, len(prs))
		for _, pr := range prs {
			openSet[strconv.Itoa(pr.Number)] = PRListItem{Number: pr.Number, UpdatedAt: pr.UpdatedAt}
		}
	}

	for _, pr := range prs {
		key := strconv.Itoa(pr.Number)
		currentState := normalizeState(pr.State)
		prev := state.PRs[key]
		prevState := prev.State
		if prevState == "" {
			prevState = currentState
		}

		reopened := prevState != "open" && currentState == "open"
		meta := PRMeta{Reopened: reopened, PreviousState: prevState}
		if err := storage.WriteJSONAtomic(cfg.RawPRMetaPath(pr.Number), meta); err != nil {
			return err
		}

		if prev.UpdatedAt == pr.UpdatedAt && prev.State == currentState {
			state.PRs[key] = PRState{UpdatedAt: pr.UpdatedAt, State: currentState}
			continue
		}

		if err := writePRSnapshot(cfg, cfg.RawPRPath(pr.Number), pr); err != nil {
			return err
		}

		state.PRs[key] = PRState{UpdatedAt: pr.UpdatedAt, State: currentState}
	}

	if stateFilter == "open" {
		for key, prState := range state.PRs {
			if _, ok := openSet[key]; !ok {
				prState.State = "closed"
				state.PRs[key] = prState
			}
		}
	}

	return saveState(cfg.StatePath, state)
}

func writePRSnapshot(cfg config.Config, path string, pr graphQLPR) error {
	if err := storage.WriteJSONAtomic(path, pr); err != nil {
		return err
	}

	files := make([]string, 0, len(pr.Files.Nodes))
	for _, file := range pr.Files.Nodes {
		files = append(files, file.Path)
	}
	filesPayload := PRFiles{
		TotalCount: pr.Files.TotalCount,
		Truncated:  pr.Files.TotalCount > len(files),
		Files:      files,
	}
	return storage.WriteJSONAtomic(cfg.RawPRFilesPath(pr.Number), filesPayload)
}

func loadState(path string) (State, error) {
	state := State{PRs: map[string]PRState{}}
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return state, nil
		}
		return state, err
	}
	if err := storage.ReadJSON(path, &state); err != nil {
		return state, err
	}
	if state.PRs == nil {
		state.PRs = map[string]PRState{}
	}
	return state, nil
}

func saveState(path string, state State) error {
	return storage.WriteJSONAtomic(path, state)
}
