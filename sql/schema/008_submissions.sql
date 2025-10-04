-- +goose up
-- Submissions Table
CREATE TABLE submissions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    submitted_by UUID NOT NULL REFERENCES users(id), -- The user who made the submission
    contest_id UUID REFERENCES contests(id) DEFAULT NULL, -- The contest the submission belongs to (optional, can be null)
    problem_id INTEGER NOT NULL REFERENCES problems(id), -- The problem that was submitted
    solution JSONB NOT NULL, -- The submitted code
    state VARCHAR(255) NOT NULL, -- The final status of the submission (e.g., 'Accepted', 'Wrong Answer')
    submitted_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- cf submission table
CREATE TABLE cf_submissions(
    cf_sub_id BIGINT PRIMARY KEY NOT NULL,
    submission_id UUID REFERENCES submissions(id) NOT NULL,
    time_consumed_millis INTEGER NOT NULL,
    memory_consumed_bytes INTEGER NOT NULL,
    passed_test_count INTEGER NOT NULL,
    CONSTRAINT uq_cf_sub_id UNIQUE (cf_sub_id),
    CONSTRAINT uq_submission_id UNIQUE (submission_id)
);

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION touch_submission_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    UPDATE submissions
    SET updated_at = NOW()
    WHERE id = NEW.submission_id;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER cf_submissions_touch
AFTER INSERT OR UPDATE ON cf_submissions
FOR EACH ROW
EXECUTE FUNCTION touch_submission_updated_at();


-- indexes for common lookup fields
CREATE INDEX idx_submissions_submitted_by ON submissions(submitted_by);
CREATE INDEX idx_submissions_contest_id ON submissions(contest_id);
CREATE INDEX idx_submissions_problem_id ON submissions(problem_id);

-- +goose StatementBegin
-- Trigger to update 'updated_at' column
CREATE OR REPLACE FUNCTION update_submissions_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';
-- +goose StatementEnd

CREATE TRIGGER update_submissions_updated_at BEFORE UPDATE ON submissions FOR EACH ROW EXECUTE FUNCTION update_submissions_updated_at_column();

-- +goose down
DROP TRIGGER update_submissions_updated_at ON submissions;
DROP INDEX idx_submissions_problem_id;
DROP INDEX idx_submissions_contest_id;
DROP INDEX idx_submissions_submitted_by;
DROP TABLE cf_submissions;
DROP TABLE submissions;