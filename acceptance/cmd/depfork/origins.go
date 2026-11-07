package main

import (
	"context"
	"fmt"
	"strconv"

	"vpn-script/internal/quality/dependencies/commands"
	"vpn-script/internal/quality/dependencies/modules"
)

func origins(ctx context.Context, args []string) (modules.OriginBatch, error) {
	offset, err := strconv.Atoi(args[0])
	if err != nil {
		return modules.OriginBatch{}, fmt.Errorf("invalid origin offset: %w", err)
	}
	limit, err := strconv.Atoi(args[1])
	if err != nil {
		return modules.OriginBatch{}, fmt.Errorf("invalid origin limit: %w", err)
	}
	return modules.ResolveMissing(ctx, commands.Command, offset, limit)
}
