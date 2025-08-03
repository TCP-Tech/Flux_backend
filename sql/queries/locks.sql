-- name: CreateLock :one
INSERT INTO locks (timeout, name, created_by, description) VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetLockById :one
SELECT * FROM locks WHERE id=$1;

-- name: UpdateLockDetails :one
UPDATE locks
SET
    name = $2,
    timeout = $3,
    description = $4
WHERE
    id = $1
RETURNING *;

-- name: GetLocksByFilter :many
SELECT * FROM locks
WHERE
    name ILIKE $1
    AND (
        sqlc.narg('created_by')::uuid IS NULL OR
        sqlc.narg('created_by')::uuid = created_by
    );