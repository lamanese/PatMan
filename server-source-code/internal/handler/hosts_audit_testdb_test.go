package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/PatchMon/PatchMon/server-source-code/internal/agentregistry"
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

func hostComplianceAuditRows(t *testing.T, d *database.DB, hostID, change string, extra ...string) int {
	t.Helper()
	q := `SELECT count(*) FROM audit_logs WHERE event = 'compliance_config_requested' AND user_id = 'user-3' AND success
		AND details LIKE $1 AND details LIKE $2`
	args := []any{`%"host_id":"` + hostID + `"%`, `%"change":"` + change + `"%`}
	for _, e := range extra {
		args = append(args, "%"+e+"%")
		q += " AND details LIKE $" + strconv.Itoa(len(args))
	}
	var n int
	if err := d.RawQueryRow(context.Background(), q, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func hostDefaultProfile(t *testing.T, d *database.DB, hostID string) *string {
	t.Helper()
	var p *string
	if err := d.RawQueryRow(context.Background(), `SELECT compliance_default_profile_id FROM hosts WHERE id = $1`, hostID).Scan(&p); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestSetComplianceScannersWritesAuditBeforePending(t *testing.T) {
	d := newHandlerTestDB(t)
	hostID := insertSolvedTestHost(t, d, "web01")
	h := hostsAuditTestHandler(d)
	w := httptest.NewRecorder()
	h.SetComplianceScanners(w, hostComplianceRequest(hostID, "scanners", `{"openscap_enabled":false,"docker_bench_enabled":true}`, "user-3"))
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if n := hostComplianceAuditRows(t, d, hostID, "scanners", `"openscap_enabled":false`, `"docker_bench_enabled":true`); n != 1 {
		t.Fatalf("audit rows=%d, want 1", n)
	}
	if got := pendingConfigRows(t, d, hostID); got != 1 {
		t.Fatalf("pending config rows=%d, want 1", got)
	}
}

func TestSetComplianceScannersFailsClosedWithoutAudit(t *testing.T) {
	d := newHandlerTestDB(t)
	hostID := insertSolvedTestHost(t, d, "web01")
	h := hostsAuditTestHandler(d)
	h.db = fakeProvider{nil}
	w := httptest.NewRecorder()
	h.SetComplianceScanners(w, hostComplianceRequest(hostID, "scanners", `{"openscap_enabled":true}`, "user-3"))
	if w.Code != http.StatusInternalServerError || !strings.Contains(w.Body.String(), "Failed to write audit log") {
		t.Fatalf("status=%d body=%s, want 500 audit failure", w.Code, w.Body.String())
	}
	if got := pendingConfigRows(t, d, hostID); got != 0 {
		t.Fatalf("pending config rows=%d, want 0", got)
	}
}

func TestSetComplianceDefaultProfileWritesAuditBeforeUpdate(t *testing.T) {
	d := newHandlerTestDB(t)
	hostID := insertSolvedTestHost(t, d, "web01")
	h := hostsAuditTestHandler(d)
	w := httptest.NewRecorder()
	h.SetComplianceDefaultProfile(w, hostComplianceRequest(hostID, "default-profile", `{"profile_id":"xccdf_cis_level1"}`, "user-3"))
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if n := hostComplianceAuditRows(t, d, hostID, "default_profile", `"profile_id":"xccdf_cis_level1"`); n != 1 {
		t.Fatalf("audit rows=%d, want 1", n)
	}
	if p := hostDefaultProfile(t, d, hostID); p == nil || *p != "xccdf_cis_level1" {
		t.Fatalf("stored profile=%v, want xccdf_cis_level1", p)
	}
}

func TestSetComplianceDefaultProfileFailsClosedWithoutAudit(t *testing.T) {
	d := newHandlerTestDB(t)
	hostID := insertSolvedTestHost(t, d, "web01")
	if _, err := d.Exec(context.Background(), `UPDATE hosts SET compliance_default_profile_id = 'old_profile' WHERE id = $1`, hostID); err != nil {
		t.Fatal(err)
	}
	h := hostsAuditTestHandler(d)
	h.db = fakeProvider{nil}
	w := httptest.NewRecorder()
	h.SetComplianceDefaultProfile(w, hostComplianceRequest(hostID, "default-profile", `{"profile_id":"new_profile"}`, "user-3"))
	if w.Code != http.StatusInternalServerError || !strings.Contains(w.Body.String(), "Failed to write audit log") {
		t.Fatalf("status=%d body=%s, want 500 audit failure", w.Code, w.Body.String())
	}
	if p := hostDefaultProfile(t, d, hostID); p == nil || *p != "old_profile" {
		t.Fatalf("stored profile=%v, want old_profile unchanged", p)
	}
}

func applyRequest(hostID, userID string) *http.Request {
	r := routedRequest(http.MethodPost, "/api/v1/hosts/"+hostID+"/integrations/apply-pending-config", "",
		map[string]string{"hostId": hostID})
	return r.WithContext(context.WithValue(r.Context(), middleware.UserIDKey, userID))
}

// registryWithAgent reports the host's agent as connected without a live
// WebSocket: IsConnected is true, SendJSON fails with ErrNotConnected.
func registryWithAgent(hostID string) *agentregistry.Registry {
	reg := agentregistry.New()
	reg.Register("api-"+hostID, false)
	return reg
}

func createPendingDocker(t *testing.T, h *HostsHandler, d *database.DB, hostID string, enabled bool) {
	t.Helper()
	w := httptest.NewRecorder()
	h.ToggleIntegration(w, toggleRequest(hostID, "docker", `{"enabled":`+strconv.FormatBool(enabled)+`}`, "user-3"))
	if w.Code != http.StatusOK || pendingConfigRows(t, d, hostID) != 1 {
		t.Fatalf("setup: status=%d body=%s", w.Code, w.Body.String())
	}
}

func pendingDocker(t *testing.T, d *database.DB, hostID string) *bool {
	t.Helper()
	var v *bool
	if err := d.RawQueryRow(context.Background(), `SELECT docker_enabled FROM host_pending_config WHERE host_id = $1`, hostID).Scan(&v); err != nil {
		t.Fatalf("pending row: %v", err)
	}
	return v
}

func TestClaimPendingConfigIsAtomic(t *testing.T) {
	d := newHandlerTestDB(t)
	hostID := insertSolvedTestHost(t, d, "web01")
	h := hostsAuditTestHandler(d)
	createPendingDocker(t, h, d, hostID, true)
	ctx := context.Background()
	first, err := h.pendingConfig.ClaimPendingConfig(ctx, hostID)
	if err != nil || first == nil || first.DockerEnabled == nil || !*first.DockerEnabled {
		t.Fatalf("first claim=%+v err=%v, want row with docker_enabled=true", first, err)
	}
	second, err := h.pendingConfig.ClaimPendingConfig(ctx, hostID)
	if err != nil || second != nil {
		t.Fatalf("second claim=%+v err=%v, want nil, nil", second, err)
	}
	if got := pendingConfigRows(t, d, hostID); got != 0 {
		t.Fatalf("pending config rows=%d, want 0", got)
	}
}

func TestClaimPendingConfigConcurrentSingleWinner(t *testing.T) {
	d := newHandlerTestDB(t)
	hostID := insertSolvedTestHost(t, d, "web01")
	h := hostsAuditTestHandler(d)
	createPendingDocker(t, h, d, hostID, true)
	const n = 8
	var wg sync.WaitGroup
	var mu sync.Mutex
	winners, errs := 0, 0
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			pc, err := h.pendingConfig.ClaimPendingConfig(context.Background(), hostID)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errs++
			} else if pc != nil {
				winners++
			}
		}()
	}
	wg.Wait()
	if winners != 1 || errs != 0 {
		t.Fatalf("winners=%d errs=%d, want exactly one winner and no errors", winners, errs)
	}
}

func TestApplyAfterDiscardReturnsNoPending(t *testing.T) {
	d := newHandlerTestDB(t)
	hostID := insertSolvedTestHost(t, d, "web01")
	h := hostsAuditTestHandler(d)
	h.registry = registryWithAgent(hostID)
	createPendingDocker(t, h, d, hostID, true)
	w := httptest.NewRecorder()
	h.DiscardPendingConfig(w, discardRequest(hostID, "user-3"))
	if w.Code != http.StatusOK {
		t.Fatalf("discard status=%d body=%s", w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	h.ApplyPendingConfig(w, applyRequest(hostID, "user-3"))
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "No pending configuration changes") {
		t.Fatalf("apply status=%d body=%s, want 400 no pending", w.Code, w.Body.String())
	}
	if got := auditRows(t, d, "integration_config_applied", hostID); got != 0 {
		t.Fatalf("applied audit rows=%d, want 0", got)
	}
}

// discardDuringAudit simulates a concurrent Discard that wins between Apply's
// read of the pending row and its claim: the audit write (the only step in
// between that touches h.db) deletes the row first.
type discardDuringAudit struct {
	d      *database.DB
	hostID string
}

func (p discardDuringAudit) DB(ctx context.Context) *database.DB {
	_, _ = p.d.Exec(ctx, `DELETE FROM host_pending_config WHERE host_id = $1`, p.hostID)
	return p.d
}

func TestApplyReturns409WhenClaimLoses(t *testing.T) {
	d := newHandlerTestDB(t)
	hostID := insertSolvedTestHost(t, d, "web01")
	h := hostsAuditTestHandler(d)
	h.registry = registryWithAgent(hostID)
	createPendingDocker(t, h, d, hostID, true)
	h.db = discardDuringAudit{d: d, hostID: hostID}
	w := httptest.NewRecorder()
	h.ApplyPendingConfig(w, applyRequest(hostID, "user-3"))
	if w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), `"code":"pending_config_gone"`) ||
		!strings.Contains(w.Body.String(), "Pending configuration was already applied or discarded") {
		t.Fatalf("status=%d body=%s, want 409 pending_config_gone", w.Code, w.Body.String())
	}
	var docker bool
	if err := d.RawQueryRow(context.Background(), `SELECT docker_enabled FROM hosts WHERE id = $1`, hostID).Scan(&docker); err != nil || docker {
		t.Fatalf("hosts.docker_enabled=%v err=%v, want false (nothing applied)", docker, err)
	}
	if got := pendingConfigRows(t, d, hostID); got != 0 {
		t.Fatalf("pending config rows=%d, want 0", got)
	}
}

func TestApplyRestoresPendingWhenSendFails(t *testing.T) {
	d := newHandlerTestDB(t)
	hostID := insertSolvedTestHost(t, d, "web01")
	h := hostsAuditTestHandler(d)
	h.registry = registryWithAgent(hostID) // connected, but SendJSON fails
	createPendingDocker(t, h, d, hostID, true)
	w := httptest.NewRecorder()
	h.ApplyPendingConfig(w, applyRequest(hostID, "user-3"))
	if w.Code != http.StatusServiceUnavailable || !strings.Contains(w.Body.String(), "Failed to send config to agent") {
		t.Fatalf("status=%d body=%s, want 503 send failure", w.Code, w.Body.String())
	}
	if v := pendingDocker(t, d, hostID); v == nil || !*v {
		t.Fatalf("restored docker_enabled=%v, want true", v)
	}
}

func TestDiscardRestoresPendingWhenAuditFails(t *testing.T) {
	d := newHandlerTestDB(t)
	hostID := insertSolvedTestHost(t, d, "web01")
	h := hostsAuditTestHandler(d)
	createPendingDocker(t, h, d, hostID, true)
	h.db = fakeProvider{nil} // audit write fails, pending store stays real
	w := httptest.NewRecorder()
	h.DiscardPendingConfig(w, discardRequest(hostID, "user-3"))
	if w.Code != http.StatusInternalServerError || !strings.Contains(w.Body.String(), "Failed to write audit log") {
		t.Fatalf("status=%d body=%s, want 500 audit failure", w.Code, w.Body.String())
	}
	if v := pendingDocker(t, d, hostID); v == nil || !*v {
		t.Fatalf("pending docker_enabled=%v, want true (unchanged)", v)
	}
}
