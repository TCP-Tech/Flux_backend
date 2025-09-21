-- name: CanSubmitProblemInPractice :one
SELECT COALESCE(
    max_end_time IS NULL OR
    now() > max_end_time
)::BOOLEAN
FROM (
    SELECT max(c.end_time) AS max_end_time
    FROM problems p
    JOIN contest_problems cp ON p.id = cp.problem_id
    JOIN contests c ON c.id = cp.contest_id
    WHERE problem_id = $1 AND c.lock_id IS NOT NULL
) AS t;

-- name: InsertSubmission :one
INSERT INTO submissions (
    submitted_by,
    contest_id,
    problem_id,
    solution,
    state
) VALUES (
    $1,
    $2,
    $3,
    $4,
    $5
)
RETURNING *;