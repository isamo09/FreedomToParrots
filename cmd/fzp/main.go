// Freedom To Parrots — a single self-contained binary that runs an
// encrypted, WebRTC-disguised tunnel panel on Windows, Linux, macOS, the
// BSDs and Termux/Android alike. See internal/session for the tunnel
// lifecycle, internal/webui for the browser panel and internal/console for
// the always-on-screen operator dashboard this file drives.
package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/isamo09/FreedomToParrots/internal/console"
	"github.com/isamo09/FreedomToParrots/internal/corebin"
	"github.com/isamo09/FreedomToParrots/internal/session"
	"github.com/isamo09/FreedomToParrots/internal/store"
	"github.com/isamo09/FreedomToParrots/internal/webui"
)

const defaultPort = 8858

// version is set via -ldflags "-X main.version=..." by the release
// workflow; a build straight from `go build` stays "dev".
var version = "dev" //nolint:gochecknoglobals // set at link time

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "-v" || os.Args[1] == "--version") {
		fmt.Println("Freedom To Parrots " + version)

		return
	}

	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "freedomtoparrots: "+err.Error())
		os.Exit(1)
	}
}

func run() error {
	dirs, dataNote, err := store.Resolve()
	if err != nil {
		return fmt.Errorf("resolve data directory: %w", err)
	}

	settings, _, err := store.LoadSettings(dirs.Root)
	if err != nil {
		return fmt.Errorf("load settings: %w", err)
	}

	corePath, coreErr := corebin.Extract(dirs.Bin)

	mgr, err := session.New(dirs, corePath, coreErr)
	if err != nil {
		return fmt.Errorf("load sessions: %w", err)
	}

	mgr.Restore()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	mgr.RunLoops(ctx)

	host, port := bindAddr()

	httpSrv := &http.Server{
		Addr:              net.JoinHostPort(host, strconv.Itoa(port)),
		Handler:           webui.New(mgr, settings.Password).Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	ln, err := net.Listen("tcp", httpSrv.Addr)
	if err != nil {
		return fmt.Errorf("panel can't listen on %s: %w", httpSrv.Addr, err)
	}

	go func() {
		if err := httpSrv.Serve(ln); err != nil && err != http.ErrServerClosed {
			fmt.Fprintln(os.Stderr, "freedomtoparrots: panel http server: "+err.Error())
		}
	}()

	dash := console.New(mgr, panelURLs(host, port), settings.Password, dataNote, version, stop)
	dash.Run(ctx) // blocks until Ctrl+C / SIGTERM

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_ = httpSrv.Shutdown(shutdownCtx)
	mgr.StopAll()

	return nil
}

func bindAddr() (string, int) {
	host := os.Getenv("FTP_HOST")
	if host == "" {
		host = "0.0.0.0"
	}

	port := defaultPort

	if v := os.Getenv("FTP_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil && p > 0 && p < 65536 {
			port = p
		}
	}

	return host, port
}

// panelURLs lists every address the panel can plausibly be reached at when
// listening on all interfaces: real, active, routable IPv4 addresses (skips
// link-local 169.254.0.0/16 autoconfig addresses - Windows machines tend to
// have several of those on disabled/virtual adapters, and none of them are
// ever the right address to open), plus loopback. If the operator pinned a
// specific bind address, that's the only URL shown.
func panelURLs(host string, port int) []string {
	if host != "0.0.0.0" && host != "::" {
		return []string{fmt.Sprintf("http://%s/", net.JoinHostPort(host, strconv.Itoa(port)))}
	}

	var urls []string

	ifaces, _ := net.Interfaces()
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, a := range addrs {
			ipNet, ok := a.(*net.IPNet)
			if !ok {
				continue
			}

			ip4 := ipNet.IP.To4()
			if ip4 == nil || ip4.IsLinkLocalUnicast() {
				continue
			}

			urls = append(urls, fmt.Sprintf("http://%s/", net.JoinHostPort(ip4.String(), strconv.Itoa(port))))
		}
	}

	urls = append(urls, fmt.Sprintf("http://127.0.0.1:%d/", port))

	return urls
}
