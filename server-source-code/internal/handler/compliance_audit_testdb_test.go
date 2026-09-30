package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/PatchMon/PatchMon/server-source-code/internal/agentregistry"
	"github.com/PatchMon/PatchMon/server-source-code/internal/database"
	"github.com/PatchMon/PatchMon/server-source-code/internal/middleware"
	"github.com/PatchMon/PatchMon/server-source-code/internal/store"
	"github.com/hibiken/asynq"
)

func complianceAuditTestHandler(d *database.DB) *ComplianceHandler {
	h := &ComplianceHandler{hostsStore: store.NewHostsStore(fakeProvider{d}), registry: agentregistry.New()}
	h.SetDB(fakeProvider{d})
	return h
}

func complianceRequest(path, body, hostID, userID string) *http.Request {
	r := routedRequest(http.MethodPost, path, body, map[string]string{"hostId": hostID})
	return r.WithContext(context.WithValue(r.Context(), middleware.UserIDKey, userID))
}

func complianceAuditRows(t *testing.T, d *database.DB, event, userID string) int {
	t.Helper()
	var n int
	if err := d.RawQueryRow(context.Background(), `SELECT count(*) FROM audit_logs WHERE event = $1 AND user_id = $2`, event, userID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// The audit row records the request, not the outcome: with the agent offline
// the remediation fails, but the attempt is on record.
func TestRemediateRuleWritesAuditEvenWhenAgentOffline(t *testing.T) {
	d := newHandlerTestDB(t)
	hostID := insertSolvedTestHost(t, d, "web01")
	h := complianceAuditTestHandler(d)
	w := httptest.NewRecorder()
	h.RemediateRule(w, complianceRequest("/api/v1/compliance/remediate/"+hostID, `{"rule_id":"xccdf_org.ssgproject.content_rule_x"}`, hostID, "user-5"))
	if w.Code == http.StatusOK {
		t.Fatalf("agent is offline, expected non-200, got %d", w.Code)
	}
	var n int
	if err := d.RawQueryRow(context.Background(), `SELECT count(*) FROM audit_logs WHERE event = 'compliance_remediation_requested' AND user_id = 'user-5' AND success
		AND details LIKE '%"rule_id":"xccdf_org.ssgproject.content_rule_x"%' AND details LIKE '%"host_id":"`+hostID+`"%' AND details LIKE '%"host_name":"web01"%'`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("audit rows=%d err=%v", n, err)
	}
}

func TestRemediateRuleFailsClosedWithoutAudit(t *testing.T) {
	d := newHandlerTestDB(t)
	hostID := insertSolvedTestHost(t, d, "web01")
	h := complianceAuditTestHandler(d)
	h.db = nil
	w := httptest.NewRecorder()
	h.RemediateRule(w, complianceRequest("/api/v1/compliance/remediate/"+hostID, `{"rule_id":"xccdf_org.ssgproject.content_rule_x"}`, hostID, "user-5"))
	if w.Code != http.StatusInternalServerError || !strings.Contains(w.Body.String(), "Failed to write audit log") {
		t.Fatalf("status=%d body=%s, want 500 audit failure", w.Code, w.Body.String())
	}
}

// TriggerScan audits only scans that may remediate. The queue client is
// closed, so the enqueue fails right after the audit decision was made.
func TestTriggerScanAuditsOnlyRemediatingScans(t *testing.T) {
	d := newHandlerTestDB(t)
	hostID := insertSolvedTestHost(t, d, "web01")
	if _, err := d.Exec(context.Background(), `UPDATE hosts SET compliance_enabled = true WHERE id = $1`, hostID); err != nil {
		t.Fatal(err)
	}
	h := complianceAuditTestHandler(d)
	h.queueClient = asynq.NewClient(asynq.RedisClientOpt{Addr: "127.0.0.1:1"})
	if err := h.queueClient.Close(); err != nil {
		t.Fatal(err)
	}

	path := "/api/v1/compliance/trigger/" + hostID
	h.TriggerScan(httptest.NewRecorder(), complianceRequest(path, `{"profile_type":"openscap"}`, hostID, "user-6"))
	if got := complianceAuditRows(t, d, "compliance_scan_triggered", "user-6"); got != 0 {
		t.Fatalf("scan without remediation audited: rows=%d", got)
	}

	h.TriggerScan(httptest.NewRecorder(), complianceRequest(path, `{"profile_type":"openscap","enable_remediation":true}`, hostID, "user-6"))
	var n int
	if err := d.RawQueryRow(context.Background(), `SELECT count(*) FROM audit_logs WHERE event = 'compliance_scan_triggered' AND user_id = 'user-6'
		AND details LIKE '%"enable_remediation":true%' AND details LIKE '%"profile":"openscap"%' AND details LIKE '%"host_name":"web01"%' AND details LIKE '%"host_id":"`+hostID+`"%'`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("audit rows=%d err=%v", n, err)
	}

	h.db = nil
	w := httptest.NewRecorder()
	h.TriggerScan(w, complianceRequest(path, `{"enable_remediation":true}`, hostID, "user-6"))
	if w.Code != http.StatusInternalServerError || !strings.Contains(w.Body.String(), "Failed to write audit log") {
		t.Fatalf("status=%d body=%s, want 500 audit failure", w.Code, w.Body.String())
	}
}
