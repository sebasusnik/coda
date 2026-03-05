package setup

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func platformInstall() (string, error) {
	// Try apt (Debian/Ubuntu/Raspberry Pi OS)
	if _, err := exec.LookPath("apt-get"); err == nil {
		update := exec.Command("sudo", "apt-get", "update", "-qq")
		update.Stdout = os.Stdout
		update.Stderr = os.Stderr
		update.Run() // non-fatal

		cmd := exec.Command("sudo", "apt-get", "install", "-y", "librespot")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err == nil {
			return exec.LookPath("librespot")
		}
	}

	// Try dnf (Fedora)
	if _, err := exec.LookPath("dnf"); err == nil {
		cmd := exec.Command("sudo", "dnf", "install", "-y", "librespot")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err == nil {
			return exec.LookPath("librespot")
		}
	}

	return installViaCargo()
}

func platformInstallService(binaryPath string, cfg *SetupConfig, token string) error {
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

func platformStartService() error {
	cmd := exec.Command("systemctl", "--user", "enable", "--now", "coda-librespot.service")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("systemctl failed: %v -- %s", err, out)
	}
	return nil
}

func platformStop() error {
	cmd := exec.Command("systemctl", "--user", "stop", "coda-librespot.service")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("systemctl stop failed: %v -- %s", err, out)
	}
	return nil
}
