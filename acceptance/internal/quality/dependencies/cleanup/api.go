package cleanup

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"

	"vpn-script/internal/quality/dependencies/commands"
)

func api(ctx context.Context, run commands.Runner, args ...string) ([]byte, error) {
	return run(ctx, "gh", append([]string{"api", "--hostname", "github.com"}, args...)...)
}

func authenticate(ctx context.Context, run commands.Runner) error {
	raw, err := api(ctx, run, "user", "--jq", "{login,id}")
	if err != nil {
		return err
	}
	var user struct {
		Login string
		ID    int64
	}
	if err := json.Unmarshal(raw, &user); err != nil {
		return err
	}
	if user.Login != "Deliner" || user.ID != accountID {
		return fmt.Errorf("cleanup requires the authorized Deliner account ID")
	}
	return nil
}

func repositories(ctx context.Context, run commands.Runner) ([]Repository, []byte, error) {
	query := "[.[] | {id,full_name,owner:{id:.owner.id,login:.owner.login},fork,private,created_at}]"
	raw, err := api(ctx, run, "--paginate", "user/repos?affiliation=owner&per_page=100&sort=full_name", "--jq", query)
	if err != nil {
		return nil, raw, err
	}
	repos, err := decodePages(raw)
	return repos, raw, err
}

func decodePages(raw []byte) ([]Repository, error) {
	result := []Repository{}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	for {
		var page []Repository
		err := decoder.Decode(&page)
		if err == io.EOF {
			return result, nil
		}
		if err != nil {
			return nil, err
		}
		result = append(result, page...)
	}
}

func repository(ctx context.Context, run commands.Runner, name string) (Repository, bool, error) {
	var repo Repository
	raw, err := api(ctx, run, "repos/"+name)
	if err != nil {
		var response struct{ Status string }
		if json.Unmarshal(raw, &response) == nil && response.Status == "404" {
			return repo, false, nil
		}
		return repo, false, err
	}
	err = json.Unmarshal(raw, &repo)
	return repo, true, err
}
