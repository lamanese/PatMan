-- name: ForkClaimPendingConfig :one
-- Atomically removes and returns a host's pending integration config.
-- Exactly one concurrent caller gets the row; the others get no rows.
DELETE FROM host_pending_config
WHERE host_id = $1
RETURNING *;
