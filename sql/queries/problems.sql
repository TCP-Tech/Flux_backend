-- name: AddProblem :one
INSERT INTO problems (
    title,
    statement,
    input_format,
    output_format,
    example_testcases,
    notes,
    memory_limit_kb,
    time_limit_ms,
    created_by,
    last_updated_by,
    difficulty,
    submission_link,
    platform,
    lock_id
) VALUES (
    $1, -- title
    $2, -- statement
    $3, -- input_format (can be NULL)
    $4, -- output_format (can be NULL)
    $5, -- samples (can be NULL)
    $6, -- notes (can be NULL)
    $7, -- memory_limit_kb
    $8, -- time_limit_ms
    $9, -- created_by (UUID)
    $9, -- last_updated_by (UUID)
    $10, -- difficulty (can be NULL)
    $11, -- submission_link (can be NULL)
    $12, -- platform (can be NULL)
    $13 -- lock_id
)
RETURNING *;

-- name: CheckPlatformType :one
SELECT $1::Platform;

-- name: GetProblemById :one
SELECT
    -- Explicitly list all columns from 'problems' except 'lock_id'
    problems.*,

    -- Select only the 'access' column from the 'locks' table
    locks.access as lock_access,
    locks.timeout as lock_timeout
FROM
    problems
LEFT JOIN
    locks ON problems.lock_id = locks.id
WHERE
    problems.id = $1;


-- name: UpdateProblem :one
UPDATE problems
SET
    title = $1,
    statement = $2,
    input_format = $3,
    output_format = $4,
    example_testcases = $5,
    notes = $6,
    memory_limit_kb = $7,
    time_limit_ms = $8,
    difficulty = $9,
    submission_link = $10,
    platform = $11,
    last_updated_by = $12,
    lock_id = $13
WHERE
    id = $14
RETURNING *;

-- name: GetProblemsByFilters :many
SELECT
    p.id,
    p.title,
    p.difficulty,
    p.platform,
    p.created_by,   
    p.created_at,
    l.id as lock_id,
    l.timeout as lock_timeout,
    l.access as lock_access
FROM
    problems AS p
LEFT JOIN
    locks AS l ON p.lock_id = l.id
WHERE
    -- Optional filter by a list of problem IDs
    -- This checks if the input slice is empty. If it is, the condition is true.
    -- If not empty, it checks if the problem ID is in the slice.
    (
        (sqlc.slice('problem_ids')::int[]) IS NULL OR
        cardinality(sqlc.slice('problem_ids')::int[]) = 0 OR
        p.id = ANY(sqlc.slice('problem_ids')::int[])
    )
AND
    -- Optional filter by lock_id
    (
        sqlc.narg('lock_id')::uuid IS NULL OR
        p.lock_id = sqlc.narg('lock_id')::uuid
    )
AND
    -- Optional filter by creator
    (
        sqlc.narg('created_by')::uuid IS NULL OR
        p.created_by = sqlc.narg('created_by')::uuid
    )
AND
    -- Title search with wildcards handled in SQL
    p.title ILIKE '%' || sqlc.arg('title_search')::text || '%'
ORDER BY
    p.created_at DESC
LIMIT
    sqlc.arg('limit')
OFFSET
    sqlc.arg('offset');

-- name: GetProblemAuth :one
SELECT locks.id, locks.access, locks.timeout
FROM problems
LEFT JOIN locks ON problems.lock_id = locks.id
WHERE problems.id = $1;
