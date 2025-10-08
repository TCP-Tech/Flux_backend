-- name: InsertBot :one
INSERT INTO bots (
    name, platform, cookies
) VALUES (
    $1, $2, $3
) RETURNING *;

-- name: GetBots :many
SELECT * FROM bots;

-- name: UpdateBotCookies :one
UPDATE bots SET cookies=$2 WHERE name=$1 RETURNING *;

-- name: DeleteBots :exec
DELETE FROM bots WHERE name=$1;