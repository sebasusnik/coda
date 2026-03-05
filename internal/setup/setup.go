package setup

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
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

	cfg := &SetupConfig{DeviceName: "coda"}

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

// installLibrespot installs librespot using the best available method for the
// current platform and returns the path to the installed binary.
func installLibrespot() (string, error) {
	if path, err := exec.LookPath("librespot"); err == nil {
		ui.Infof("librespot already installed at %s, skipping", path)
		return path, nil
	}
	return platformInstall()
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
	return platformInstallService(binaryPath, cfg, token)
}

func startService() error {
	return platformStartService()
}

func Start() error {
	if err := startService(); err != nil {
		return err
	}
	ui.Success("service started")
	return nil
}

func Stop() error {
	if err := platformStop(); err != nil {
		return err
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
	defer func() { _ = in.Close() }()

	out, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}

	if _, err = io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}
