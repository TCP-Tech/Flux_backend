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