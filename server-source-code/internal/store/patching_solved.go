package store

import (
	"context"

	"github.com/PatchMon/PatchMon/server-source-code/internal/db"
)

// Fork: "solved" patch runs. A failed run keeps its output and error; only
// the status and the fork_solved_* columns change.

// Solve marks a failed run as solved. It returns false when the run is not
// in status failed (nothing changed).
func (s *PatchRunsStore) Solve(ctx context.Context, id string, userID *string, note *string) (bool, error) {
	n, err := s.db.DB(ctx).Queries.ForkSolvePatchRun(ctx, db.ForkSolvePatchRunParams{ID: id, UserID: userID, Note: note})
	return n == 1, err
}

// BulkSolve marks every failed run among ids as solved and returns the ids
// that changed; ids in another status or unknown ids are skipped.
func (s *PatchRunsStore) BulkSolve(ctx context.Context, ids []string, userID *string, note *string) ([]string, error) {
	return s.db.DB(ctx).Queries.ForkBulkSolvePatchRuns(ctx, db.ForkBulkSolvePatchRunsParams{Ids: ids, UserID: userID, Note: note})
}

// Reopen puts a solved run back to failed; the note is kept.
func (s *PatchRunsStore) Reopen(ctx context.Context, id string) (bool, error) {
	n, err := s.db.DB(ctx).Queries.ForkReopenPatchRun(ctx, id)
	return n == 1, err
}

// UpdateSolvedNote changes the note of a solved run.
func (s *PatchRunsStore) UpdateSolvedNote(ctx context.Context, id string, note *string) (bool, error) {
	n, err := s.db.DB(ctx).Queries.ForkUpdatePatchRunSolvedNote(ctx, db.ForkUpdatePatchRunSolvedNoteParams{ID: id, Note: note})
	return n == 1, err
}

// SolvedByUsername resolves who solved the run (nil for automatic solves or
// deleted users).
func (s *PatchRunsStore) SolvedByUsername(ctx context.Context, id string) (*string, error) {
	return s.db.DB(ctx).Queries.ForkGetPatchRunSolvedByUsername(ctx, id)
}

// AutoSolveAfterCompleted solves the older failed runs of the completed
// run's host (see ForkAutoSolvePatchRuns) and returns their ids.
func (s *PatchRunsStore) AutoSolveAfterCompleted(ctx context.Context, completedRunID string) ([]string, error) {
	return s.db.DB(ctx).Queries.ForkAutoSolvePatchRuns(ctx, completedRunID)
}
