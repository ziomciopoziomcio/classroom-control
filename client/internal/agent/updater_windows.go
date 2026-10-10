//go:build windows

package agent

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
)

func RunUpdater(cfg Config, downloadURL string) error {
	tempDir := os.TempDir()
	tmpExe := filepath.Join(tempDir, "school-agent-new.exe")
	resp, err := http.Get(cfg.ServerURL + downloadURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	out, err := os.Create(tmpExe)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err = io.Copy(out, resp.Body); err != nil {
		return err
	}
	batch := filepath.Join(tempDir, "school-agent-updater.bat")
	script := "@echo off\r\n" +
		"sc stop SchoolAgent\r\n" +
		"timeout /t 3 /nobreak\r\n" +
		"copy /Y \"" + tmpExe + "\" \"C:\\Program Files\\SchoolAgent\\school-agent.exe\"\r\n" +
		"sc start SchoolAgent\r\n"
	if err := os.WriteFile(batch, []byte(script), 0o700); err != nil {
		return err
	}
	if err := exec.Command("cmd", "/C", batch).Start(); err != nil {
		return fmt.Errorf("failed starting updater script: %w", err)
	}
	return nil
}
