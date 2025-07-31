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
    platform
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
    $12  -- platform (can be NULL)
)
RETURNING id, created_at, updated_at;

-- name: CheckPlatformType :one
SELECT $1::platform_type;

-- name: GetProblemById :one
SELECT * FROM problems WHERE id = $1;

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
    last_updated_by = $12
WHERE
    id = $13
RETURNING *;