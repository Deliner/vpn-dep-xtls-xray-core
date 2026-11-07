package mirroring

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"net/url"
	"regexp"

	"vpn-script/internal/platform/safeio"
)

var namePattern = regexp.MustCompile(`^vpn-dep-[a-z0-9._-]+$`)
var commitPattern = regexp.MustCompile(`^[0-9a-f]{40}$`)

func readSource(name string) (Source, error) {
	raw, err := safeio.Read("ops/dependencies/git-sources.tsv", 1<<20)
	if err != nil {
		return Source{}, err
	}
	reader := csv.NewReader(bytes.NewReader(raw))
	reader.Comma, reader.Comment, reader.FieldsPerRecord = '\t', '#', 3
	rows, err := reader.ReadAll()
	if err != nil {
		return Source{}, err
	}
	return selectSource(rows, name)
}

func selectSource(rows [][]string, name string) (Source, error) {
	seen := map[string]Source{}
	for _, row := range rows {
		if len(row) != 3 {
			return Source{}, fmt.Errorf("source row requires name, URL and commit")
		}
		item := Source{Name: row[0], URL: row[1], Commit: row[2]}
		address, err := url.Parse(item.URL)
		if !namePattern.MatchString(item.Name) || len(item.Name) > 100 || !commitPattern.MatchString(item.Commit) || err != nil || address.Scheme != "https" || address.Hostname() == "" || address.User != nil || address.RawQuery != "" || address.Fragment != "" {
			return Source{}, fmt.Errorf("invalid reviewed Git source identity")
		}
		if _, exists := seen[item.Name]; exists {
			return Source{}, fmt.Errorf("duplicate Git source name")
		}
		seen[item.Name] = item
	}
	item, found := seen[name]
	if !found {
		return Source{}, fmt.Errorf("source name is not in the reviewed Git source table")
	}
	return item, nil
}
