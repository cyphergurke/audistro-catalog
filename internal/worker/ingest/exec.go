package ingest

import (
	"context"
	"fmt"
	"os/exec"
)

type ExecRunner struct{}

func (ExecRunner) Run(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %v: %w output=%s", name, args, err, string(output))
	}
	return nil
}
