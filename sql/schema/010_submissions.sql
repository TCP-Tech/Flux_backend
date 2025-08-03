-- +goose up
-- Submissions Table
CREATE TABLE submissions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    bot_account_id UUID NOT NULL REFERENCES bots(id), -- The bot account used for the submission
    website_data JSONB, -- Stores platform-specific submission data like the site submission ID
    submitted_by UUID NOT NULL REFERENCES users(id), -- The user who made the submission
    contest_id UUID REFERENCES contest(id), -- The contest the submission belongs to (optional, can be null)
    problem_id UUID NOT NULL REFERENCES problems(id), -- The problem that was submitted
    language VARCHAR(50) NOT NULL, -- The programming language used
    solution TEXT NOT NULL, -- The submitted code
    status TEXT, -- The final status of the submission (e.g., 'Accepted', 'Wrong Answer')
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- indexes for common lookup fields
CREATE INDEX idx_submissions_submitted_by ON submissions(submitted_by);
CREATE INDEX idx_submissions_contest_id ON submissions(contest_id);
CREATE INDEX idx_submissions_problem_id ON submissions(problem_id);
CREATE INDEX idx_submissions_bot_account_id ON submissions(bot_account_id);
CREATE INDEX idx_submissions_language ON submissions(language);

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
DROP INDEX idx_submissions_bot_account_id;
DROP INDEX idx_submissions_problem_id;
DROP INDEX idx_submissions_contest_id;
DROP INDEX idx_submissions_submitted_by;
DROP TABLE submissions;