//go:build !windows

package service

import (
	"context"

	"classroom-control/client/internal/agent"
)

func Run(cfg agent.Config) error {
	runner := agent.NewRunner(cfg)
	return runner.Run(context.Background())
}
