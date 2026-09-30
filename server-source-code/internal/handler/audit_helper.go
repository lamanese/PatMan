package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"unicode/utf8"

	"github.com/PatchMon/PatchMon/server-source-code/internal/database"
	"github.com/PatchMon/PatchMon/server-source-code/internal/db"
	"github.com/PatchMon/PatchMon/server-source-code/internal/middleware"
	"github.com/google/uuid"
)

// insertAudit writes one audit_logs row. ip/ua may be empty (WebSocket
// teardown has no request any more). Callers decide whether a failure is
// fatal for the action (fail-closed) or only logged (best effort).
func insertAudit(ctx context.Context, d *database.DB, event string, userID *string, ip, ua string, requestID *string, detail map[string]interface{}) error {
	if d == nil {
		return errors.New("audit: no database")
	}
	var ipPtr, uaPtr, details *string
	if ip != "" {
		ipPtr = &ip
	}
	if ua != "" {
		uaPtr = &ua
	}
	if detail != nil {
		if b, err := json.Marshal(detail); err == nil {
			s := string(b)
			details = &s
		}
	}
	return d.Queries.InsertAuditLog(ctx, db.InsertAuditLogParams{
		ID: uuid.New().String(), Event: event, UserID: userID, IpAddress: ipPtr,
		UserAgent: uaPtr, RequestID: requestID, Details: details, Success: true,
	})
}

// auditFromRequest fills ip, user agent, user id and request id from r and
// calls insertAudit. Handlers use it for HTTP-triggered events.
func auditFromRequest(r *http.Request, d *database.DB, event string, detail map[string]interface{}) error {
	ctx := r.Context()
	var userID *string
	if uid, _ := ctx.Value(middleware.UserIDKey).(string); uid != "" {
		userID = &uid
	}
	var requestID *string
	if rid, _ := ctx.Value(middleware.RequestIDKey).(string); rid != "" {
		requestID = &rid
	}
	return insertAudit(ctx, d, event, userID, clientIPFromRequest(r), r.UserAgent(), requestID, detail)
}

// auditTextMaxRunes caps free-form client strings stored in audit details.
const auditTextMaxRunes = 256

// auditText truncates s to auditTextMaxRunes runes and appends "…" when it
// was cut, so a client cannot bloat audit rows with arbitrary input.
func auditText(s string) string {
	if utf8.RuneCountInString(s) <= auditTextMaxRunes {
		return s
	}
	return string([]rune(s)[:auditTextMaxRunes]) + "…"
}
