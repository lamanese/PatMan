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

func discardRequest(hostID, userID string) *http.Request {
	r := routedRequest(http.MethodDelete, "/api/v1/hosts/"+hostID+"/integrations/pending-config", "",
		map[string]string{"hostId": hostID})
	return r.WithContext(context.WithValue(r.Context(), middleware.UserIDKey, userID))
}

func auditRows(t *testing.T, d *database.DB, event, hostID string) int {
	t.Helper()
	var n int
	if err := d.RawQueryRow(context.Background(), `SELECT count(*) FROM audit_logs WHERE event = $1 AND details LIKE $2`,
		event, `%"host_id":"`+hostID+`"%`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestDiscardPendingConfigAuditsAndClears(t *testing.T) {
	d := newHandlerTestDB(t)
	hostID := insertSolvedTestHost(t, d, "web01")
	h := hostsAuditTestHandler(d)
	w := httptest.NewRecorder()
	h.ToggleIntegration(w, toggleRequest(hostID, "docker", `{"enabled":true}`, "user-3"))
	if w.Code != http.StatusOK || pendingConfigRows(t, d, hostID) != 1 {
		t.Fatalf("setup: status=%d body=%s", w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	h.DiscardPendingConfig(w, discardRequest(hostID, "user-3"))
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "Pending configuration discarded") {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if got := pendingConfigRows(t, d, hostID); got != 0 {
		t.Fatalf("pending config rows=%d, want 0", got)
	}
	var n int
	if err := d.RawQueryRow(context.Background(), `SELECT count(*) FROM audit_logs WHERE event = 'integration_config_discarded' AND user_id = 'user-3' AND success
		AND details LIKE '%"docker":true%' AND details LIKE '%"host_id":"`+hostID+`"%'`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("audit rows=%d err=%v", n, err)
	}
}

func TestDiscardPendingConfigWithoutPendingIsNoop(t *testing.T) {
	d := newHandlerTestDB(t)
	hostID := insertSolvedTestHost(t, d, "web01")
	h := hostsAuditTestHandler(d)
	w := httptest.NewRecorder()
	h.DiscardPendingConfig(w, discardRequest(hostID, "user-3"))
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "No pending configuration") {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if got := auditRows(t, d, "integration_config_discarded", hostID); got != 0 {
		t.Fatalf("audit rows=%d, want 0", got)
	}
}

func TestDiscardPendingConfigUnknownHost(t *testing.T) {
	d := newHandlerTestDB(t)
	h := hostsAuditTestHandler(d)
	w := httptest.NewRecorder()
	h.DiscardPendingConfig(w, discardRequest("00000000-0000-0000-0000-000000000000", "user-3"))
	if w.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s, want 404", w.Code, w.Body.String())
	}
}

func TestDiscardPendingConfigFailsClosedWithoutAudit(t *testing.T) {
	d := newHandlerTestDB(t)
	hostID := insertSolvedTestHost(t, d, "web01")
	h := hostsAuditTestHandler(d)
	w := httptest.NewRecorder()
	h.ToggleIntegration(w, toggleRequest(hostID, "docker", `{"enabled":false}`, "user-3"))
	if w.Code != http.StatusOK || pendingConfigRows(t, d, hostID) != 1 {
		t.Fatalf("setup: status=%d body=%s", w.Code, w.Body.String())
	}
	h.db = fakeProvider{nil} // audit write cannot succeed, pending store stays real
	w = httptest.NewRecorder()
	h.DiscardPendingConfig(w, discardRequest(hostID, "user-3"))
	if w.Code != http.StatusInternalServerError || !strings.Contains(w.Body.String(), "Failed to write audit log") {
		t.Fatalf("status=%d body=%s, want 500 audit failure", w.Code, w.Body.String())
	}
	if got := pendingConfigRows(t, d, hostID); got != 1 {
		t.Fatalf("pending config rows=%d, want 1 (no side effect without audit)", got)
	}
}

func hostComplianceRequest(hostID, path, body, userID string) *http.Request {
	r := routedRequest(http.MethodPost, "/api/v1/hosts/"+hostID+"/integrations/compliance/"+path, body,
		map[string]string{"hostId": hostID})
	return r.WithContext(context.WithValue(r.Context(), middleware.UserIDKey, userID))
}

func TestSetComplianceModeWritesAuditBeforePending(t *testing.T) {
	d := newHandlerTestDB(t)
	hostID := insertSolvedTestHost(t, d, "web01")
	h := hostsAuditTestHandler(d)
	w := httptest.NewRecorder()
	h.SetComplianceMode(w, hostComplianceRequest(hostID, "mode", `{"mode":"on-demand"}`, "user-3"))
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var n int
	if err := d.RawQueryRow(context.Background(), `SELECT count(*) FROM audit_logs WHERE event = 'compliance_config_requested' AND user_id = 'user-3' AND success
		AND details LIKE '%"change":"mode"%' AND details LIKE '%"mode":"on-demand"%' AND details LIKE '%"host_id":"`+hostID+`"%'`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("audit rows=%d err=%v", n, err)
	}
	if got := pendingConfigRows(t, d, hostID); got != 1 {
		t.Fatalf("pending config rows=%d, want 1", got)
	}
}

func TestSetComplianceModeFailsClosedWithoutAudit(t *testing.T) {
	d := newHandlerTestDB(t)
	hostID := insertSolvedTestHost(t, d, "web01")
	h := hostsAuditTestHandler(d)
	h.db = nil
	w := httptest.NewRecorder()
	h.SetComplianceMode(w, hostComplianceRequest(hostID, "mode", `{"mode":"enabled"}`, "user-3"))
	if w.Code != http.StatusInternalServerError || !strings.Contains(w.Body.String(), "Failed to write audit log") {
		t.Fatalf("status=%d body=%s, want 500 audit failure", w.Code, w.Body.String())
	}
	if got := pendingConfigRows(t, d, hostID); got != 0 {
		t.Fatalf("pending config rows=%d, want 0", got)
	}
}
