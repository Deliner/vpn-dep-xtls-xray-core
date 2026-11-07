package main

import (
	"context"
	"fmt"
	"strconv"

	"vpn-script/internal/quality/dependencies/cleanup"
	"vpn-script/internal/quality/dependencies/commands"
	"vpn-script/internal/quality/dependencies/github"
	"vpn-script/internal/quality/dependencies/scope"
)

func scopedFork(ctx context.Context, upstream, owner string) (github.ForkRecord, error) {
	if err := scope.Allows(upstream); err != nil {
		return github.ForkRecord{}, err
	}
	return github.Fork(ctx, commands.Command, upstream, owner)
}

func cleanupCommand(ctx context.Context, args []string) (any, error) {
	if len(args) == 1 && args[0] == "cleanup-plan" {
		return cleanup.Build(ctx, commands.Command)
	}
	if len(args) == 4 && args[0] == "cleanup-apply" {
		return cleanupBatch(ctx, args[1:])
	}
	if len(args) == 2 && args[0] == "cleanup-verify" {
		return cleanup.Verify(ctx, commands.Command, args[1])
	}
	return nil, fmt.Errorf("use the dependency recipes from just --list; broad fork creation has been retired")
}

func cleanupBatch(ctx context.Context, args []string) (cleanup.Batch, error) {
	offset, err := strconv.Atoi(args[1])
	if err != nil {
		return cleanup.Batch{}, err
	}
	limit, err := strconv.Atoi(args[2])
	if err != nil {
		return cleanup.Batch{}, err
	}
	return cleanup.Apply(ctx, commands.Command, args[0], offset, limit)
}
