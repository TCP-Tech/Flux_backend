-- +goose up
-- Solved Table
-- This is a lightweight cache to quickly check if a user has solved a problem
-- in a specific contest. It's a great tool for the UI to display checkmarks
-- and also quickly check if the score should be added to user for duplicate submission
CREATE TABLE solved (
    user_id UUID NOT NULL REFERENCES users(id),
    contest_id UUID NOT NULL REFERENCES contest(id),
    problem_id UUID NOT NULL REFERENCES problems(id),
    
    -- The composite primary key ensures a user can only have one "solved" entry
    -- for a specific problem within a specific contest.
    PRIMARY KEY (user_id, contest_id, problem_id)
);

-- indexes for quick lookups
CREATE INDEX idx_solved_user_id ON solved(user_id);
CREATE INDEX idx_solved_contest_id ON solved(contest_id);
CREATE INDEX idx_solved_problem_id ON solved(problem_id);


-- User Scores Table
-- This is the materialized view or cache for the leaderboard, storing the final
-- score for a user on a specific problem in a contest.
CREATE TABLE user_scores (
    user_id UUID NOT NULL REFERENCES users(id),
    contest_id UUID NOT NULL REFERENCES contest(id),
    problem_id UUID NOT NULL REFERENCES problems(id),
    score INTEGER NOT NULL,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    
    -- References the specific submission that resulted in this score.
    -- Used for calculating penalties and providing a link to the winning submission.
    submission_id UUID NOT NULL REFERENCES submissions(id),
    
    -- The composite primary key ensures a user only has one final score entry
    -- for a specific problem within a specific contest.
    PRIMARY KEY (user_id, contest_id, problem_id)
);

-- indexes for foreign keys
CREATE INDEX idx_user_scores_user_id ON user_scores(user_id);
CREATE INDEX idx_user_scores_contest_id ON user_scores(contest_id);
CREATE INDEX idx_user_scores_problem_id ON user_scores(problem_id);
CREATE INDEX idx_user_scores_submission_id ON user_scores(submission_id);

-- +goose StatementBegin
-- A function to automatically update 'updated_at' on every row modification
CREATE OR REPLACE FUNCTION update_user_scores_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';
-- +goose StatementEnd

-- A trigger to run the function before every update
CREATE TRIGGER update_user_scores_updated_at BEFORE UPDATE ON user_scores FOR EACH ROW EXECUTE FUNCTION update_user_scores_updated_at_column();

-- +goose Down
DROP TRIGGER update_user_scores_updated_at ON user_scores;
DROP INDEX idx_user_scores_submission_id;
DROP INDEX idx_user_scores_problem_id;
DROP INDEX idx_user_scores_contest_id;
DROP INDEX idx_user_scores_user_id;
DROP TABLE user_scores;
DROP INDEX idx_solved_problem_id;
DROP INDEX idx_solved_contest_id;
DROP INDEX idx_solved_user_id;
DROP TABLE solved;