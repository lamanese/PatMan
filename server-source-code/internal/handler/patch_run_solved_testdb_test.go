package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/PatchMon/PatchMon/server-source-code/internal/config"
	"github.com/PatchMon/PatchMon/server-source-code/internal/database"
	"github.com/PatchMon/PatchMon/server-source-code/internal/middleware"
	"github.com/PatchMon/PatchMon/server-source-code/internal/store"
	"github.com/google/uuid"
)

func insertSolvedTestHost(t *testing.T, d *database.DB, name string) string {
	t.Helper()
	id := uuid.NewString()
	if _, err := d.Exec(context.Background(), `INSERT INTO hosts (id, friendly_name, os_type, os_version, updated_at, api_id, api_key)
		VALUES ($1, $2, 'ubuntu', '22.04', NOW(), $3, $4)`, id, name, "api-"+id, "key-"+id); err != nil {
		t.Fatalf("insert host: %v", err)
	}
	return id
}

// insertSolvedTestRun inserts a non-dry-run patch run; pkgs nil = patch_all.
func insertSolvedTestRun(t *testing.T, d *database.DB, hostID, status string, pkgs []string, createdAt time.Time) string {
	t.Helper()
	id := uuid.NewString()
	patchType := "patch_all"
	var names []byte
	if pkgs != nil {
		patchType = "patch_package"
		names, _ = json.Marshal(pkgs)
	}
	if _, err := d.Exec(context.Background(), `INSERT INTO patch_runs (id, host_id, job_id, patch_type, package_names, status, shell_output, error_message, dry_run, created_at, updated_at, completed_at)
		VALUES ($1, $2, $3, $4, $5, $6, 'E: broken', 'exit 100', false, $7, $7, $7)`, id, hostID, "job-"+id, patchType, names, status, createdAt); err != nil {
		t.Fatalf("insert run: %v", err)
	}
	return id
}

func solvedTestHandler(d *database.DB) *PatchingHandler {
	p := fakeProvider{d}
	return &PatchingHandler{patchRuns: store.NewPatchRunsStore(p), hosts: store.NewHostsStore(p), db: p, log: discardLogger()}
}

func solvedRequest(method, path, body, runID, userID string) *http.Request {
	r := routedRequest(method, path, body, map[string]string{"id": runID})
	return r.WithContext(context.WithValue(r.Context(), middleware.UserIDKey, userID))
}

func runState(t *testing.T, d *database.DB, id string) (status string, note *string, solvedAt *time.Time, solvedBy, solvedByRun *string) {
	t.Helper()
	if err := d.RawQueryRow(context.Background(), `SELECT status, fork_solved_note, fork_solved_at, fork_solved_by, fork_solved_by_run_id FROM patch_runs WHERE id = $1`, id).
		Scan(&status, &note, &solvedAt, &solvedBy, &solvedByRun); err != nil {
		t.Fatal(err)
	}
	return
}

func TestSolveRunFromFailedRecordsUserNoteAndAudit(t *testing.T) {
	d := newHandlerTestDB(t)
	h := solvedTestHandler(d)
	host := insertSolvedTestHost(t, d, "web01")
	run := insertSolvedTestRun(t, d, host, "failed", nil, time.Now().Add(-time.Hour))
	w := httptest.NewRecorder()
	h.SolveRun(w, solvedRequest(http.MethodPost, "/api/v1/patching/runs/"+run+"/solve", `{"note":"fixed dpkg by hand"}`, run, "user-1"))
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	status, note, solvedAt, solvedBy, byRun := runState(t, d, run)
	if status != "solved" || note == nil || *note != "fixed dpkg by hand" || solvedAt == nil || solvedBy == nil || *solvedBy != "user-1" || byRun != nil {
		t.Fatalf("run after solve: status=%s note=%v at=%v by=%v byRun=%v", status, note, solvedAt, solvedBy, byRun)
	}
	var out, errMsg string
	if err := d.RawQueryRow(context.Background(), `SELECT shell_output, COALESCE(error_message,'') FROM patch_runs WHERE id = $1`, run).Scan(&out, &errMsg); err != nil || out != "E: broken" || errMsg != "exit 100" {
		t.Fatalf("output must survive solving: %q %q %v", out, errMsg, err)
	}
	var audits int
	if err := d.RawQueryRow(context.Background(), `SELECT count(*) FROM audit_logs WHERE event = 'patch_run_solved' AND user_id = 'user-1'`).Scan(&audits); err != nil || audits != 1 {
		t.Fatalf("audit rows=%d err=%v", audits, err)
	}
	var body map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if body["status"] != "solved" || body["fork_solved_note"] != "fixed dpkg by hand" {
		t.Fatalf("response must carry the solved run: %v", body)
	}
}

func TestSolveRunRejectsOtherStatuses(t *testing.T) {
	d := newHandlerTestDB(t)
	h := solvedTestHandler(d)
	host := insertSolvedTestHost(t, d, "web01")
	for _, st := range []string{"completed", "cancelled", "running", "solved"} {
		run := insertSolvedTestRun(t, d, host, st, nil, time.Now().Add(-time.Hour))
		w := httptest.NewRecorder()
		h.SolveRun(w, solvedRequest(http.MethodPost, "/api/v1/patching/runs/"+run+"/solve", `{}`, run, "user-1"))
		if w.Code != http.StatusConflict {
			t.Fatalf("%s: status %d, want 409", st, w.Code)
		}
		if got, _, _, _, _ := runState(t, d, run); got != st {
			t.Fatalf("%s must stay, got %s", st, got)
		}
	}
}

func TestReopenRunKeepsNote(t *testing.T) {
	d := newHandlerTestDB(t)
	h := solvedTestHandler(d)
	host := insertSolvedTestHost(t, d, "web01")
	run := insertSolvedTestRun(t, d, host, "failed", nil, time.Now().Add(-time.Hour))
	w := httptest.NewRecorder()
	h.SolveRun(w, solvedRequest(http.MethodPost, "/x", `{"note":"n1"}`, run, "user-1"))
	w = httptest.NewRecorder()
	h.ReopenRun(w, solvedRequest(http.MethodPost, "/x", ``, run, "user-2"))
	if w.Code != http.StatusOK {
		t.Fatalf("reopen: %d %s", w.Code, w.Body.String())
	}
	status, note, solvedAt, solvedBy, _ := runState(t, d, run)
	if status != "failed" || note == nil || *note != "n1" || solvedAt != nil || solvedBy != nil {
		t.Fatalf("after reopen: status=%s note=%v at=%v by=%v", status, note, solvedAt, solvedBy)
	}
	w = httptest.NewRecorder()
	h.ReopenRun(w, solvedRequest(http.MethodPost, "/x", ``, run, "user-2"))
	if w.Code != http.StatusConflict {
		t.Fatalf("reopen of a failed run must be 409, got %d", w.Code)
	}
}

func TestSolvedNoteUpdateOnlyOnSolvedRuns(t *testing.T) {
	d := newHandlerTestDB(t)
	h := solvedTestHandler(d)
	host := insertSolvedTestHost(t, d, "web01")
	failed := insertSolvedTestRun(t, d, host, "failed", nil, time.Now().Add(-time.Hour))
	solved := insertSolvedTestRun(t, d, host, "solved", nil, time.Now().Add(-time.Hour))
	w := httptest.NewRecorder()
	h.UpdateSolvedNote(w, solvedRequest(http.MethodPatch, "/x", `{"note":"updated"}`, solved, "user-1"))
	if w.Code != http.StatusOK {
		t.Fatalf("note on solved: %d %s", w.Code, w.Body.String())
	}
	if _, note, _, _, _ := runState(t, d, solved); note == nil || *note != "updated" {
		t.Fatalf("note not stored: %v", note)
	}
	w = httptest.NewRecorder()
	h.UpdateSolvedNote(w, solvedRequest(http.MethodPatch, "/x", `{"note":"x"}`, failed, "user-1"))
	if w.Code != http.StatusConflict {
		t.Fatalf("note on failed must be 409, got %d", w.Code)
	}
	w = httptest.NewRecorder()
	h.UpdateSolvedNote(w, solvedRequest(http.MethodPatch, "/x", `{"note":"`+strings.Repeat("x", 4001)+`"}`, solved, "user-1"))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("over-long note must be 400, got %d", w.Code)
	}
}

func TestBulkSolveSkipsNonFailed(t *testing.T) {
	d := newHandlerTestDB(t)
	h := solvedTestHandler(d)
	host := insertSolvedTestHost(t, d, "web01")
	f1 := insertSolvedTestRun(t, d, host, "failed", nil, time.Now().Add(-2*time.Hour))
	f2 := insertSolvedTestRun(t, d, host, "failed", []string{"openssl"}, time.Now().Add(-time.Hour))
	done := insertSolvedTestRun(t, d, host, "completed", nil, time.Now())
	body := `{"ids":["` + f1 + `","` + f2 + `","` + done + `","` + uuid.NewString() + `"],"note":"bulk"}`
	r := routedRequest(http.MethodPost, "/api/v1/patching/runs/bulk-solve", body, nil)
	r = r.WithContext(context.WithValue(r.Context(), middleware.UserIDKey, "user-1"))
	w := httptest.NewRecorder()
	h.BulkSolveRuns(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("bulk: %d %s", w.Code, w.Body.String())
	}
	var out struct {
		Solved  []string `json:"solved"`
		Skipped int      `json:"skipped"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	if len(out.Solved) != 2 || out.Skipped != 2 {
		t.Fatalf("solved=%v skipped=%d", out.Solved, out.Skipped)
	}
	if st, note, _, _, _ := runState(t, d, f2); st != "solved" || note == nil || *note != "bulk" {
		t.Fatalf("f2: %s %v", st, note)
	}
	if st, _, _, _, _ := runState(t, d, done); st != "completed" {
		t.Fatalf("completed run must stay: %s", st)
	}
}

// A completed real run solves the older failures of its host: patch_all all
// of them, patch_package only those with the same package selection.
func TestCompletedRunAutoSolvesOlderFailures(t *testing.T) {
	d := newHandlerTestDB(t)
	p := fakeProvider{d}
	runs := store.NewPatchRunsStore(p)
	a := insertSolvedTestHost(t, d, "a")
	b := insertSolvedTestHost(t, d, "b")
	base := time.Now().Add(-3 * time.Hour)
	aAll := insertSolvedTestRun(t, d, a, "failed", nil, base)
	aPkg := insertSolvedTestRun(t, d, a, "failed", []string{"openssl"}, base.Add(time.Minute))
	bAll := insertSolvedTestRun(t, d, b, "failed", nil, base)
	bPkgY := insertSolvedTestRun(t, d, b, "failed", []string{"y"}, base.Add(time.Minute))
	bPkgZ := insertSolvedTestRun(t, d, b, "failed", []string{"z"}, base.Add(2*time.Minute))
	aLater := insertSolvedTestRun(t, d, a, "failed", nil, time.Now().Add(time.Hour)) // newer than the completed run

	aDone := insertSolvedTestRun(t, d, a, "running", nil, time.Now())
	if err := runs.UpdateOutput(context.Background(), aDone, "ubuntu", "completed", "Setting up openssl", ""); err != nil {
		t.Fatal(err)
	}
	for id, want := range map[string]string{aAll: "solved", aPkg: "solved", bAll: "failed", bPkgY: "failed", aLater: "failed"} {
		if st, _, _, _, byRun := runState(t, d, id); st != want || (want == "solved" && (byRun == nil || *byRun != aDone)) {
			t.Fatalf("after patch_all on a: run %s status=%s byRun=%v, want %s", id, st, byRun, want)
		}
	}
	if _, note, _, by, _ := runState(t, d, aAll); by != nil || note == nil || !strings.Contains(*note, "automatically") {
		t.Fatalf("auto-solved run must carry no user and an automatic note: by=%v note=%v", by, note)
	}

	bDoneY := insertSolvedTestRun(t, d, b, "running", []string{"y"}, time.Now())
	if err := runs.UpdateOutput(context.Background(), bDoneY, "ubuntu", "completed", "ok", ""); err != nil {
		t.Fatal(err)
	}
	for id, want := range map[string]string{bPkgY: "solved", bPkgZ: "failed", bAll: "failed"} {
		if st, _, _, _, _ := runState(t, d, id); st != want {
			t.Fatalf("after patch_package y on b: run %s status=%s, want %s", id, st, want)
		}
	}
}

func TestGetRunExposesSolvedInfo(t *testing.T) {
	d := newHandlerTestDB(t)
	h := solvedTestHandler(d)
	host := insertSolvedTestHost(t, d, "web01")
	run := insertSolvedTestRun(t, d, host, "failed", nil, time.Now().Add(-time.Hour))
	if _, err := d.Exec(context.Background(), `INSERT INTO users (id, username, email, password_hash, role, updated_at) VALUES ('user-9', 'ops', 'ops@example.com', 'x', 'admin', NOW())`); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	w := httptest.NewRecorder()
	h.SolveRun(w, solvedRequest(http.MethodPost, "/x", `{"note":"n"}`, run, "user-9"))
	w = httptest.NewRecorder()
	h.GetRun(w, solvedRequest(http.MethodGet, "/x", ``, run, "user-9"))
	var body map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if body["status"] != "solved" || body["fork_solved_note"] != "n" || body["fork_solved_by_username"] != "ops" || body["fork_solved_at"] == nil {
		t.Fatalf("run detail must expose the solved info: %v", body)
	}
}

func TestSolvedIsTerminalForStop(t *testing.T) {
	if !isTerminalPatchStatus("solved") {
		t.Fatal("solved must be terminal for StopRun")
	}
}

func TestAuditRunErrWritesRowAndReturnsErrorWithoutDB(t *testing.T) {
	d := newHandlerTestDB(t)
	h := solvedTestHandler(d)
	r := solvedRequest(http.MethodPost, "/x", "", "run-1", "user-9")
	if err := h.auditRunErr(r, "patch_run_triggered", map[string]interface{}{"patch_run_id": "run-1"}); err != nil {
		t.Fatalf("auditRunErr: %v", err)
	}
	var n int
	if err := d.RawQueryRow(context.Background(), `SELECT count(*) FROM audit_logs WHERE event='patch_run_triggered' AND user_id='user-9' AND details LIKE '%run-1%'`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("rows=%d err=%v", n, err)
	}
	h.db = nil
	if err := h.auditRunErr(r, "patch_run_triggered", nil); err == nil {
		t.Fatal("expected error without db")
	}
}

func triggerTestHandler(d *database.DB) *PatchingHandler {
	h := solvedTestHandler(d)
	h.patchPolicies = store.NewPatchPoliciesStore(fakeProvider{d})
	h.cfg = &config.Config{}
	return h
}

func insertTriggerTestUser(t *testing.T, d *database.DB) {
	t.Helper()
	if _, err := d.Exec(context.Background(), `INSERT INTO users (id, username, email, password_hash, role, updated_at) VALUES ('user-3', 'ops3', 'ops3@example.com', 'x', 'admin', NOW())`); err != nil {
		t.Fatal(err)
	}
}

func TestTriggerWritesAuditBeforeCreatingRun(t *testing.T) {
	d := newHandlerTestDB(t)
	h := triggerTestHandler(d)
	host := insertSolvedTestHost(t, d, "web01")
	insertTriggerTestUser(t, d)
	w := httptest.NewRecorder()
	h.Trigger(w, solvedRequest(http.MethodPost, "/x", `{"host_id":"`+host+`","patch_type":"patch_all","pending_approval":true}`, "", "user-3"))
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	var n int
	if err := d.RawQueryRow(context.Background(), `SELECT count(*) FROM audit_logs WHERE event='patch_run_triggered' AND user_id='user-3'
		AND details LIKE '%"host_id":"`+host+`"%' AND details LIKE '%"pending_approval":true%' AND details LIKE '%"host_name":"web01"%'`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("audit rows=%d err=%v", n, err)
	}
}

func TestTriggerRefusesWithoutAudit(t *testing.T) {
	d := newHandlerTestDB(t)
	h := triggerTestHandler(d)
	host := insertSolvedTestHost(t, d, "web01")
	insertTriggerTestUser(t, d)
	h.db = fakeProvider{nil}
	w := httptest.NewRecorder()
	h.Trigger(w, solvedRequest(http.MethodPost, "/x", `{"host_id":"`+host+`","patch_type":"patch_all","pending_approval":true}`, "", "user-3"))
	if w.Code != http.StatusInternalServerError || !strings.Contains(w.Body.String(), "Failed to write audit log") {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	var n int
	if err := d.RawQueryRow(context.Background(), `SELECT count(*) FROM patch_runs WHERE host_id = $1`, host).Scan(&n); err != nil || n != 0 {
		t.Fatalf("no run may exist without audit: rows=%d err=%v", n, err)
	}
}

func TestApproveRunRefusesWithoutAudit(t *testing.T) {
	d := newHandlerTestDB(t)
	h := triggerTestHandler(d)
	host := insertSolvedTestHost(t, d, "web01")
	run := insertSolvedTestRun(t, d, host, "pending_approval", nil, time.Now().Add(-time.Hour))
	h.db = fakeProvider{nil}
	w := httptest.NewRecorder()
	h.ApproveRun(w, solvedRequest(http.MethodPost, "/x", `{}`, run, "user-3"))
	if w.Code != http.StatusInternalServerError || !strings.Contains(w.Body.String(), "Failed to write audit log") {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	if st, _, _, _, _ := runState(t, d, run); st != "pending_approval" {
		t.Fatalf("validation run must stay pending_approval, got %s", st)
	}
	var n int
	if err := d.RawQueryRow(context.Background(), `SELECT count(*) FROM patch_runs WHERE host_id = $1`, host).Scan(&n); err != nil || n != 1 {
		t.Fatalf("no new run may exist without audit: rows=%d err=%v", n, err)
	}
}
