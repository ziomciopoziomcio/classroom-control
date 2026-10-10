//go:build windows

package service

import (
	"context"
	"time"

	"classroom-control/client/internal/agent"
	"golang.org/x/sys/windows/svc"
)

type schoolService struct {
	runner *agent.Runner
}

func (s *schoolService) Execute(_ []string, r <-chan svc.ChangeRequest, status chan<- svc.Status) (bool, uint32) {
	status <- svc.Status{State: svc.StartPending}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go s.runner.Run(ctx)
	status <- svc.Status{State: svc.Running, Accepts: svc.AcceptStop | svc.AcceptShutdown}

	for c := range r {
		switch c.Cmd {
		case svc.Interrogate:
			status <- c.CurrentStatus
		case svc.Stop, svc.Shutdown:
			status <- svc.Status{State: svc.StopPending}
			cancel()
			time.Sleep(2 * time.Second)
			return false, 0
		}
	}
	return false, 0
}

func Run(cfg agent.Config) error {
	runner := agent.NewRunner(cfg)
	isService, err := svc.IsWindowsService()
	if err != nil {
		return err
	}
	if !isService {
		return runner.Run(context.Background())
	}
	return svc.Run("SchoolAgent", &schoolService{runner: runner})
}
