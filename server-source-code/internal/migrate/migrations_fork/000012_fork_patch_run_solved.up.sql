-- Solved patch runs: an operator (or a later successful run on the same
-- host) marks a failed run as solved. The run keeps its output and error;
-- only status = 'solved' plus these fork columns are added.
ALTER TABLE patch_runs
    ADD COLUMN IF NOT EXISTS fork_solved_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS fork_solved_by TEXT,
    ADD COLUMN IF NOT EXISTS fork_solved_note TEXT,
    ADD COLUMN IF NOT EXISTS fork_solved_by_run_id TEXT;
