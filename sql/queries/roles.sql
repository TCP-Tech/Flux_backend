-- name: GetUserRolesByUserName :many
SELECT * FROM user_roles WHERE user_id = $1;