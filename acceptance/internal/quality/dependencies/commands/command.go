package commands

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"time"

	"vpn-script/internal/platform/process"
)

type Runner func(context.Context, string, ...string) ([]byte, error)

type capture struct {
	bytes.Buffer
	overflow bool
}

func (c *capture) Write(data []byte) (int, error) {
	const limit = 8 * 1024 * 1024
	length := len(data)
	remaining := limit - c.Len()
	if length > remaining {
		c.overflow = true
		data = data[:remaining]
	}
	_, _ = c.Buffer.Write(data)
	return length, nil
}

func Command(parent context.Context, binary string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(parent, 5*time.Minute)
	defer cancel()
	command := process.Command(ctx, binary, args...)
	defer process.Stop(command)
	command.Env = append(os.Environ(), "LC_ALL=C", "GH_PROMPT_DISABLED=1", "GIT_TERMINAL_PROMPT=0", "GOWORK=off")
	var output, diagnostic capture
	command.Stdout, command.Stderr = &output, &diagnostic
	err := command.Run()
	if output.overflow || diagnostic.overflow {
		return nil, fmt.Errorf("%s output exceeded capture limit", binary)
	}
	if err != nil {
		return output.Bytes(), fmt.Errorf("%s: %w: %s", binary, err, diagnostic.String())
	}
	return output.Bytes(), nil
}
