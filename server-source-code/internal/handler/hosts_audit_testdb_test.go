package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/PatchMon/PatchMon/server-source-code/internal/database"
	"github.com/PatchMon/PatchMon/server-source-code/internal/middleware"
	"github.com/PatchMon/PatchMon/server-source-code/internal/store"
)

func hostsAuditTestHandler(d *database.DB) *HostsHandler {
	p := fakeProvider{d}
	return &HostsHandler{hosts: store.NewHostsStore(p), pendingConfig: store.NewPendingConfigStore(p), db: p}
}

func toggleRequest(hostID, integration, body, userID string) *http.Request {
	r := routedRequest(http.MethodPost, "/api/v1/hosts/"+hostID+"/integrations/"+integration+"/toggle", body,
		map[string]string{"hostId": hostID, "integrationName": integration})
	return r.WithContext(context.WithValue(r.Context(), middleware.UserIDKey, userID))
}

func pendingConfigRows(t *testing.T, d *database.DB, hostID string) int {
	t.Helper()
	var n int
	if err := d.RawQueryRow(context.Background(), `SELECT count(*) FROM host_pending_config WHERE host_id = $1`, hostID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestToggleIntegrationWritesAuditBeforePending(t *testing.T) {
	d := newHandlerTestDB(t)
	hostID := insertSolvedTestHost(t, d, "web01")
	h := hostsAuditTestHandler(d)
	w := httptest.NewRecorder()
	h.ToggleIntegration(w, toggleRequest(hostID, "docker", `{"enabled":true}`, "user-3"))
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var n int
	if err := d.RawQueryRow(context.Background(), `SELECT count(*) FROM audit_logs WHERE event = 'integration_toggle_requested' AND user_id = 'user-3' AND success
		AND details LIKE '%"integration":"docker"%' AND details LIKE '%"enabled":true%' AND details LIKE '%"host_id":"`+hostID+`"%'`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("audit rows=%d err=%v", n, err)
	}
	if got := pendingConfigRows(t, d, hostID); got != 1 {
		t.Fatalf("pending config rows=%d, want 1", got)
	}
}

func TestToggleIntegrationFailsClosedWithoutAudit(t *testing.T) {
	d := newHandlerTestDB(t)
	hostID := insertSolvedTestHost(t, d, "web01")
	h := hostsAuditTestHandler(d)
	h.db = nil // audit write cannot succeed
	w := httptest.NewRecorder()
	h.ToggleIntegration(w, toggleRequest(hostID, "compliance", `{"enabled":true}`, "user-3"))
	if w.Code != http.StatusInternalServerError || !strings.Contains(w.Body.String(), "Failed to write audit log") {
		t.Fatalf("status=%d body=%s, want 500 audit failure", w.Code, w.Body.String())
	}
	if got := pendingConfigRows(t, d, hostID); got != 0 {
		t.Fatalf("pending config rows=%d, want 0 (no side effect without audit)", got)
	}
}
