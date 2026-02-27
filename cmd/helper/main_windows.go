//go:build windows

package main

import (
	"flag"
	"log"
	"os"

	"golang.org/x/sys/windows/svc"
)

const (
	serviceName = "NekkusGateHelper"
	serviceDesc = "Nekkus Gate DNS Helper — manages system DNS settings"
)

var (
	install   = flag.Bool("install", false, "Install as Windows service")
	uninstall = flag.Bool("uninstall", false, "Uninstall Windows service")
	run       = flag.Bool("run", false, "Run as service (called by SCM)")
)

func main() {
	flag.Parse()

	switch {
	case *install:
		if err := installService(); err != nil {
			log.Fatalf("Install: %v", err)
		}
		log.Println("Service installed and started")
	case *uninstall:
		if err := uninstallService(); err != nil {
			log.Fatalf("Uninstall: %v", err)
		}
		log.Println("Service uninstalled")
	case *run:
		if err := runService(); err != nil {
			log.Fatalf("Service: %v", err)
		}
	default:
		log.Fatal("Use -install, -uninstall or -run. When installed, SCM starts the service with -run.")
	}
}

func installService() error {
	exePath, err := os.Executable()
	if err != nil {
		return err
	}
	m, err := connectSCM()
	if err != nil {
		return err
	}
	defer m.Disconnect()

	s, err := m.CreateService(serviceName, exePath, serviceConfig(), "-run")
	if err != nil {
		return err
	}
	defer s.Close()
	return s.Start()
}

func uninstallService() error {
	m, err := connectSCM()
	if err != nil {
		return err
	}
	defer m.Disconnect()

	s, err := m.OpenService(serviceName)
	if err != nil {
		return err
	}
	defer s.Close()

	_, _ = s.Control(svc.Stop)
	return s.Delete()
}
