package catalog

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"net/url"
	"sort"
	"strings"

	"vpn-script/internal/platform/safeio"
)

type mirror struct {
	Source, Repository, Commit, Tree string
}

func readMirrors() (map[string]mirror, string, error) {
	raw, err := safeio.Read("ops/dependencies/mirrors.tsv", 1<<20)
	if err != nil {
		return nil, "", err
	}
	reader := csv.NewReader(bytes.NewReader(raw))
	reader.Comma, reader.Comment, reader.FieldsPerRecord = '\t', '#', 4
	rows, err := reader.ReadAll()
	if err != nil {
		return nil, "", err
	}
	mirrors, err := parseMirrors(rows)
	return mirrors, safeio.Hash(raw), err
}

func parseMirrors(rows [][]string) (map[string]mirror, error) {
	result := map[string]mirror{}
	for _, row := range rows {
		if len(row) != 4 {
			return nil, fmt.Errorf("mirror row requires source URL, repository, commit and tree")
		}
		item := mirror{Source: row[0], Repository: row[1], Commit: row[2], Tree: row[3]}
		address, err := url.Parse(item.Source)
		if err != nil || address.Scheme != "https" || address.Hostname() == "" || address.Hostname() == "github.com" || address.User != nil || address.RawQuery != "" || address.Fragment != "" {
			return nil, fmt.Errorf("invalid external mirror source URL: %s", item.Source)
		}
		if !namePattern.MatchString(item.Repository) || !commitPattern.MatchString(item.Commit) || !commitPattern.MatchString(item.Tree) {
			return nil, fmt.Errorf("invalid exact mirror identity: %s", item.Source)
		}
		key := item.Source + "\x00" + item.Commit
		if _, exists := result[key]; exists {
			return nil, fmt.Errorf("duplicate mirror source/commit: %s", item.Source)
		}
		result[key] = item
	}
	return result, nil
}

func applyMirrors(plan Plan, mirrors map[string]mirror) Plan {
	entries := map[string]*Entry{}
	for _, entry := range plan.Entries {
		entries[strings.ToLower(entry.Upstream)] = &entry
	}
	remaining := []Unresolved{}
	for _, item := range plan.Unresolved {
		mapping, ok := mirrors[item.URL+"\x00"+item.Commit]
		if !ok {
			remaining = append(remaining, item)
			continue
		}
		key := strings.ToLower(mapping.Repository)
		if entries[key] == nil {
			entries[key] = &Entry{Upstream: mapping.Repository}
		}
		ref := item.Reference
		ref.SourceURL = item.URL
		entries[key].References = append(entries[key].References, ref)
	}
	plan.Entries, plan.Unresolved = []Entry{}, remaining
	for _, entry := range entries {
		plan.Entries = append(plan.Entries, *entry)
	}
	sort.Slice(plan.Entries, func(i, j int) bool {
		return strings.ToLower(plan.Entries[i].Upstream) < strings.ToLower(plan.Entries[j].Upstream)
	})
	return plan
}
