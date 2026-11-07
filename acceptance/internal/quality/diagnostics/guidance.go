package diagnostics

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
)

type Rule string

const (
	FunctionSize          Rule = "function-size"
	FileSize              Rule = "file-size"
	DirectorySize         Rule = "directory-size"
	FileDependencies      Rule = "file-dependencies"
	DirectoryDependencies Rule = "directory-dependencies"
	PackageDependencies   Rule = "package-dependencies"
	Formatting            Rule = "formatting"
	StaticAnalysis        Rule = "static-analysis"
	SecurityAudit         Rule = "security-audit"
	TaskCatalog           Rule = "task-catalog"
	DocumentationLinks    Rule = "documentation-links"
	ResearchInventory     Rule = "research-inventory"
)

var rules = []Rule{FunctionSize, FileSize, DirectorySize, FileDependencies,
	DirectoryDependencies, PackageDependencies, Formatting, StaticAnalysis, TaskCatalog,
	DocumentationLinks, ResearchInventory, SecurityAudit}

type Finding struct {
	Rule    Rule
	Message string
}

func Issue(rule Rule, format string, arguments ...any) Finding {
	return Finding{Rule: rule, Message: fmt.Sprintf(format, arguments...)}
}

func (finding Finding) String() string {
	return fmt.Sprintf("[%s] %s\n  %s", finding.Rule, finding.Message, finding.Rule.Instruction())
}

func (rule Rule) SkillPath() string {
	if rule == SecurityAudit {
		return "ops/quality/skills/static-analysis/security-audit/SKILL.md"
	}
	return filepath.ToSlash(filepath.Join("ops", "quality", "skills", string(rule), "SKILL.md"))
}

func (rule Rule) Instruction() string {
	return fmt.Sprintf("Для исправления используй skill `%s`: %s\n  Прочитать: just explain %s", rule, rule.SkillPath(), rule)
}

func writeGuide(root, name string, output io.Writer) error {
	relative := Rule(name).SkillPath()
	contents, err := os.ReadFile(filepath.Join(root, relative))
	if err != nil {
		return fmt.Errorf("required repair instruction is missing: %s: %w", relative, err)
	}
	_, err = fmt.Fprintf(output, "\nRead and apply %s:\n%s\n", filepath.ToSlash(relative), contents)
	return err
}

func Explain(root string, rule Rule, output io.Writer) error {
	if !slices.Contains(rules, rule) {
		return fmt.Errorf("unknown rule %q; available: %v", rule, rules)
	}
	if err := writeGuide(root, "refactor", output); err != nil {
		return err
	}
	return writeGuide(root, string(rule), output)
}

func Report(root string, findings []Finding, output io.Writer) error {
	selected := map[Rule]bool{}
	for _, finding := range findings {
		if !slices.Contains(rules, finding.Rule) {
			return fmt.Errorf("unregistered diagnostic rule %q", finding.Rule)
		}
		if _, err := fmt.Fprintln(output, finding.String()); err != nil {
			return err
		}
		selected[finding.Rule] = true
	}
	if len(findings) == 0 {
		return nil
	}
	if err := writeGuide(root, "refactor", output); err != nil {
		return err
	}
	for _, rule := range rules {
		if selected[rule] {
			if err := writeGuide(root, string(rule), output); err != nil {
				return err
			}
		}
	}
	return nil
}
