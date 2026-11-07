package release

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"vpn-script/internal/core/model"
	"vpn-script/internal/node/artifact"
	"vpn-script/internal/platform/safeio"
	"vpn-script/internal/runtime/engines"
)

func Prepare(ctx context.Context, data model.NodeConfig, arch, stage, cache string, group int, run func(...string) ([]byte, error)) error {
	for _, program := range data.Programs {
		binary, err := fetch(ctx, program, arch, stage, cache)
		if err != nil {
			return err
		}
		version, err := run(binary, "version")
		prefix := "Xray " + program.Runtime.Version + " "
		if program.Name == "sing-box" {
			prefix = "sing-box version " + program.Runtime.Version + "\n"
		}
		if err != nil || !strings.HasPrefix(string(version), prefix) {
			return fmt.Errorf("unexpected engine version")
		}
		path := filepath.Join(stage, program.Name+".json")
		if err := configFile(path, program.Config, group); err != nil {
			return err
		}
		if _, err := run(append([]string{binary}, engines.Arguments(program.Name, path, true)...)...); err != nil {
			return err
		}
	}
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	raw, err := safeio.Read(executable, 100_000_000)
	if err != nil {
		return err
	}
	if err := safeio.Atomic(filepath.Join(stage, "nhop-node"), raw, 0755); err != nil {
		return err
	}
	names := []string{}
	for _, program := range data.Programs {
		names = append(names, program.Name)
	}
	return configFile(filepath.Join(stage, "processes.json"), names, group)
}

func configFile(path string, value any, group int) error {
	raw, err := safeio.Encode(value)
	if err != nil {
		return err
	}
	if err := safeio.Atomic(path, raw, 0640); err != nil {
		return err
	}
	return os.Chown(path, 0, group)
}

func fetch(ctx context.Context, program model.Program, arch, stage, cache string) (string, error) {
	raw, err := artifact.Cached(ctx, program.Runtime.Assets[arch], cache)
	if err != nil {
		return "", err
	}
	if program.Name == "xray" {
		return artifact.Extract(raw, stage)
	}
	cpu := "amd64"
	if arch == "aarch64" {
		cpu = "arm64"
	}
	prefix := "sing-box-" + program.Runtime.Version + "-linux-" + cpu + "/"
	paths, err := artifact.ExtractTarFiles(raw, stage, prefix+"sing-box", prefix+"libcronet.so")
	return paths["sing-box"], err
}
