//go:build windows

package main

import (
	"log"
	"sync"

	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/debug"
	"golang.org/x/sys/windows/svc/mgr"
)

func connectSCM() (*mgr.Mgr, error) {
	return mgr.Connect()
}

func serviceConfig() mgr.Config {
	return mgr.Config{
		DisplayName: "Nekkus Gate Helper",
		Description: serviceDesc,
		StartType:   mgr.StartAutomatic,
	}
}

func runService() error {
	// Если запущены из консоли (не SCM) — используем debug.Run для отладки.
	isService, err := svc.IsWindowsService()
	if err != nil {
		return err
	}
	if isService {
		return svc.Run(serviceName, &pipeService{})
	}
	return debug.Run(serviceName, &pipeService{})
}

type pipeService struct {
	stopCh chan struct{}
	once   sync.Once
}

func (s *pipeService) Execute(args []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (bool, uint32) {
	const accept = svc.AcceptStop | svc.AcceptShutdown
	changes <- svc.Status{State: svc.StartPending}

	s.stopCh = make(chan struct{})
	go startPipeServer(s.stopCh)

	changes <- svc.Status{State: svc.Running, Accepts: accept}
	log.Println("Helper pipe server listening on", pipeName)

	for {
		select {
		case c := <-r:
			switch c.Cmd {
			case svc.Interrogate:
				changes <- c.CurrentStatus
			case svc.Stop, svc.Shutdown:
				changes <- svc.Status{State: svc.StopPending}
				s.once.Do(func() { close(s.stopCh) })
				return false, 0
			}
		}
	}
}

