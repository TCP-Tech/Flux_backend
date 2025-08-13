-- name: CreateLock :one
INSERT INTO locks (
    name,
    created_by,
    description,
    lock_type,
    timeout,
    group_id
) VALUES (
    $1, -- name
    $2, -- created_by
    $3, -- description
    $4, -- lock_type: either timer or manual
    $5, -- timeout: null only if manual
    $6 -- group_id: to manage a group of locks
)
RETURNING *;

-- name: GetLockById :one
SELECT * FROM locks WHERE id=sqlc.arg('group_d');

-- name: GetLockGroupTimeout :one
SELECT timeout
FROM locks
WHERE group_id = $1
LIMIT 1;

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
    ) AND
    (
        sqlc.narg('group_id')::uuid IS NULL OR
        sqlc.narg('group_id')::uuid = group_id
    )
LIMIT sqlc.arg('limit')
OFFSET sqlc.arg('offset');

-- name: DeleteLockById :exec
DELETE FROM locks 
WHERE id=$1;