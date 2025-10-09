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

-- name: GetSubmissionByID :one
SELECT * From submissions WHERE id=$1;

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

-- name: GetCfSubmissionById :one
SELECT cs.cf_sub_id, cs.time_consumed_millis, 
    cs.memory_consumed_bytes, cs.passed_test_count
FROM submissions s LEFT JOIN cf_submissions cs ON s.id = cs.submission_id
WHERE s.id = $1;

-- name: UpdateSubmissionByID :one
UPDATE submissions SET state=$2 WHERE id=$1 RETURNING *;

-- name: InsertCfSubmission :one
INSERT INTO cf_submissions
    (
        cf_sub_id,
        submission_id,
        time_consumed_millis,
        memory_consumed_bytes,
        passed_test_count
    )
VALUES ($1, $2, $3, $4, $5) RETURNING *;

-- name: GetBulkCfSubmission :many
SELECT s.state, cs.* 
FROM cf_submissions cs
JOIN submissions s ON cs.submission_id = s.id
WHERE s.state != ALL(sqlc.arg(cf_sink_states)::VARCHAR[]) AND cs.cf_sub_id IS NOT NULL;

-- name: BulkUpdateSubmissionState :exec
UPDATE submissions
SET
    state = data.state
FROM (
    SELECT
        id_arr.id,
        state_arr.state
    FROM UNNEST(sqlc.arg(ids)::uuid[]) WITH ORDINALITY AS id_arr(id, idx)
    JOIN UNNEST(sqlc.arg(states)::VARCHAR[]) WITH ORDINALITY AS state_arr(state, idx2) ON idx = idx2
) AS data
WHERE submissions.id = data.id;

-- name: BulkUpdateCfSubmission :exec
UPDATE cf_submissions
SET
    time_consumed_millis = data.time_consumed_millis,
    memory_consumed_bytes = data.memory_consumed_bytes,
    passed_test_count = data.passed_test_count
FROM (
    SELECT 
        id_arr.id,
        time_arr.time_consumed_millis,
        mem_arr.memory_consumed_bytes,
        passed_arr.passed_test_count
    FROM UNNEST(sqlc.arg(ids)::BIGINT[]) WITH ORDINALITY AS id_arr(id, idx)
    JOIN UNNEST(sqlc.arg(times)::INTEGER[]) WITH ORDINALITY AS time_arr(time_consumed_millis, idx2) ON idx = idx2
    JOIN UNNEST(sqlc.arg(memories)::INTEGER[]) WITH ORDINALITY AS mem_arr(memory_consumed_bytes, idx3) ON idx = idx3
    JOIN UNNEST(sqlc.arg(passed_test_counts)::INTEGER[]) WITH ORDINALITY AS passed_arr(passed_test_count, idx4) ON idx = idx4
) AS data
WHERE cf_submissions.cf_sub_id = data.id;

-- name: PollPendingSubmissions :many
SELECT * FROM submissions WHERE state = ANY(sqlc.arg(pending_states)::VARCHAR[]);

-- name: UpdateBot :one
UPDATE bots SET cookies=$2 WHERE name=$1 RETURNING *;

-- name: DeleteBot :exec
DELETE FROM bots WHERE name=$1;