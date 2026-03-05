package setup

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func platformInstall() (string, error) {
	if _, err := exec.LookPath("brew"); err == nil {
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

func platformInstallService(binaryPath string, cfg *SetupConfig, token string) error {
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

func platformStartService() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	plistPath := filepath.Join(homeDir, "Library", "LaunchAgents", "com.coda.librespot.plist")

	// Unload first in case it was previously installed
	_ = exec.Command("launchctl", "unload", plistPath).Run()

	cmd := exec.Command("launchctl", "load", plistPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("launchctl load failed: %v -- %s", err, out)
	}
	return nil
}

func platformStop() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	plistPath := filepath.Join(homeDir, "Library", "LaunchAgents", "com.coda.librespot.plist")
	cmd := exec.Command("launchctl", "unload", plistPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("launchctl unload failed: %v -- %s", err, out)
	}
	return nil
}
