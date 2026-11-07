package validate

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSSHPathIsLiteralAfterResolvingSymlinks(t *testing.T) {
	root := sshPathFixture(t)
	for _, name := range []string{"ordinary", "with spaces", "literal %h", "quote \" and \\ slash"} {
		path := filepath.Join(root, name)
		if actual, err := SSHPath(path); err != nil || actual != path {
			t.Fatalf("literal path refused: %q; %v", name, err)
		}
	}
	for index, name := range []string{"${OUTSIDE}/hosts", "new\nline", "carriage\rreturn"} {
		target := filepath.Join(root, name)
		if _, err := SSHPath(target); err == nil {
			t.Fatalf("expandable/control path accepted: %q", name)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, nil, 0600); err != nil {
			t.Fatal(err)
		}
		link := filepath.Join(root, string(rune('a'+index)))
		if err := os.Symlink(target, link); err != nil {
			t.Fatal(err)
		}
		if _, err := SSHPath(link); err == nil {
			t.Fatal("symlink concealed an unsafe resolved path")
		}
	}
}

func sshPathFixture(t *testing.T) string {
	t.Helper()
	home, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	root := filepath.Join(home, ".ssh-custom")
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	return root
}
