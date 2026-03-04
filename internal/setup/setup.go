package setup

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/sebasusnik/coda/internal/auth"
	"github.com/sebasusnik/coda/internal/device"
	"github.com/sebasusnik/coda/internal/ui"
)

type SetupConfig struct {
	DeviceName string
}

func Run() error {
	ui.Header("coda device setup")
	ui.Info("this will install librespot on your machine and register it as a Spotify Connect device")
	fmt.Println()

	cfg, err := promptSetupConfig()
	if err != nil {
		return err
	}

	fmt.Println()

	// 1. Install librespot
	binaryPath, err := installLibrespot()
	if err != nil {
		return fmt.Errorf("failed to install librespot: %v", err)
	}
	ui.Success("librespot installed")

	// 2. Get a valid access token from coda's own auth
	token, err := auth.GetValidToken()
	if err != nil {
		return fmt.Errorf("not authenticated -- run 'coda auth' first: %v", err)
	}

	// 3. Install as a service
	if err := installService(binaryPath, cfg, token); err != nil {
		return fmt.Errorf("failed to install service: %v", err)
	}
	ui.Success("service installed")

	// 4. Start the service
	if err := startService(); err != nil {
		return fmt.Errorf("failed to start service: %v", err)
	}
	ui.Success("service started")

	// 5. Wait for librespot to register with Spotify Connect, then save device ID
	ui.Info("waiting for device to register...")
	if err := waitAndSaveDevice(cfg.DeviceName); err != nil {
		ui.Infof("could not auto-select device: %v", err)
		ui.Infof("run 'coda device use' once it appears in 'coda device list'")
	}

	fmt.Println()
	ui.Successf("device \"%s\" is ready -- run 'coda play' to start listening", cfg.DeviceName)
	return nil
}

func waitAndSaveDevice(name string) error {
	ticker := time.NewTicker(2 * time.Second)
	timeout := time.After(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			fmt.Print(".")
			if err := device.Use(name); err == nil {
				fmt.Println()
				return nil
			}
		case <-timeout:
			fmt.Println()
			return fmt.Errorf("timed out waiting for device \"%s\" to appear", name)
		}
	}
}

func promptSetupConfig() (*SetupConfig, error) {
	reader := bufio.NewReader(os.Stdin)

	defaultName := defaultDeviceName()
	fmt.Printf("  Device name (default: %s): ", defaultName)
	name, _ := reader.ReadString('\n')
	name = strings.TrimSpace(name)
	if name == "" {
		name = defaultName
	}

	return &SetupConfig{
		DeviceName: name,
	}, nil
}

func defaultDeviceName() string {
	hostname, err := os.Hostname()
	if err != nil {
		return "coda"
	}
	return strings.TrimSuffix(hostname, ".local")
}

// installLibrespot installs librespot using the best available method for the
// current platform and returns the path to the installed binary.
func installLibrespot() (string, error) {
	// Check if already installed
	if path, err := exec.LookPath("librespot"); err == nil {
		ui.Infof("librespot already installed at %s, skipping", path)
		return path, nil
	}

	switch runtime.GOOS {
	case "darwin":
		return installMac()
	case "linux":
		return installLinux()
	default:
		return "", fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
}

func installMac() (string, error) {
	// Try Homebrew first
	if _, err := exec.LookPath("brew"); err == nil {
		ui.Info("installing via Homebrew...")
		cmd := exec.Command("brew", "install", "librespot")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return "", fmt.Errorf("brew install failed: %v", err)
		}
		return exec.LookPath("librespot")
	}
	return installViaCargo()
}

func installLinux() (string, error) {
	// Try apt (Debian/Ubuntu/Raspberry Pi OS)
	if _, err := exec.LookPath("apt-get"); err == nil {
		ui.Info("installing via apt...")
		update := exec.Command("sudo", "apt-get", "update", "-qq")
		update.Stdout = os.Stdout
		update.Stderr = os.Stderr
		update.Run() // non-fatal

		cmd := exec.Command("sudo", "apt-get", "install", "-y", "librespot")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			ui.Info("apt install failed, falling back to cargo...")
			return installViaCargo()
		}
		return exec.LookPath("librespot")
	}

	// Try dnf (Fedora)
	if _, err := exec.LookPath("dnf"); err == nil {
		ui.Info("installing via dnf...")
		cmd := exec.Command("sudo", "dnf", "install", "-y", "librespot")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			ui.Info("dnf install failed, falling back to cargo...")
			return installViaCargo()
		}
		return exec.LookPath("librespot")
	}

	return installViaCargo()
}

func installViaCargo() (string, error) {
	if _, err := exec.LookPath("cargo"); err != nil {
		return "", fmt.Errorf(
			"neither a system package manager nor cargo (Rust) was found\n" +
				"  install Rust from https://rustup.rs and re-run 'coda device setup'",
		)
	}

	ui.Info("installing via cargo (this may take a few minutes)...")
	cmd := exec.Command("cargo", "install", "librespot")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("cargo install failed: %v", err)
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	path := filepath.Join(homeDir, ".cargo", "bin", "librespot")
	if _, err := os.Stat(path); err != nil {
		return "", fmt.Errorf("cargo install succeeded but binary not found at %s", path)
	}
	return path, nil
}

// cacheDir returns the librespot credential/audio cache directory.
func cacheDir() string {
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, ".cache", "coda", "librespot")
}

func installService(binaryPath string, cfg *SetupConfig, token string) error {
	switch runtime.GOOS {
	case "darwin":
		return installLaunchdService(binaryPath, cfg, token)
	case "linux":
		return installSystemdService(binaryPath, cfg, token)
	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
}

func startService() error {
	switch runtime.GOOS {
	case "darwin":
		return startLaunchdService()
	case "linux":
		return startSystemdService()
	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
}

func installLaunchdService(binaryPath string, cfg *SetupConfig, token string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	launchAgentsDir := filepath.Join(homeDir, "Library", "LaunchAgents")
	if err := os.MkdirAll(launchAgentsDir, 0755); err != nil {
		return err
	}

	cache := cacheDir()
	if err := os.MkdirAll(cache, 0700); err != nil {
		return err
	}

	plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>com.coda.librespot</string>
	<key>ProgramArguments</key>
	<array>
		<string>%s</string>
		<string>--name</string>
		<string>%s</string>
		<string>--access-token</string>
		<string>%s</string>
		<string>--cache</string>
		<string>%s</string>
		<string>--disable-audio-cache</string>
	</array>
	<key>RunAtLoad</key>
	<true/>
	<key>KeepAlive</key>
	<true/>
	<key>StandardOutPath</key>
	<string>%s/Library/Logs/coda-librespot.log</string>
	<key>StandardErrorPath</key>
	<string>%s/Library/Logs/coda-librespot.error.log</string>
</dict>
</plist>`, binaryPath, cfg.DeviceName, token, cache, homeDir, homeDir)

	plistPath := filepath.Join(launchAgentsDir, "com.coda.librespot.plist")
	// 0600 — access token is stored in this file
	return os.WriteFile(plistPath, []byte(plist), 0600)
}

func startLaunchdService() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	plistPath := filepath.Join(homeDir, "Library", "LaunchAgents", "com.coda.librespot.plist")

	// Unload first in case it was previously installed
	exec.Command("launchctl", "unload", plistPath).Run()

	cmd := exec.Command("launchctl", "load", plistPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("launchctl load failed: %v -- %s", err, out)
	}
	return nil
}

func installSystemdService(binaryPath string, cfg *SetupConfig, token string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	cache := cacheDir()
	if err := os.MkdirAll(cache, 0700); err != nil {
		return err
	}

	systemdDir := filepath.Join(homeDir, ".config", "systemd", "user")
	if err := os.MkdirAll(systemdDir, 0755); err != nil {
		return err
	}

	service := fmt.Sprintf(`[Unit]
Description=Coda librespot (Spotify Connect)
After=network.target sound.target

[Service]
Type=simple
ExecStart=%s --name "%s" --access-token "%s" --cache "%s" --disable-audio-cache
Restart=always
RestartSec=10

[Install]
WantedBy=default.target`, binaryPath, cfg.DeviceName, token, cache)

	servicePath := filepath.Join(systemdDir, "coda-librespot.service")
	// 0600 — access token is stored in this file
	if err := os.WriteFile(servicePath, []byte(service), 0600); err != nil {
		return err
	}

	exec.Command("systemctl", "--user", "daemon-reload").Run()
	return nil
}

func startSystemdService() error {
	cmd := exec.Command("systemctl", "--user", "enable", "--now", "coda-librespot.service")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("systemctl failed: %v -- %s", err, out)
	}
	return nil
}

func Start() error {
	if err := startService(); err != nil {
		return err
	}
	ui.Success("service started")
	return nil
}

func Stop() error {
	switch runtime.GOOS {
	case "darwin":
		return stopLaunchdService()
	case "linux":
		return stopSystemdService()
	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
}

func stopLaunchdService() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	plistPath := filepath.Join(homeDir, "Library", "LaunchAgents", "com.coda.librespot.plist")
	cmd := exec.Command("launchctl", "unload", plistPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("launchctl unload failed: %v -- %s", err, out)
	}
	ui.Success("service stopped")
	return nil
}

func stopSystemdService() error {
	cmd := exec.Command("systemctl", "--user", "stop", "coda-librespot.service")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("systemctl stop failed: %v -- %s", err, out)
	}
	ui.Success("service stopped")
	return nil
}

func Install() error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("could not determine binary path: %v", err)
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("could not determine home directory: %v", err)
	}
	binDir := filepath.Join(homeDir, ".local", "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		return fmt.Errorf("could not create %s: %v", binDir, err)
	}
	dest := filepath.Join(binDir, "coda")
	if err := copyFile(exe, dest); err != nil {
		return fmt.Errorf("failed to install: %v", err)
	}
	ui.Successf("installed to %s", dest)
	return nil
}

func copyFile(src, dest string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}
