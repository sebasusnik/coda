//go:build !darwin && !linux

package setup

import "fmt"

func platformInstall() (string, error) {
	return "", fmt.Errorf("unsupported OS")
}

func platformInstallService(_ string, _ *SetupConfig, _ string) error {
	return fmt.Errorf("unsupported OS")
}

func platformStartService() error {
	return fmt.Errorf("unsupported OS")
}

func platformStop() error {
	return fmt.Errorf("unsupported OS")
}
