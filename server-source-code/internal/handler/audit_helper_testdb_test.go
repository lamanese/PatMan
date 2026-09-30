package handler

import (
	"context"
	"testing"
)

func TestInsertAuditWritesRow(t *testing.T) {
	d := newHandlerTestDB(t)
	uid := "u1"
	if err := insertAudit(context.Background(), d, "ssh_session_closed", &uid, "", "", nil, map[string]interface{}{"host_id": "h1", "duration_s": 12}); err != nil {
		t.Fatalf("insertAudit: %v", err)
	}
	var n int
	if err := d.RawQueryRow(context.Background(), `SELECT count(*) FROM audit_logs WHERE event = 'ssh_session_closed' AND user_id = 'u1'
		AND ip_address IS NULL AND user_agent IS NULL AND details LIKE '%"host_id":"h1"%'`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("rows=%d err=%v", n, err)
	}
}

func TestInsertAuditNilDB(t *testing.T) {
	if err := insertAudit(context.Background(), nil, "ssh_session_closed", nil, "", "", nil, nil); err == nil {
		t.Fatal("expected error for nil db")
	}
}
