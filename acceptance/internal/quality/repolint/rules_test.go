package repolint_test

import (
	"fmt"
	"strings"
	"testing"

	"vpn-script/internal/quality/repolint"
)

func TestFortyLinesBoundary(t *testing.T) {
	for _, count := range []int{40, 41} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			code := "package sample\n\nfunc Long() {\n" + strings.Repeat("\t_ = 1\n", count-2) + "}\n"
			root := fixture(t, map[string]string{"long.go": code})
			findings, err := repolint.Source(root)
			if err != nil {
				t.Fatal(err)
			}
			if (len(findings) != 0) != (count > 40) {
				t.Fatalf("count %d: %v", count, findings)
			}
		})
	}
}

func TestThreeHundredLinesFileBoundary(t *testing.T) {
	for _, count := range []int{300, 301} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			code := "package sample\n\n" + strings.Repeat("// line\n", count-2)
			root := fixture(t, map[string]string{"large_test.go": code})
			findings, err := repolint.Source(root)
			if err != nil {
				t.Fatal(err)
			}
			if (len(findings) != 0) != (count > 300) {
				t.Fatalf("count %d: %v", count, findings)
			}
		})
	}
}

func TestAnonymousAndTestFunctionsAreChecked(t *testing.T) {
	code := "package sample\n\nvar callback = func() {\n" + strings.Repeat("\t_ = 1\n", 39) + "}\n"
	root := fixture(t, map[string]string{"example_test.go": code})
	findings, err := repolint.Source(root)
	if err != nil || len(findings) != 1 || !strings.Contains(findings[0].String(), "41 lines") {
		t.Fatalf("anonymous function in test escaped lint: %v, %v", findings, err)
	}
}

func TestLineDirectiveCannotHideLongFunction(t *testing.T) {
	code := "package sample\n\nfunc Long() {\n" + strings.Repeat("\t_ = 1\n", 37) + "//line hidden.go:1\n\t_ = 1\n}\n"
	root := fixture(t, map[string]string{"long.go": code})
	findings, err := repolint.Source(root)
	if err != nil || !strings.Contains(fmt.Sprint(findings), "41 lines") {
		t.Fatalf("line directive bypassed physical function length: %v, %v", findings, err)
	}
}

func TestLineDirectiveCannotHideFileCycle(t *testing.T) {
	root := fixture(t, map[string]string{
		"a.go": "package sample\n//line hidden.go:1\nfunc A() { B() }\n",
		"b.go": "package sample\n//line hidden.go:1\nfunc B() { A() }\n",
	})
	findings, err := dependencies(t, root, "linux/amd64")
	if err != nil || !strings.Contains(fmt.Sprint(findings), "file dependency cycle") {
		t.Fatalf("line directive bypassed file cycle: %v, %v", findings, err)
	}
}

func TestFileCycleRejected(t *testing.T) {
	root := fixture(t, map[string]string{"a.go": "package sample\nfunc A() { B() }\n", "b.go": "package sample\nfunc B() { A() }\n"})
	findings, err := dependencies(t, root, "linux/amd64")
	if err != nil || len(findings) == 0 || !strings.Contains(fmt.Sprint(findings), "file dependency cycle") {
		t.Fatalf("file cycle escaped lint: %v, %v", findings, err)
	}
}

func TestLocalShadowDoesNotCreateFileEdge(t *testing.T) {
	root := fixture(t, map[string]string{"a.go": "package sample\nfunc A() { B := func() {}; B() }\n",
		"b.go": "package sample\nfunc B() { A() }\n"})
	findings, err := dependencies(t, root, "linux/amd64")
	if err != nil || len(findings) != 0 {
		t.Fatalf("false dependency from local shadow: %v, %v", findings, err)
	}
}

func TestDirectoryCycleWithoutImportCycleRejected(t *testing.T) {
	root := fixture(t, map[string]string{
		"a/one/a.go": "package one\nimport \"lintfixture/b/two\"\nfunc A() { two.B() }\n",
		"b/one/b.go": "package one\nimport \"lintfixture/a/two\"\nfunc B() { two.A() }\n",
		"a/two/a.go": "package two\nfunc A() {}\n", "b/two/b.go": "package two\nfunc B() {}\n",
	})
	findings, err := dependencies(t, root, "linux/amd64")
	if err != nil || len(findings) == 0 || !strings.Contains(fmt.Sprint(findings), "directory level 1") {
		t.Fatalf("directory cycle escaped lint: %v, %v", findings, err)
	}
}

func TestPackageImportCycleRejected(t *testing.T) {
	root := fixture(t, map[string]string{
		"a/a.go": "package a\nimport \"lintfixture/b\"\nfunc A() { b.B() }\n",
		"b/b.go": "package b\nimport \"lintfixture/a\"\nfunc B() { a.A() }\n",
	})
	findings, err := dependencies(t, root, "linux/amd64")
	if err != nil || !strings.Contains(fmt.Sprint(findings), "package-dependencies") {
		t.Fatal("compiler import cycle escaped lint")
	}
}

func TestBuildTargetSpecificCycle(t *testing.T) {
	root := fixture(t, map[string]string{
		"b.go":        "package sample\nfunc B() { A() }\n",
		"a_linux.go":  "package sample\nfunc A() { B() }\n",
		"a_darwin.go": "package sample\nfunc A() {}\n",
	})
	for _, platform := range []string{"linux/amd64", "darwin/arm64"} {
		findings, err := dependencies(t, root, platform)
		if err != nil || (len(findings) > 0) != strings.HasPrefix(platform, "linux") {
			t.Fatalf("%s build filtering: %v, %v", platform, findings, err)
		}
	}
}
