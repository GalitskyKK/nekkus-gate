//go:build windows

package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows/svc"
)

const (
	serviceName   = "NekkusGateHelper"
	serviceDesc   = "Nekkus Gate DNS Helper — manages system DNS settings"
	helperSubdir  = "Nekkus"
	helperExeName = "nekkus-gate-helper.exe"
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
		return fmt.Errorf("executable path: %w", err)
	}
	exePath = filepath.Clean(exePath)

	// Копируем helper в C:\ProgramData\Nekkus\, чтобы сервис (LocalSystem) всегда видел exe.
	targetPath, err := copyToProgramData(exePath)
	if err != nil {
		return fmt.Errorf("copy to ProgramData: %w", err)
	}
	targetPath = filepath.Clean(targetPath)

	m, err := connectSCM()
	if err != nil {
		return fmt.Errorf("connect SCM: %w", err)
	}
	defer m.Disconnect()

	// Если сервис уже есть (со старым путём, например E:\) — удаляем и создаём заново с путём на C:.
	if s, oerr := m.OpenService(serviceName); oerr == nil {
		_, _ = s.Control(svc.Stop)
		_ = s.Delete()
		s.Close()
	}

	s, err := m.CreateService(serviceName, targetPath, serviceConfig(), "-run")
	if err != nil {
		return fmt.Errorf("CreateService: %w", err)
	}
	defer s.Close()
	if err := s.Start(); err != nil {
		return fmt.Errorf("Start service: %w", err)
	}
	return nil
}

// copyToProgramData копирует текущий exe в C:\ProgramData\Nekkus\nekkus-gate-helper.exe и возвращает этот путь.
func copyToProgramData(sourceExe string) (string, error) {
	programData := os.Getenv("ProgramData")
	if programData == "" {
		programData = "C:\\ProgramData"
	}
	dir := filepath.Join(programData, helperSubdir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("mkdir %s: %w", dir, err)
	}
	dst := filepath.Join(dir, helperExeName)
	srcFile, err := os.Open(sourceExe)
	if err != nil {
		return "", fmt.Errorf("open source %s: %w", sourceExe, err)
	}
	defer srcFile.Close()
	dstFile, err := os.Create(dst)
	if err != nil {
		return "", fmt.Errorf("create %s: %w", dst, err)
	}
	defer dstFile.Close()
	if _, err := io.Copy(dstFile, srcFile); err != nil {
		os.Remove(dst)
		return "", fmt.Errorf("copy: %w", err)
	}
	if err := dstFile.Sync(); err != nil {
		os.Remove(dst)
		return "", fmt.Errorf("sync: %w", err)
	}
	return dst, nil
}

func isAlreadyExists(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "already exists") ||
		strings.Contains(msg, "уже существует") ||
		strings.Contains(msg, "уже зарегистрирован")
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
