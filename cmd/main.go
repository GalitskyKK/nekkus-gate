package main

import (
	"context"
	"flag"
	"io/fs"
	"log"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/GalitskyKK/nekkus-core/pkg/config"
	"github.com/GalitskyKK/nekkus-core/pkg/desktop"
	"github.com/GalitskyKK/nekkus-core/pkg/discovery"
	coreserver "github.com/GalitskyKK/nekkus-core/pkg/server"
	pb "github.com/GalitskyKK/nekkus-core/pkg/protocol"

	"github.com/GalitskyKK/nekkus-gate/internal/blocklist"
	"github.com/GalitskyKK/nekkus-gate/internal/module"
	"github.com/GalitskyKK/nekkus-gate/internal/server"
	"github.com/GalitskyKK/nekkus-gate/internal/stats"
	"github.com/GalitskyKK/nekkus-gate/internal/sysdns"
	"github.com/GalitskyKK/nekkus-gate/ui"

	"google.golang.org/grpc"
)

var (
	httpPort   = flag.Int("port", 9003, "HTTP port")
	grpcPort   = flag.Int("grpc-port", 19003, "gRPC port")
	dnsPort    = flag.Int("dns-port", 5354, "DNS server port (UDP) when filter off; 5353 часто занят mDNS. Порт 53 — при включении фильтра (нужен admin).")
	headless   = flag.Bool("headless", false, "Run without GUI")
	trayOnly   = flag.Bool("tray-only", false, "Start minimized to tray")
	mode       = flag.String("mode", "standalone", "Run mode: standalone or hub")
	hubAddr    = flag.String("hub-addr", "", "Hub gRPC address when started by Hub")
	addr       = flag.String("addr", "", "gRPC listen address (e.g. 127.0.0.1:19003)")
	dataDirF   = flag.String("data-dir", "", "Data directory (overrides default)")
)

func waitForServer(host string, port int, timeout time.Duration) {
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 200*time.Millisecond)
		if err == nil {
			conn.Close()
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func main() {
	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	grpcPortVal := *grpcPort
	if *addr != "" {
		if _, portStr, err := net.SplitHostPort(*addr); err == nil {
			if p, err := strconv.Atoi(portStr); err == nil {
				grpcPortVal = p
			}
		}
	}

	dataDir := *dataDirF
	if dataDir == "" {
		dataDir = config.GetDataDir("gate")
	}

	st := stats.New()
	bl := blocklist.New()
	blocklistPath := filepath.Join(dataDir, "blocklist.txt")
	if err := bl.Load(blocklistPath); err != nil {
		if !os.IsNotExist(err) {
			log.Printf("blocklist load: %v", err)
		}
	} else {
		log.Printf("blocklist loaded: %d domains from %s", bl.Count(), blocklistPath)
	}

	// При старте восстанавливаем системный DNS, если остался бэкап (например, после сбоя).
	if sysdns.HasBackup(dataDir) {
		if err := sysdns.Disable(dataDir); err != nil {
			log.Printf("restore DNS on startup: %v", err)
		}
	}

	dnsRunner := server.NewDNSRunner(bl, st, "8.8.8.8:53")
	dnsRunner.Start(*dnsPort)
	defer dnsRunner.Shutdown()

	dnsAddr := "127.0.0.1:" + strconv.Itoa(*dnsPort)
	uiFS, _ := fs.Sub(ui.Assets, "frontend/dist")
	srv := coreserver.New(*httpPort, grpcPortVal, uiFS)
	server.RegisterRoutes(srv, st, bl, dataDir, dnsRunner, *dnsPort)

	go func() {
		if err := srv.Start(ctx); err != nil && ctx.Err() == nil {
			log.Printf("HTTP server: %v", err)
		}
	}()

	mod := module.New(*httpPort)
	go func() {
		if err := srv.StartGRPC(func(s *grpc.Server) {
			pb.RegisterNekkusModuleServer(s, mod)
		}); err != nil && ctx.Err() == nil {
			log.Printf("gRPC server: %v", err)
		}
	}()

	disc, err := discovery.Announce(discovery.ModuleAnnouncement{
		ID:       "gate",
		Name:     "Nekkus Gate",
		HTTPPort: *httpPort,
		GRPCPort: grpcPortVal,
	})
	if err != nil {
		log.Printf("Discovery: %v", err)
	} else {
		defer disc.Shutdown()
	}

	log.Printf("Nekkus Gate → http://localhost:%d, DNS udp://%s (filter: enable in UI)", *httpPort, dnsAddr)
	_ = hubAddr

	showUIFromHub := os.Getenv("NEKKUS_SHOW_UI") == "1"
	runHeadless := *headless || (*mode == "hub" && !showUIFromHub)

	if runHeadless {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		<-sig
		cancel()
	} else {
		waitForServer("127.0.0.1", *httpPort, 5*time.Second)
		desktop.Launch(desktop.AppConfig{
			ModuleID:   "gate",
			ModuleName: "Nekkus Gate",
			HTTPPort:   *httpPort,
			IconBytes:  nil,
			Headless:   false,
			TrayOnly:   *trayOnly,
			TrayMenuItems: nil,
			OnQuit:     func() { cancel() },
		})
	}
}
