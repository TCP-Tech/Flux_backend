-- name: CreateContest :one
INSERT INTO contests (
    title,
    created_by,
    start_time,
    end_time,
    is_published,
    lock_id
) VALUES (
    sqlc.arg('title'),
    sqlc.arg('created_by'),
    sqlc.arg('start_time'),
    sqlc.arg('end_time'),
    sqlc.arg('is_published'),
    sqlc.arg('lock_id')
)
RETURNING *;

-- name: UnsetContestProblems :exec
DELETE FROM contest_problems WHERE contest_id=$1;

-- name: AddProblemToContest :one
INSERT INTO contest_problems (
    contest_id,
    problem_id,
    score
) VALUES (
    $1,
    $2,
    $3
)
RETURNING *;

-- name: UnRegisterContestUsers :exec
DELETE FROM contest_registered_users WHERE contest_id=$1;

-- name: RegisterUserToContest :one
INSERT INTO contest_registered_users (
    user_id,
    contest_id
) VALUES (
    $1,
    $2
)
RETURNING *;

-- name: DeleteProblemsByContestId :exec
DELETE FROM contest_problems WHERE contest_id = $1;

-- name: DeleteUsersByContestId :exec
DELETE FROM contest_registered_users WHERE contest_id = $1;

-- name: IsUserRegisteredInContest :one
SELECT EXISTS(
    SELECT contest_id, user_id FROM
     contest_registered_users WHERE contest_id=$1 AND user_id=$1
);

-- name: GetContestByID :one
SELECT 
    contests.*,
    locks.timeout as lock_timeout,
    locks.access
FROM contests
LEFT JOIN locks ON
contests.lock_id = locks.id
WHERE contests.id=$1;

-- name: GetContestProblemsByContestID :many
SELECT * FROM contest_problems WHERE contest_id=$1;

-- name: GetContestUsers :many
SELECT user_id FROM contest_registered_users WHERE contest_id=$1;

-- name: GetContestsByFilters :many
SELECT
    c.id,
    c.title,
    c.created_by,
    c.created_at,
    c.updated_at,
    c.start_time,
    c.end_time,
    c.is_published,
    c.lock_id,
    l.access as lock_access,
    l.timeout as lock_timeout
FROM
    contests AS c
LEFT JOIN
    locks AS l ON c.lock_id = l.id
WHERE
    -- Optional filter by a list of contest IDs
    (   sqlc.slice('contest_ids')::uuid[] IS NULL OR
        cardinality(sqlc.slice('contest_ids')::uuid[]) = 0 OR c.id = ANY(sqlc.slice('contest_ids')::uuid[]))
AND
    -- Optional filter by published status
    (sqlc.narg('is_published')::boolean IS NULL OR c.is_published = sqlc.narg('is_published')::boolean)
AND
    -- Optional filter by lock_id
    (sqlc.narg('lock_id')::uuid IS NULL OR c.lock_id = sqlc.narg('lock_id')::uuid)
AND
    -- Title search with wildcards handled in SQL
    c.title ILIKE '%' || sqlc.arg('title_search')::text || '%'
ORDER BY
    c.created_at DESC
LIMIT
    sqlc.arg('limit')
OFFSET
    sqlc.arg('offset');

-- name: GetUserRegisteredContests :many
SELECT contest_id FROM contest_registered_users WHERE user_id=$1;

-- name: UpdateContest :one
UPDATE contests SET
    title=$1,
    start_time=$2,
    end_time=$3
WHERE id=$4
RETURNING *;

-- name: DeleteContestByID :exec
DELETE FROM contests WHERE id=$1;