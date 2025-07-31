-- name: CreateToken :one
INSERT INTO tokens (hashed_token, purpose, payload, email, expires_at)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetTokenByEmailAndPurpose :one
SELECT *
FROM tokens
WHERE email = $1 AND purpose = $2
-- helps handle duplicate tokens:
ORDER BY created_at DESC 
LIMIT 1;

-- name: DeleteByEmailAndPurpose :exec
DELETE FROM tokens WHERE email = $1 AND purpose = $2;