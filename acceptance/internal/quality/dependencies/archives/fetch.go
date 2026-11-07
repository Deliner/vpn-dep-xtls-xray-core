package archives

import (
	"context"
	"os"
	"path"
	"path/filepath"
	"strings"

	"vpn-script/internal/platform/safeio"
	"vpn-script/internal/quality/dependencies/checkout"
	"vpn-script/internal/quality/dependencies/commands"
)

func Fetch(ctx context.Context, run commands.Runner, root, revision string) (Pin, error) {
	if err := checkout.Verify(ctx, run, root, revision); err != nil {
		return Pin{}, err
	}
	identity, err := checkout.ReadIdentity(ctx, run, root, revision)
	if err != nil {
		return Pin{}, err
	}
	expected, err := snapshot(ctx, run, root)
	if err != nil {
		return Pin{}, err
	}
	repo := strings.TrimSuffix(strings.TrimPrefix(identity.Repository, "https://github.com/"), ".git")
	name := path.Base(repo) + "-" + revision
	pin := Pin{Source: identity, URL: "https://codeload.github.com/" + repo + "/tar.gz/" + revision, Files: len(expected)}
	pin.Submodules = submodules(expected)
	pin.Files -= len(pin.Submodules)
	directory, raw, err := download(ctx, run, name, pin.URL)
	if err != nil {
		return pin, err
	}
	if err := verifyArchive(raw, name+"/", expected); err != nil {
		return pin, err
	}
	if err := checkout.Verify(ctx, run, root, revision); err != nil {
		return pin, err
	}
	pin.SHA256, pin.Size = safeio.Hash(raw), len(raw)
	return pin, safeio.JSON(filepath.Join(directory, "pin.json"), pin)
}

func download(ctx context.Context, run commands.Runner, name, address string) (string, []byte, error) {
	parent := "build/dependencies/source-pins"
	if err := safeio.PrivateDir(parent); err != nil {
		return "", nil, err
	}
	directory := filepath.Join(parent, name)
	if err := os.Mkdir(directory, 0700); err != nil {
		return "", nil, err
	}
	archive := filepath.Join(directory, "source.tar.gz")
	_, err := run(ctx, "curl", "--fail", "--location", "--retry", "3", "--connect-timeout", "15", "--max-time", "240",
		"--proto", "=https", "--proto-redir", "=https", "--max-filesize", "67108864", "--output", archive, address)
	if err != nil {
		return directory, nil, err
	}
	raw, err := safeio.Read(archive, 64*1024*1024)
	return directory, raw, err
}
