-- name: CreateTournament :one
INSERT INTO tournaments (
    title, is_published, created_by
) VALUES ($1, $2, $3)
RETURNING *;

-- name: GetTournamentLatestRoundEndTime :one
SELECT
    -- COALESCE handles the case where the latest round has no contests yet
    -- cast to timestamptz for type inference by sqlc
    COALESCE(MAX(c.end_time), '1970-01-01'::timestamptz)::timestamptz
FROM
    contests c
JOIN
    tournament_contests tc ON c.id = tc.contest_id
JOIN
    tournament_rounds tr ON tc.round_id = tr.id
WHERE
    tr.tournament_id = $1
AND
    -- This subquery finds the highest round number for the tournament
    tr.round_number = (
        SELECT MAX(round_number)
        FROM tournament_rounds
        WHERE tournament_id = $1
    );


-- name: CreateTournamentRound :one
INSERT INTO tournament_rounds (
    tournament_id, round_number, title, lock_id, created_by
) VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetTournamentById :one
SELECT
    t.id,
    t.title,
    t.created_by,
    t.is_published,
    COALESCE(MAX(tr.round_number), 0)::int as rounds
FROM
    tournaments t
LEFT JOIN
    tournament_rounds tr
ON
    t.id = tr.tournament_id
WHERE
    t.id = $1
GROUP BY
    t.id;

-- name: GetTournamentLatestRound :one
SELECT 
    tr.id, tr.tournament_id, tr.round_number, tr.title, tr.lock_id, tr.created_by
FROM
    tournament_rounds tr
JOIN
    tournaments t ON tr.tournament_id = t.id
WHERE
    t.id = $1
ORDER BY
    tr.round_number DESC
LIMIT 1;

-- name: GetTournamentRoundByNumber :one
SELECT
    tr.id, tr.tournament_id,
    tr.round_number,
    tr.title,
    tr.lock_id,
    tr.created_by,

    -- lock fields
    l.access,
    l.timeout
FROM
    tournament_rounds tr
LEFT JOIN
    locks l
ON 
    tr.lock_id = l.id
WHERE
    tr.tournament_id = $1 AND tr.round_number = $2;

-- name: GetTournamentContests :many
SELECT contest_id FROM tournament_contests WHERE round_id = $1;

-- name: DeleteTournamentContests :exec
DELETE FROM tournament_contests WHERE round_id = $1;

-- name: AddTournamentContest :exec
INSERT INTO tournament_contests (round_id, contest_id)
VALUES ($1, $2);

-- name: GetTournamentsByFilters :many
SELECT
    t.id,
    t.title,
    t.created_by,
    t.is_published,
    COALESCE(MAX(tr.round_number), 0)::int as rounds
FROM
    tournaments t
LEFT JOIN
    tournament_rounds tr
ON
    t.id = tr.tournament_id
WHERE
    -- Optional filter by creator
    (sqlc.narg('created_by')::uuid IS NULL OR t.created_by = sqlc.narg('created_by')::uuid)
AND
    -- Optional filter by published status
    (sqlc.narg('is_published')::boolean IS NULL OR t.is_published = sqlc.narg('is_published')::boolean)
AND
    -- Title search with wildcards handled in SQL
    t.title ILIKE '%' || sqlc.arg('title_search')::text || '%'
ORDER BY
    t.created_at DESC
LIMIT
    sqlc.arg('limit')
OFFSET
    sqlc.arg('offset');