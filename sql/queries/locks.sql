-- name: CreateLock :one
INSERT INTO locks (
    name,
    created_by,
    description,
    lock_type,
    timeout
) VALUES (
    $1, -- name
    $2, -- created_by
    $3, -- description
    $4, -- lock_type: either timer or manual
    $5 -- timeout: null only if manual
)
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

-- name: DeleteLockById :exec
DELETE FROM locks 
WHERE id=$1;