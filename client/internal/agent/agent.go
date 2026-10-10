package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
)

type Runner struct {
	ConfigPath string
	Config     Config
	HTTPClient *http.Client
	cfgMu      sync.RWMutex
}

func NewRunner(cfg Config) *Runner {
	return &Runner{
		ConfigPath: DefaultConfigPath(),
		Config:     cfg,
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
	}
}

func (r *Runner) Run(ctx context.Context) error {
	go r.serveLocalUI(ctx)
	for {
		err := r.connectAndServe(ctx)
		if err != nil {
			log.Printf("ws disconnected: %v", err)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Second):
		}
	}
}

func (r *Runner) connectAndServe(ctx context.Context) error {
	r.cfgMu.RLock()
	cfg := r.Config
	r.cfgMu.RUnlock()
	u := strings.TrimSuffix(cfg.ServerURL, "/") + "/ws/agent/" + cfg.ComputerID + "?token=" + cfg.AgentToken
	u = strings.Replace(u, "https://", "wss://", 1)
	u = strings.Replace(u, "http://", "ws://", 1)
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, u, nil)
	if err != nil {
		return err
	}
	defer conn.Close()

	hbTicker := time.NewTicker(time.Duration(cfg.HeartbeatIntervalSeconds) * time.Second)
	defer hbTicker.Stop()

	if err := r.sendHeartbeat(conn); err != nil {
		return err
	}

	errCh := make(chan error, 1)
	go func() {
		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				errCh <- err
				return
			}
			if err := r.handleMessage(conn, data); err != nil {
				log.Printf("message handler error: %v", err)
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			_ = conn.WriteJSON(map[string]any{"type": "shutdown"})
			return ctx.Err()
		case err := <-errCh:
			return err
		case <-hbTicker.C:
			if err := r.sendHeartbeat(conn); err != nil {
				return err
			}
		}
	}
}

func (r *Runner) sendHeartbeat(conn *websocket.Conn) error {
	r.cfgMu.RLock()
	cfg := r.Config
	r.cfgMu.RUnlock()
	cpuPct, _ := cpu.Percent(0, false)
	vmem, _ := mem.VirtualMemory()
	cpuVal := 0.0
	if len(cpuPct) > 0 {
		cpuVal = cpuPct[0]
	}
	ramVal := 0.0
	if vmem != nil {
		ramVal = vmem.UsedPercent
	}
	return conn.WriteJSON(map[string]any{
		"type": "heartbeat",
		"data": map[string]any{
			"computer_id": cfg.ComputerID,
			"room_id":     cfg.RoomID,
			"version":     cfg.Version,
			"language":    cfg.Language,
			"cpu_usage":   cpuVal,
			"ram_usage":   ramVal,
		},
	})
}

func (r *Runner) handleMessage(conn *websocket.Conn, data []byte) error {
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}
	if m["type"] != "action" {
		return nil
	}
	action, _ := m["action"].(string)
	payload, _ := m["payload"].(map[string]any)
	r.cfgMu.RLock()
	lang := r.Config.Language
	r.cfgMu.RUnlock()
	_ = Notify("SchoolAgent", T(lang, "action_received"))

	switch action {
	case "push_config":
		r.cfgMu.Lock()
		if room, ok := payload["room_id"].(string); ok {
			r.Config.RoomID = room
		}
		if lang, ok := payload["language"].(string); ok {
			r.Config.Language = lang
		}
		if interval, ok := payload["heartbeat_interval_seconds"].(float64); ok {
			r.Config.HeartbeatIntervalSeconds = int(interval)
		}
		cfg := r.Config
		r.cfgMu.Unlock()
		return SaveConfig(r.ConfigPath, cfg)
	case "run_shell":
		cmdline, _ := payload["command"].(string)
		stdout, stderr, err := runShell(cmdline)
		return conn.WriteJSON(map[string]any{
			"type":   "action_result",
			"action": "run_shell",
			"data": map[string]any{
				"command": cmdline,
				"stdout":  stdout,
				"stderr":  stderr,
				"error":   errString(err),
			},
		})
	case "run_speedtest":
		result := r.runSpeedtest()
		r.cfgMu.RLock()
		speedLang := r.Config.Language
		r.cfgMu.RUnlock()
		_ = Notify("SchoolAgent", T(speedLang, "speedtest_done"))
		return conn.WriteJSON(map[string]any{"type": "action_result", "action": "run_speedtest", "data": result})
	case "update_check", "force_update":
		manifest, err := r.fetchManifest()
		if err != nil {
			return err
		}
		if available, _ := manifest["update_available"].(bool); available || action == "force_update" {
			dl, _ := manifest["download_url"].(string)
			return RunUpdater(r.Config, dl)
		}
	}
	return nil
}

func runShell(command string) (string, string, error) {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("powershell", "-NoProfile", "-Command", command)
	} else {
		cmd = exec.Command("sh", "-lc", command)
	}
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()
	return outBuf.String(), errBuf.String(), err
}

func (r *Runner) fetchManifest() (map[string]any, error) {
	r.cfgMu.RLock()
	cfg := r.Config
	r.cfgMu.RUnlock()
	url := strings.TrimSuffix(cfg.ServerURL, "/") + "/agent/update/manifest?current_version=" + cfg.Version
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	req.Header.Set("X-Agent-Token", cfg.AgentToken)
	resp, err := r.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *Runner) runSpeedtest() map[string]any {
	r.cfgMu.RLock()
	cfg := r.Config
	r.cfgMu.RUnlock()
	pingStart := time.Now()
	_, _ = r.HTTPClient.Get(strings.TrimSuffix(cfg.ServerURL, "/") + "/healthz")
	ping := float64(time.Since(pingStart).Milliseconds())

	downURL := strings.TrimSuffix(cfg.ServerURL, "/") + "/agent/speedtest/download"
	start := time.Now()
	req, _ := http.NewRequest(http.MethodGet, downURL, nil)
	req.Header.Set("X-Agent-Token", cfg.AgentToken)
	resp, err := r.HTTPClient.Do(req)
	if err != nil {
		return map[string]any{"download_mbps": 0, "upload_mbps": 0, "ping_ms": ping}
	}
	_ = resp.Body.Close()
	downSec := time.Since(start).Seconds()
	downloadMbps := (5 * 8) / downSec

	upPayload := bytes.Repeat([]byte("1"), 1*1024*1024)
	upStart := time.Now()
	upReq, _ := http.NewRequest(http.MethodPost, strings.TrimSuffix(cfg.ServerURL, "/")+"/agent/speedtest/upload", bytes.NewReader(upPayload))
	upReq.Header.Set("X-Agent-Token", cfg.AgentToken)
	_, _ = r.HTTPClient.Do(upReq)
	upSec := time.Since(upStart).Seconds()
	uploadMbps := (1 * 8) / upSec

	return map[string]any{
		"download_mbps": round(downloadMbps),
		"upload_mbps":   round(uploadMbps),
		"ping_ms":       ping,
	}
}

func round(v float64) float64 {
	return float64(int(v*100)) / 100
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func (r *Runner) SubmitTicket(category, description string) error {
	r.cfgMu.RLock()
	cfg := r.Config
	r.cfgMu.RUnlock()
	body, _ := json.Marshal(map[string]string{
		"computer_id": cfg.ComputerID,
		"category":    category,
		"description": description,
	})
	req, _ := http.NewRequest(http.MethodPost, strings.TrimSuffix(cfg.ServerURL, "/")+"/agent/tickets", bytes.NewReader(body))
	req.Header.Set("X-Agent-Token", cfg.AgentToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := r.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("ticket submit failed with status %d", resp.StatusCode)
	}
	return nil
}

func (r *Runner) TicketsOverview() ([]map[string]any, error) {
	r.cfgMu.RLock()
	cfg := r.Config
	r.cfgMu.RUnlock()
	req, _ := http.NewRequest(http.MethodGet, strings.TrimSuffix(cfg.ServerURL, "/")+"/agent/tickets/overview", nil)
	req.Header.Set("X-Agent-Token", cfg.AgentToken)
	resp, err := r.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("overview failed: %s", string(raw))
	}
	var items []map[string]any
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, err
	}
	return items, nil
}
