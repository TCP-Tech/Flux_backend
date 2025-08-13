-- name: CreateUser :one
INSERT INTO users (user_name, roll_no, password_hash, first_name, last_name, email)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetUserByUserName :one
SELECT * FROM users WHERE user_name = $1;

-- name: GetUserByRollNumber :one
SELECT * FROM users WHERE roll_no = $1;

-- name: GetUsersCountByUserName :one
SELECT COUNT(*) FROM users WHERE user_name = $1;

-- name: ResetPassword :exec
UPDATE users SET password_hash = $2 WHERE user_name = $1;

-- name: GetUserById :one
SELECT * FROM users WHERE id = $1;

-- name: IsUserIDValid :one
SELECT EXISTS(SELECT id from users WHERE id=$1);

-- name: GetUserIDByUserName :one
SELECT id from users WHERE user_name=$1;

-- name: GetUsersByFilters :many
SELECT id, user_name, roll_no FROM users
WHERE 
    (
        sqlc.narg('user_ids')::uuid[] IS NULL OR
        cardinality(sqlc.narg('user_ids')::uuid[]) = 0 OR
        id = ANY(sqlc.narg('user_ids')::uuid[])
    ) AND
    (
        sqlc.narg('user_names')::text[] IS NULL OR
        cardinality(sqlc.narg('user_names')::text[]) = 0 OR
        user_name = ANY(sqlc.narg('user_names')::text[])
    ) AND
    (
        sqlc.narg('roll_nos')::text[] IS NULL OR
        cardinality(sqlc.narg('roll_nos')::text[]) = 0 OR
        roll_no = ANY(sqlc.narg('roll_nos')::text[])
    )
LIMIT sqlc.arg('limit')
OFFSET sqlc.arg('offset');