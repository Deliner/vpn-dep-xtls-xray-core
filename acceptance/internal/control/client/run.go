package client

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"time"
	"vpn-script/internal/core/model"
	"vpn-script/internal/platform/process"
	"vpn-script/internal/platform/safeio"
	"vpn-script/internal/runtime/engines"
)

func Detect(config model.Object) (string, error) {
	if config["routing"] != nil && config["route"] == nil {
		return "xray", nil
	}
	if config["route"] != nil && config["routing"] == nil {
		return "sing-box", nil
	}
	return "", fmt.Errorf("unrecognized native client config")
}

func ValidateBinary(ctx context.Context, engine, binary string, run process.Runner) error {
	if binary == "" {
		return fmt.Errorf("provide --%s with pinned runtime %s", engine, engines.Runtime(engine).Version)
	}
	result, err := run(ctx, 10*time.Second, binary, "version")
	prefix := "Xray " + engines.Runtime(engine).Version + " "
	if engine == "sing-box" {
		prefix = "sing-box version " + engines.Runtime(engine).Version + "\n"
	}
	if err != nil || !result.Success || !bytes.HasPrefix(result.Output, []byte(prefix)) {
		return fmt.Errorf("requires %s %s", engine, engines.Runtime(engine).Version)
	}
	return nil
}

func Run(ctx context.Context, path string, binaries map[string]string, output io.Writer) error {
	var config model.Object
	if err := safeio.LoadJSON(path, &config); err != nil {
		return err
	}
	engine, err := Detect(config)
	if err != nil {
		return err
	}
	binary := binaries[engine]
	if err := ValidateBinary(ctx, engine, binary, process.Run); err != nil {
		return err
	}
	command := process.Command(ctx, binary, engines.Arguments(engine, path, false)...)
	defer process.Stop(command)
	command.Stdin, command.Stdout, command.Stderr = os.Stdin, output, output
	return command.Run()
}
