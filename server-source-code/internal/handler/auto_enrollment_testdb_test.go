package handler

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/PatchMon/PatchMon/server-source-code/internal/config"
	"github.com/PatchMon/PatchMon/server-source-code/internal/database"
	"github.com/PatchMon/PatchMon/server-source-code/internal/db"
	"github.com/PatchMon/PatchMon/server-source-code/internal/store"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/crypto/bcrypt"
)

const testEnrollSecret = "s3cret-for-tests"

func discardLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func newEnrollmentHandler(t *testing.T, d *database.DB) (*AutoEnrollmentHandler, string) {
	t.Helper()
	p := fakeProvider{d}
	hash, err := bcrypt.GenerateFromPassword([]byte(testEnrollSecret), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	key := "patchmon_ae_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:16]
	tokens := store.NewAutoEnrollmentStore(p)
	if err := tokens.Create(context.Background(), db.CreateAutoEnrollmentTokenParams{
		ID: uuid.NewString(), TokenName: "test", TokenKey: key, TokenSecret: string(hash), IsActive: true,
		AllowedIpRanges: []string{}, MaxHostsPerDay: 100, Metadata: []byte("{}"), Scopes: []byte("[]"),
		UpdatedAt: pgtype.Timestamp{Time: time.Now(), Valid: true},
	}); err != nil {
		t.Fatalf("create token: %v", err)
	}
	h := NewAutoEnrollmentHandler(tokens, store.NewHostGroupsStore(p), store.NewHostsStore(p), store.NewSettingsStore(p), discardLogger(), &config.Config{})
	return h, key
}

func enroll(t *testing.T, h *AutoEnrollmentHandler, key, body string) (int, map[string]any) {
	t.Helper()
	r := httptest.NewRequest(http.MethodPost, "/api/v1/auto-enrollment/enroll", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-Auto-Enrollment-Key", key)
	r.Header.Set("X-Auto-Enrollment-Secret", testEnrollSecret)
	w := httptest.NewRecorder()
	h.Enroll(w, r)
	var out map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("status %d, body not JSON: %s", w.Code, w.Body.String())
	}
	return w.Code, out
}

func hostField(out map[string]any, field string) string {
	host, _ := out["host"].(map[string]any)
	v, _ := host[field].(string)
	return v
}

// Two customers may both have a "web01": the second enrollment keeps working
// and gets a numbered name instead of a silent duplicate.
func TestEnrollSuffixesDuplicateFriendlyName(t *testing.T) {
	d := newHandlerTestDB(t)
	h, key := newEnrollmentHandler(t, d)
	code, out := enroll(t, h, key, `{"friendly_name":"web01","machine_id":"m-1"}`)
	if code != http.StatusCreated || hostField(out, "friendly_name") != "web01" {
		t.Fatalf("first: %d %v", code, out)
	}
	code, out = enroll(t, h, key, `{"friendly_name":"web01","machine_id":"m-2"}`)
	if code != http.StatusCreated || hostField(out, "friendly_name") != "web01-2" {
		t.Fatalf("second: %d %v", code, out)
	}
	code, out = enroll(t, h, key, `{"friendly_name":"web01"}`)
	if code != http.StatusCreated || hostField(out, "friendly_name") != "web01-3" {
		t.Fatalf("third (no machine id): %d %v", code, out)
	}
}

// The same machine enrolling twice must not create a second host: the
// caller gets 409 and the existing host, so a reinstall is done on purpose
// (delete the host first) instead of by accident.
func TestEnrollRejectsKnownMachineID(t *testing.T) {
	d := newHandlerTestDB(t)
	h, key := newEnrollmentHandler(t, d)
	code, first := enroll(t, h, key, `{"friendly_name":"web01","machine_id":"same-machine"}`)
	if code != http.StatusCreated {
		t.Fatalf("first: %d %v", code, first)
	}
	code, out := enroll(t, h, key, `{"friendly_name":"web01-renamed","machine_id":"same-machine"}`)
	if code != http.StatusConflict {
		t.Fatalf("second enrollment of the same machine: %d %v", code, out)
	}
	if hostField(out, "id") != hostField(first, "id") || hostField(out, "friendly_name") != "web01" {
		t.Fatalf("409 must name the existing host: %v", out)
	}
	var n int
	if err := d.RawQueryRow(context.Background(), `SELECT count(*) FROM hosts`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("hosts=%d err=%v, want exactly one host", n, err)
	}
}

// The Windows variant serves a PowerShell script with the same token
// injected as $env: variables, so "irm ... | iex" on the host works.
func TestServeScriptWindowsVariant(t *testing.T) {
	d := newHandlerTestDB(t)
	h, key := newEnrollmentHandler(t, d)
	r := httptest.NewRequest(http.MethodGet, "/api/v1/auto-enrollment/script?type=direct-host-windows&token_key="+key+"&token_secret="+testEnrollSecret, nil)
	w := httptest.NewRecorder()
	h.ServeScript(w, r)
	body := w.Body.String()
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, body)
	}
	if !strings.HasPrefix(body, "$env:PATCHMON_URL = \"") || !strings.Contains(body, "$env:AUTO_ENROLLMENT_KEY = \""+key+"\"") {
		t.Fatalf("PowerShell env block missing:\n%s", body[:min(400, len(body))])
	}
	if !strings.Contains(w.Header().Get("Content-Disposition"), ".ps1") {
		t.Fatalf("content-disposition %q", w.Header().Get("Content-Disposition"))
	}
	if !strings.Contains(body, "X-Auto-Enrollment-Key") || !strings.Contains(body, "os=windows") {
		t.Fatal("script must enroll with the token headers and fetch the Windows installer")
	}
	if strings.Contains(body, "#!/bin/sh") {
		t.Fatal("no shell shebang in a PowerShell script")
	}
}
