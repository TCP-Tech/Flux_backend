-- name: GetManagerMailIDs :many
SELECT email FROM users u JOIN user_roles ur ON u.id = ur.user_id WHERE role_name=$1;