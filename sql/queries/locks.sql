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
    $5  -- timeout: null only if manual
)
RETURNING *;

-- name: GetLockById :one
SELECT * FROM locks WHERE id=sqlc.arg('group_d');

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
    name ILIKE '%' || sqlc.arg('lock_name')::text || '%'
    AND (
        sqlc.narg('created_by')::uuid IS NULL OR
        sqlc.narg('created_by')::uuid = created_by
    )
LIMIT sqlc.arg('limit')
OFFSET sqlc.arg('offset');

-- name: DeleteLockById :exec
DELETE FROM locks 
WHERE id=$1;