package commands

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"patchmon-agent/internal/config"
)

func withTestCfgManager(t *testing.T) *config.Manager {
	t.Helper()
	dir := t.TempDir()
	m := config.New()
	m.SetConfigFile(filepath.Join(dir, "config.yml"))
	m.GetConfig().CredentialsFile = filepath.Join(dir, "credentials.yml")
	m.GetConfig().LogFile = filepath.Join(dir, "logs", "agent.log")
	prev := cfgManager
	cfgManager = m
	t.Cleanup(func() { cfgManager = prev })
	return m
}

func TestToggleIntegrationRejectsLocalOnlyKeys(t *testing.T) {
	m := withTestCfgManager(t)
	for _, name := range []string{"ssh-proxy-enabled", "rdp-proxy-enabled"} {
		err := toggleIntegration(name, true)
		if !errors.Is(err, config.ErrIntegrationNotServerManaged) {
			t.Fatalf("%s: err=%v", name, err)
		}
		if m.IsIntegrationEnabled(name) {
			t.Fatalf("%s must stay off", name)
		}
	}
	if _, err := os.Stat(m.GetConfigFile()); !os.IsNotExist(err) {
		t.Fatalf("config.yml must not be written, stat err=%v", err)
	}
}

func TestSyncIntegrationsFromServerSkipsLocalOnlyKeys(t *testing.T) {
	m := withTestCfgManager(t)
	changed := syncIntegrationsFromServer(map[string]bool{
		"docker":            true,
		"ssh-proxy-enabled": true,
		"rdp-proxy-enabled": true,
	})
	if !changed {
		t.Fatal("docker change must be reported")
	}
	if !m.IsIntegrationEnabled("docker") {
		t.Fatal("docker must be enabled")
	}
	if m.IsIntegrationEnabled("ssh-proxy-enabled") || m.IsIntegrationEnabled("rdp-proxy-enabled") {
		t.Fatal("proxy keys must never be set by server sync")
	}
}

func TestProxyTargetIsAlwaysLocal(t *testing.T) {
	if got := proxyTargetHost("192.168.50.10"); got != "localhost" {
		t.Fatalf("got %q", got)
	}
	if got := proxyTargetHost(""); got != "localhost" {
		t.Fatalf("got %q", got)
	}
}

func TestRDPProxyTargetPortIsAlways3389(t *testing.T) {
	for _, in := range []int{0, 3389, 5432} {
		if got := rdpProxyTargetPort(in); got != 3389 {
			t.Fatalf("rdpProxyTargetPort(%d) = %d, want 3389", in, got)
		}
	}
}
