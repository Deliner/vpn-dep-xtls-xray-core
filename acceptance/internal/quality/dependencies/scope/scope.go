package scope

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"strconv"
	"strings"

	"vpn-script/internal/platform/safeio"
)

type Fork struct {
	Upstream string `json:"upstream"`
	Name     string `json:"name"`
	ID       int64  `json:"id"`
}

// Read is the explicit maintenance boundary, not the full transitive Go graph.
func Read() ([]Fork, string, error) {
	raw, err := safeio.Read("ops/dependencies/forks.tsv", 1<<20)
	if err != nil {
		return nil, "", err
	}
	reader := csv.NewReader(bytes.NewReader(raw))
	reader.Comma, reader.Comment, reader.FieldsPerRecord = '\t', '#', 6
	rows, err := reader.ReadAll()
	if err != nil {
		return nil, "", err
	}
	forks, err := parse(rows)
	return forks, safeio.Hash(raw), err
}

func parse(rows [][]string) ([]Fork, error) {
	forks := []Fork{}
	seen := map[string]bool{}
	ids := map[int64]bool{}
	for _, row := range rows {
		if len(row) != 6 {
			return nil, fmt.Errorf("maintenance scope requires six fields")
		}
		id, err := strconv.ParseInt(row[3], 10, 64)
		if err != nil || id <= 0 || !strings.HasPrefix(row[1], "Deliner/vpn-dep-") || strings.Count(row[1], "/") != 1 || seen[row[1]] || ids[id] {
			return nil, fmt.Errorf("invalid or duplicate maintained repository identity")
		}
		seen[row[1]], ids[id] = true, true
		forks = append(forks, Fork{Upstream: row[0], Name: row[1], ID: id})
	}
	if len(forks) == 0 {
		return nil, fmt.Errorf("maintenance scope is empty")
	}
	return forks, nil
}

func Allows(upstream string) error {
	forks, _, err := Read()
	if err != nil {
		return err
	}
	for _, fork := range forks {
		if strings.EqualFold(fork.Upstream, upstream) {
			return nil
		}
	}
	return fmt.Errorf("%s is outside ops/dependencies/forks.tsv; adding unrelated transitive forks is not authorized", upstream)
}
