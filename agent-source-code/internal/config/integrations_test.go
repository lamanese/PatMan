package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func newTestManager(t *testing.T) *Manager {
	t.Helper()
	dir := t.TempDir()
	m := New()
	m.SetConfigFile(filepath.Join(dir, "config.yml"))
	m.GetConfig().CredentialsFile = filepath.Join(dir, "credentials.yml")
	m.GetConfig().LogFile = filepath.Join(dir, "logs", "agent.log")
	return m
}

func TestIsServerManagedIntegration(t *testing.T) {
	for _, name := range []string{"docker", "compliance"} {
		if !IsServerManagedIntegration(name) {
			t.Fatalf("%s must be server managed", name)
		}
	}
	for _, name := range []string{"ssh-proxy-enabled", "rdp-proxy-enabled", "", "Docker", "proxmox"} {
		if IsServerManagedIntegration(name) {
			t.Fatalf("%q must not be server managed", name)
		}
	}
}

func TestSetIntegrationEnabledFromServerRejectsProxyKeys(t *testing.T) {
	m := newTestManager(t)
	for _, name := range []string{"ssh-proxy-enabled", "rdp-proxy-enabled", "anything"} {
		err := m.SetIntegrationEnabledFromServer(name, true)
		if !errors.Is(err, ErrIntegrationNotServerManaged) {
			t.Fatalf("%s: err=%v want ErrIntegrationNotServerManaged", name, err)
		}
		if m.IsIntegrationEnabled(name) {
			t.Fatalf("%s must stay disabled in memory", name)
		}
	}
	if _, err := os.Stat(m.GetConfigFile()); !os.IsNotExist(err) {
		t.Fatalf("config.yml must not be written on rejection, stat err=%v", err)
	}
}

func TestSetIntegrationEnabledFromServerAcceptsDocker(t *testing.T) {
	m := newTestManager(t)
	if err := m.SetIntegrationEnabledFromServer("docker", true); err != nil {
		t.Fatalf("docker: %v", err)
	}
	if !m.IsIntegrationEnabled("docker") {
		t.Fatal("docker must be enabled")
	}
	if _, err := os.Stat(m.GetConfigFile()); err != nil {
		t.Fatalf("config.yml must be written: %v", err)
	}
}
