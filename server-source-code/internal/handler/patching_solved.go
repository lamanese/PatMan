package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/PatchMon/PatchMon/server-source-code/internal/database"
	"github.com/PatchMon/PatchMon/server-source-code/internal/db"
	"github.com/PatchMon/PatchMon/server-source-code/internal/middleware"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// Fork: mark failed patch runs as solved (and back). The run's output, error
// and the failure help in the UI stay untouched; only the status and the
// fork_solved_* columns change. Every change is audited.

// maxSolvedNoteLength bounds the free-text note.
const maxSolvedNoteLength = 4000

// SetDB gives the handler a database provider for audit entries.
func (h *PatchingHandler) SetDB(p database.DBProvider) { h.db = p }

type solvedNoteBody struct {
	Note string `json:"note"`
}

// solvedNote reads and validates the optional note; ok=false means a 400 was written.
func solvedNote(w http.ResponseWriter, r *http.Request) (*string, bool) {
	var body solvedNoteBody
	if r.Body != nil && r.ContentLength != 0 {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			JSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid JSON"})
			return nil, false
		}
	}
	note := strings.TrimSpace(body.Note)
	if len(note) > maxSolvedNoteLength {
		JSON(w, http.StatusBadRequest, map[string]string{"error": "Note too long (max 4000 characters)"})
		return nil, false
	}
	if note == "" {
		return nil, true
	}
	return &note, true
}

func userIDFromContext(r *http.Request) *string {
	if uid, _ := r.Context().Value(middleware.UserIDKey).(string); uid != "" {
		return &uid
	}
	return nil
}

// runDetailResponse is the run detail plus who solved it.
func (h *PatchingHandler) runDetailResponse(r *http.Request, id string) (map[string]interface{}, bool) {
	run, err := h.patchRuns.GetByID(r.Context(), id)
	if err != nil || run == nil {
		return nil, false
	}
	resp := patchRunToResponse(run)
	if run.Status == "solved" {
		if name, err := h.patchRuns.SolvedByUsername(r.Context(), id); err == nil {
			resp["fork_solved_by_username"] = name
		}
	}
	return resp, true
}

// SolveRun handles POST /patching/runs/{id}/solve (body: {note}).
func (h *PatchingHandler) SolveRun(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !isValidPatchUUID(id) {
		JSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid run ID"})
		return
	}
	note, ok := solvedNote(w, r)
	if !ok {
		return
	}
	run, err := h.patchRuns.GetByID(r.Context(), id)
	if err != nil || run == nil {
		JSON(w, http.StatusNotFound, map[string]string{"error": "Patch run not found"})
		return
	}
	if run.Status != "failed" {
		JSON(w, http.StatusConflict, map[string]string{"error": "Only failed runs can be marked as solved"})
		return
	}
	changed, err := h.patchRuns.Solve(r.Context(), id, userIDFromContext(r), note)
	if err != nil {
		h.log.Error("patching: solve run failed", "patch_run_id", id, "error", err)
		JSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to mark run as solved"})
		return
	}
	if !changed {
		JSON(w, http.StatusConflict, map[string]string{"error": "Only failed runs can be marked as solved"})
		return
	}
	h.auditRun(r, "patch_run_solved", map[string]interface{}{"patch_run_id": id, "host_id": run.HostID, "note_set": note != nil})
	resp, _ := h.runDetailResponse(r, id)
	JSON(w, http.StatusOK, resp)
}

// ReopenRun handles POST /patching/runs/{id}/reopen: solved -> failed, note kept.
func (h *PatchingHandler) ReopenRun(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !isValidPatchUUID(id) {
		JSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid run ID"})
		return
	}
	run, err := h.patchRuns.GetByID(r.Context(), id)
	if err != nil || run == nil {
		JSON(w, http.StatusNotFound, map[string]string{"error": "Patch run not found"})
		return
	}
	if run.Status != "solved" {
		JSON(w, http.StatusConflict, map[string]string{"error": "Only solved runs can be reopened"})
		return
	}
	changed, err := h.patchRuns.Reopen(r.Context(), id)
	if err != nil {
		h.log.Error("patching: reopen run failed", "patch_run_id", id, "error", err)
		JSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to reopen run"})
		return
	}
	if !changed {
		JSON(w, http.StatusConflict, map[string]string{"error": "Only solved runs can be reopened"})
		return
	}
	h.auditRun(r, "patch_run_reopened", map[string]interface{}{"patch_run_id": id, "host_id": run.HostID})
	resp, _ := h.runDetailResponse(r, id)
	JSON(w, http.StatusOK, resp)
}

// UpdateSolvedNote handles PATCH /patching/runs/{id}/solved-note (body: {note}).
func (h *PatchingHandler) UpdateSolvedNote(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !isValidPatchUUID(id) {
		JSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid run ID"})
		return
	}
	note, ok := solvedNote(w, r)
	if !ok {
		return
	}
	run, err := h.patchRuns.GetByID(r.Context(), id)
	if err != nil || run == nil {
		JSON(w, http.StatusNotFound, map[string]string{"error": "Patch run not found"})
		return
	}
	if run.Status != "solved" {
		JSON(w, http.StatusConflict, map[string]string{"error": "Only solved runs carry a note"})
		return
	}
	changed, err := h.patchRuns.UpdateSolvedNote(r.Context(), id, note)
	if err != nil {
		h.log.Error("patching: update solved note failed", "patch_run_id", id, "error", err)
		JSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to update note"})
		return
	}
	if !changed {
		JSON(w, http.StatusConflict, map[string]string{"error": "Only solved runs carry a note"})
		return
	}
	h.auditRun(r, "patch_run_solved_note_updated", map[string]interface{}{"patch_run_id": id, "host_id": run.HostID})
	resp, _ := h.runDetailResponse(r, id)
	JSON(w, http.StatusOK, resp)
}

// BulkSolveRuns handles POST /patching/runs/bulk-solve (body: {ids, note}).
// Runs that are not failed (or unknown) are skipped, not an error.
func (h *PatchingHandler) BulkSolveRuns(w http.ResponseWriter, r *http.Request) {
	var body struct {
		IDs  []string `json:"ids"`
		Note string   `json:"note"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		JSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid JSON"})
		return
	}
	if len(body.IDs) == 0 || len(body.IDs) > 500 {
		JSON(w, http.StatusBadRequest, map[string]string{"error": "ids must contain 1 to 500 run IDs"})
		return
	}
	for _, id := range body.IDs {
		if !isValidPatchUUID(id) {
			JSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid run ID: " + id})
			return
		}
	}
	noteText := strings.TrimSpace(body.Note)
	if len(noteText) > maxSolvedNoteLength {
		JSON(w, http.StatusBadRequest, map[string]string{"error": "Note too long (max 4000 characters)"})
		return
	}
	var note *string
	if noteText != "" {
		note = &noteText
	}
	solved, err := h.patchRuns.BulkSolve(r.Context(), body.IDs, userIDFromContext(r), note)
	if err != nil {
		h.log.Error("patching: bulk solve failed", "error", err)
		JSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to mark runs as solved"})
		return
	}
	if solved == nil {
		solved = []string{}
	}
	h.auditRun(r, "patch_runs_bulk_solved", map[string]interface{}{"requested": len(body.IDs), "solved": len(solved), "patch_run_ids": solved, "note_set": note != nil})
	JSON(w, http.StatusOK, map[string]interface{}{"solved": solved, "skipped": len(body.IDs) - len(solved)})
}

// auditRun writes a best-effort audit_logs entry (the status change itself
// is not destructive, so a failed audit write is logged, not fatal).
func (h *PatchingHandler) auditRun(r *http.Request, event string, detail map[string]interface{}) {
	if h.db == nil {
		return
	}
	ctx := r.Context()
	d := h.db.DB(ctx)
	if d == nil {
		return
	}
	var requestID *string
	if rid, _ := ctx.Value(middleware.RequestIDKey).(string); rid != "" {
		requestID = &rid
	}
	ip := clientIPFromRequest(r)
	ua := r.UserAgent()
	var details *string
	if b, err := json.Marshal(detail); err == nil {
		s := string(b)
		details = &s
	}
	if err := d.Queries.InsertAuditLog(ctx, db.InsertAuditLogParams{
		ID: uuid.New().String(), Event: event, UserID: userIDFromContext(r), IpAddress: &ip, UserAgent: &ua, RequestID: requestID, Details: details, Success: true,
	}); err != nil {
		h.log.Warn("patching: audit write failed", "event", event, "error", err)
	}
}
