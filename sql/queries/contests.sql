-- name: CreateContest :one
INSERT INTO contest (
    title,
    created_by,
    start_time,
    end_time,
    is_published
) VALUES (
    $1,
    $2,
    $3,
    $4,
    $5
)
RETURNING *;

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

-- name: AddUserToContest :one
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