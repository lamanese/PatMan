package handler

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/PatchMon/PatchMon/server-source-code/internal/models"
)

func TestSshSessionClosedDetail(t *testing.T) {
	started := time.Date(2026, 9, 30, 10, 0, 0, 0, time.UTC)
	host := &models.Host{ID: "h1", FriendlyName: "web01"}
	user := &models.User{ID: "u1", Username: "ahmad"}
	d := sshSessionClosedDetail(host, user, "proxy", 2222, started, started.Add(95*time.Second+400*time.Millisecond))
	want := map[string]interface{}{"host_id": "h1", "host_name": "web01", "user": "ahmad", "mode": "proxy", "port": 2222, "duration_s": 95}
	for k, v := range want {
		if d[k] != v {
			t.Errorf("%s = %v, want %v", k, d[k], v)
		}
	}
	if len(d) != len(want) {
		t.Errorf("unexpected keys: %v", d)
	}
}

// Direct mode never records a port.
func TestSSHSessionClosedDetailDirectHasNoPort(t *testing.T) {
	started := time.Date(2026, 9, 30, 10, 0, 0, 0, time.UTC)
	host := &models.Host{ID: "h1", FriendlyName: "web01"}
	user := &models.User{ID: "u1", Username: "ahmad"}
	d := sshSessionClosedDetail(host, user, "direct", 22, started, started)
	if _, ok := d["port"]; ok {
		t.Errorf("direct mode must not record port: %v", d)
	}
}

func TestRDPOpenSessionClosedDetail(t *testing.T) {
	started := time.Date(2026, 9, 30, 10, 0, 0, 0, time.UTC)
	s := rdpOpenSession{userID: "u1", hostID: "h1", sessionID: "s1", started: started}
	d := s.closedDetail(started.Add(3 * time.Minute))
	want := map[string]interface{}{"host_id": "h1", "session_id": "s1", "user_id": "u1", "duration_s": 180}
	for k, v := range want {
		if d[k] != v {
			t.Errorf("%s = %v, want %v", k, d[k], v)
		}
	}
	if len(d) != len(want) {
		t.Errorf("unexpected keys: %v", d)
	}
}

// The disconnect hook must drop the bookkeeping entry exactly once and
// ignore unknown tunnel ids.
func TestRDPOnTunnelDisconnectRemovesEntry(t *testing.T) {
	h := &RDPHandler{}
	h.openSessions.Store("conn-1", rdpOpenSession{userID: "u1", hostID: "h1", sessionID: "s1", started: time.Now()})
	r := httptest.NewRequest("GET", "/api/v1/rdp/websocket-tunnel", nil)

	h.onTunnelDisconnect("unknown", r, nil)
	if _, ok := h.openSessions.Load("conn-1"); !ok {
		t.Fatal("unknown id removed the wrong entry")
	}
	h.onTunnelDisconnect("conn-1", r, nil)
	if _, ok := h.openSessions.Load("conn-1"); ok {
		t.Fatal("entry still present after disconnect")
	}
	h.onTunnelDisconnect("conn-1", r, nil) // second call is a no-op
}
