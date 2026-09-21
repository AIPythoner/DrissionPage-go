package drissionpage

import (
	"context"
	"fmt"
	"github.com/go-rod/rod/lib/launcher"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

func (o *ChromiumOptions) AutoPort(on bool) *ChromiumOptions         { o.UseAutoPort = on; return o }
func (o *ChromiumOptions) UseSystemProfile(on bool) *ChromiumOptions { o.SystemUserPath = on; return o }
func (o *ChromiumOptions) UseEdge() *ChromiumOptions                 { o.BrowserPath = "msedge"; return o }
func (o *ChromiumOptions) SetLocalPort(port int) *ChromiumOptions {
	o.Address = fmt.Sprintf("127.0.0.1:%d", port)
	o.UseAutoPort = false
	return o
}

// ConnectOrLaunch matches Python's local-address behavior: attach when available,
// otherwise launch on that local port. ExistingOnly prohibits launching. Remote
// addresses and WebSocket endpoints are never interpreted as launch requests.
func ConnectOrLaunch(ctx context.Context, options *ChromiumOptions) (*Chromium, error) {
	if options == nil {
		return NewChromium(ctx)
	}
	browser, attachErr := NewChromium(ctx, options)
	if attachErr == nil {
		return browser, nil
	}
	if options.ExistingOnly || options.Address == "" || ctx.Err() != nil {
		return nil, attachErr
	}
	address := options.Address
	if !strings.Contains(address, "://") {
		address = "http://" + address
	}
	parsed, err := url.Parse(address)
	if err != nil || parsed.Scheme != "http" {
		return nil, attachErr
	}
	ip := net.ParseIP(parsed.Hostname())
	if parsed.Hostname() != "localhost" && (ip == nil || !ip.IsLoopback()) || parsed.Port() == "" {
		return nil, attachErr
	}
	// A listening service is not an absent browser; preserve its diagnostic.
	connection, err := net.DialTimeout("tcp", parsed.Host, options.Timeout)
	if err == nil {
		connection.Close()
		return nil, attachErr
	}
	copied := *options
	copied.Arguments = append([]string(nil), options.Arguments...)
	copied.Address = ""
	copied.Argument("--remote-debugging-port", parsed.Port())
	return NewChromium(ctx, &copied)
}

func browserExecutable(value string) (string, error) {
	if value == "" || value == "chrome" || value == "chromium" {
		if path, err := exec.LookPath(value); value != "" && err == nil {
			return path, nil
		}
		if path, ok := launcher.LookPath(); ok {
			return path, nil
		}
	}
	if value == "msedge" || value == "edge" {
		if path, err := exec.LookPath("msedge"); err == nil {
			return path, nil
		}
		if runtime.GOOS == "windows" {
			for _, base := range []string{os.Getenv("PROGRAMFILES(X86)"), os.Getenv("PROGRAMFILES"), os.Getenv("LOCALAPPDATA")} {
				path := filepath.Join(base, "Microsoft", "Edge", "Application", "msedge.exe")
				if info, err := os.Stat(path); err == nil && !info.IsDir() {
					return path, nil
				}
			}
		}
	}
	if value != "" {
		if path, err := exec.LookPath(value); err == nil {
			return path, nil
		}
		if info, err := os.Stat(value); err == nil && !info.IsDir() {
			return value, nil
		}
	}
	return "", fmt.Errorf("Chromium executable not found: %q", value)
}
func systemProfilePath(browser string) (string, error) {
	edge := strings.Contains(strings.ToLower(browser), "edge")
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	switch runtime.GOOS {
	case "windows":
		if edge {
			return filepath.Join(os.Getenv("LOCALAPPDATA"), "Microsoft", "Edge", "User Data"), nil
		}
		return filepath.Join(os.Getenv("LOCALAPPDATA"), "Google", "Chrome", "User Data"), nil
	case "darwin":
		if edge {
			return filepath.Join(home, "Library", "Application Support", "Microsoft Edge"), nil
		}
		return filepath.Join(home, "Library", "Application Support", "Google", "Chrome"), nil
	default:
		base := os.Getenv("XDG_CONFIG_HOME")
		if base == "" {
			base = filepath.Join(home, ".config")
		}
		if edge {
			return filepath.Join(base, "microsoft-edge"), nil
		}
		return filepath.Join(base, "google-chrome"), nil
	}
}
