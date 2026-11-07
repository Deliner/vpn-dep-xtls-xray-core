package documents

import (
	"bufio"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var inlineLink = regexp.MustCompile(`\]\((<[^>]+>|[^\s()]+)(?:\s+"[^"]*")?\)`)
var referenceLink = regexp.MustCompile(`^\s*\[[^\]]+\]:\s+(<[^>]+>|\S+)`)

func linkTarget(source, target string) error {
	target = strings.Trim(target, "<>")
	address, err := url.Parse(target)
	if err != nil {
		return fmt.Errorf("invalid link %q", target)
	}
	if address.IsAbs() || address.Host != "" || address.Path == "" {
		return nil
	}
	path := filepath.Join(filepath.Dir(source), filepath.FromSlash(address.Path))
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("missing local target %q", target)
	}
	return nil
}

func Links(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 4096), 2_000_000)
	var findings []string
	line, fence := 0, ""
	for scanner.Scan() {
		line++
		text := strings.TrimSpace(scanner.Text())
		next, skip := advanceFence(text, fence)
		fence = next
		if skip {
			continue
		}
		matches := append(inlineLink.FindAllStringSubmatch(text, -1), referenceLink.FindAllStringSubmatch(text, -1)...)
		for _, match := range matches {
			if err := linkTarget(path, match[1]); err != nil {
				findings = append(findings, fmt.Sprintf("%s:%d: %v", path, line, err))
			}
		}
	}
	return findings, scanner.Err()
}

func advanceFence(text, current string) (string, bool) {
	if current != "" {
		if text == current {
			return "", true
		}
		return current, true
	}
	if strings.HasPrefix(text, "```") || strings.HasPrefix(text, "~~~") {
		marker := text[:3]
		for len(marker) < len(text) && text[len(marker)] == marker[0] {
			marker += marker[:1]
		}
		return marker, true
	}
	return "", false
}
