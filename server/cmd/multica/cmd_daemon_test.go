package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/spf13/cobra"
)

func TestRunDaemonStart_WithInstallTokenAlreadyRunningRedeemsAndSyncs(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	profile := fmt.Sprintf("already-running-%d", time.Now().UnixNano())
	var syncCalls int32
	startRunningDaemonHealth(t, profile, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&syncCalls, 1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"synced"}`))
	})

	var exchangeCalls int32
	exchangeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/install-tokens/exchange" {
			atomic.AddInt32(&exchangeCalls, 1)
			var body map[string]string
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Errorf("decode exchange request: %v", err)
				http.Error(w, "bad request", http.StatusBadRequest)
				return
			}
			if body["token"] != "mit_test_already_running" {
				t.Errorf("exchange token = %q", body["token"])
				http.Error(w, "bad token", http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{
				"daemon_token": "mdt_saved",
				"workspace_id": "ws-new",
				"daemon_id":    body["daemon_id"],
				"expires_at":   time.Now().Add(time.Hour).Format(time.RFC3339),
			})
			return
		}
		http.Error(w, "exchange should not be called", http.StatusInternalServerError)
	}))
	t.Cleanup(exchangeServer.Close)

	cmd := newDaemonStartTestCommand(t)
	mustSetFlag(t, cmd, "profile", profile)
	mustSetFlag(t, cmd, "server-url", exchangeServer.URL)
	mustSetFlag(t, cmd, "install-token", "mit_test_already_running")

	err := runDaemonStart(cmd, nil)
	if err != nil {
		t.Fatalf("runDaemonStart returned error: %v", err)
	}
	if got := atomic.LoadInt32(&exchangeCalls); got != 1 {
		t.Fatalf("exchange calls = %d, want 1", got)
	}
	if got := atomic.LoadInt32(&syncCalls); got != 1 {
		t.Fatalf("sync calls = %d, want 1", got)
	}
}

func newDaemonStartTestCommand(t *testing.T) *cobra.Command {
	t.Helper()
	cmd := &cobra.Command{}
	cmd.Flags().Bool("foreground", false, "")
	cmd.Flags().String("daemon-id", "", "")
	cmd.Flags().String("device-name", "", "")
	cmd.Flags().String("runtime-name", "", "")
	cmd.Flags().Duration("poll-interval", 0, "")
	cmd.Flags().Duration("heartbeat-interval", 0, "")
	cmd.Flags().Duration("agent-timeout", 0, "")
	cmd.Flags().Duration("codex-semantic-inactivity-timeout", 0, "")
	cmd.Flags().Int("max-concurrent-tasks", 0, "")
	cmd.Flags().Bool("no-auto-update", false, "")
	cmd.Flags().Duration("auto-update-interval", 0, "")
	cmd.Flags().String("install-token", "", "")
	cmd.Flags().String("server-url", "", "")
	cmd.Flags().String("profile", "", "")
	return cmd
}

func mustSetFlag(t *testing.T, cmd *cobra.Command, name, value string) {
	t.Helper()
	if err := cmd.Flags().Set(name, value); err != nil {
		t.Fatalf("set flag %s: %v", name, err)
	}
}

func startRunningDaemonHealth(t *testing.T, profile string, sync http.HandlerFunc) {
	t.Helper()
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", healthPortForProfile(profile)))
	if err != nil {
		t.Skipf("health port unavailable for profile %q: %v", profile, err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": "running",
			"pid":    12345,
		})
	})
	if sync != nil {
		mux.HandleFunc("/workspaces/sync", sync)
	}
	srv := &http.Server{Handler: mux}
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = srv.Serve(ln)
	}()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
		<-done
	})
}
