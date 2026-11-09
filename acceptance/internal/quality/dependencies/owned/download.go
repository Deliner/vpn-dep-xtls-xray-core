package owned

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"vpn-script/internal/platform/safeio"
	"vpn-script/internal/quality/dependencies/commands"
	"vpn-script/internal/quality/dependencies/records"
)

// Download resolves only a reviewed full Git revision under our public namespace.
// The Go command runs outside all maintained module roots, leaving their locks intact.
func Download(ctx context.Context, run commands.Runner, module, revision string) (Record, error) {
	repository, err := requestIdentity(module, revision)
	if err != nil {
		return Record{}, err
	}
	directory, err := requestDirectory()
	if err != nil {
		return Record{}, err
	}
	result := Record{RequestDirectory: directory, Files: map[string]string{}}
	args := []string{"GOPROXY=https://proxy.golang.org,direct", "GOSUMDB=sum.golang.org", "GOPRIVATE=", "GONOPROXY=", "GONOSUMDB=", "GOTOOLCHAIN=local",
		"go", "-C", directory, "mod", "download", "-json", "github.com/" + module + "@" + revision}
	raw, commandErr := run(ctx, "env", args...)
	if err := safeio.Atomic(filepath.Join(directory, "download.json"), raw, 0600); err != nil {
		return result, err
	}
	result.Module, err = downloadResult(raw, commandErr)
	if err != nil {
		return result, err
	}
	if err := verifyIdentity(result.Module, module, repository, revision); err != nil {
		return result, err
	}
	if err := recordHashes(&result); err != nil {
		return result, err
	}
	return result, records.Save("owned-module-"+safeio.Hash([]byte(module+"@"+revision))+".json", result)
}

func downloadResult(raw []byte, commandErr error) (Module, error) {
	var module Module
	if err := json.Unmarshal(raw, &module); err != nil {
		return module, fmt.Errorf("decode module download: %w (command: %v)", err, commandErr)
	}
	if module.Error != "" {
		return module, fmt.Errorf("owned module download: %s (command: %v)", module.Error, commandErr)
	}
	return module, commandErr
}

func requestDirectory() (string, error) {
	for _, path := range []string{"build", "build/dependencies", "build/dependencies/owned-modules"} {
		if err := safeio.PrivateDir(path); err != nil {
			return "", err
		}
	}
	path, err := os.MkdirTemp("build/dependencies/owned-modules", "request-")
	if err != nil {
		return "", err
	}
	return filepath.Abs(path)
}

func recordHashes(record *Record) error {
	if err := verifyContent(record.Module); err != nil {
		return err
	}
	for label, path := range map[string]string{"info": record.Module.Info, "go_mod": record.Module.GoMod, "zip": record.Module.Zip} {
		digest, err := safeio.FileHash(path)
		if err != nil {
			return err
		}
		record.Files[label] = digest
	}
	return nil
}
