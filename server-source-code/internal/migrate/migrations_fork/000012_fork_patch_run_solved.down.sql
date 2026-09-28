-- Older images do not know status 'solved': fold those runs back to failed
-- before dropping the columns.
UPDATE patch_runs SET status = 'failed', updated_at = NOW() WHERE status = 'solved';
ALTER TABLE patch_runs
    DROP COLUMN IF EXISTS fork_solved_by_run_id,
    DROP COLUMN IF EXISTS fork_solved_note,
    DROP COLUMN IF EXISTS fork_solved_by,
    DROP COLUMN IF EXISTS fork_solved_at;
