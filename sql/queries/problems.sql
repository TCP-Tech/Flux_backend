-- name: AddProblem :one
INSERT INTO problems (
    title,
    difficulty,
    evaluator,
    lock_id,
    created_by,
    last_updated_by
) VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: AddStandardProblemData :one
INSERT INTO standard_problem_data (
    problem_id,
    statement,
    input_format,
    output_format,
    function_definitons,
    example_testcases,
    notes,
    memory_limit_kb,
    time_limit_ms,
    submission_link,
    last_updated_by
) VALUES (
    $1,  -- problem_id
    $2,  -- statement
    $3,  -- input_format
    $4,  -- output_format
    $5,  -- function_definiton
    $6,  -- example_testcases
    $7,  -- notes
    $8,  -- memory_limit_kb
    $9,  -- time_limit_ms
    $10,  -- submission_link
    $11  -- last_)updated_by
)
RETURNING *;

-- name: CheckPlatformType :one
SELECT $1::Platform;

-- name: UpdateProblem :one
UPDATE problems SET
    title=$2,
    difficulty=$3,
    evaluator=$4,
    lock_id=$5,
    last_updated_by=$6
WHERE 
    id=$1
RETURNING *;

-- name: GetProblemByID :one
SELECT 
    p.id,
    p.title,
    p.difficulty,
    p.evaluator,
    p.lock_id,
    p.created_by,

    l.access,
    l.timeout
FROM
    problems p
LEFT JOIN
    locks l
ON
    p.lock_id = l.id
WHERE p.id=$1;

-- name: GetStandardProblemData :one
SELECT * FROM standard_problem_data WHERE problem_id=$1;

-- name: UpdateStandardProblemData :one
UPDATE standard_problem_data
SET
    statement = $2,
    input_format = $3,
    output_format = $4,
    function_definitons = $5,
    example_testcases = $6,
    notes = $7,
    memory_limit_kb = $8,
    time_limit_ms = $9,
    submission_link = $10,
    last_updated_by = $11
WHERE
    problem_id = $1
RETURNING *;

-- name: GetProblemsByFilters :many
SELECT
    p.id,
    p.title,
    p.difficulty,
    p.evaluator,
    p.created_by,
    p.lock_id,

    l.timeout,
    l.access
FROM
    problems AS p
JOIN 
    standard_problem_data AS pd
ON
    p.id = pd.problem_id
LEFT JOIN
    locks AS l ON p.lock_id = l.id
WHERE
    -- Optional filter by a list of problem IDs
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
    -- Optional filter by evaluator
    (
        sqlc.narg('evaluator')::text IS NULL OR
        p.evaluator = sqlc.narg('evaluator')::text
    )
AND
    (
        -- Title search with wildcards handled in SQL
        sqlc.narg('title_search')::text IS NULL OR
        p.title ILIKE '%' || sqlc.arg('title_search')::text || '%'    
    )
ORDER BY
    p.created_at DESC
LIMIT
    sqlc.arg('limit')
OFFSET
    sqlc.arg('offset');
