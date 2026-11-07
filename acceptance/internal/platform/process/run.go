package process

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"time"
)

const OutputLimit = 65536

type Result struct {
	Output  []byte
	Success bool
}

type Runner func(context.Context, time.Duration, ...string) (Result, error)

type bounded struct {
	bytes.Buffer
}

func (b *bounded) Write(data []byte) (int, error) {
	length := len(data)
	if b.Len() < OutputLimit {
		_, _ = b.Buffer.Write(data[:min(length, OutputLimit-b.Len())])
	}
	return length, nil
}

func Run(parent context.Context, limit time.Duration, args ...string) (Result, error) {
	if len(args) == 0 {
		return Result{}, fmt.Errorf("empty command")
	}
	ctx, cancel := context.WithTimeout(parent, limit)
	defer cancel()
	command := Command(ctx, args[0], args[1:]...)
	defer Stop(command)
	command.Env = append(os.Environ(), "LC_ALL=C")
	var output bounded
	command.Stdout, command.Stderr = &output, &output
	err := command.Run()
	return Result{Output: output.Bytes(), Success: err == nil}, err
}
