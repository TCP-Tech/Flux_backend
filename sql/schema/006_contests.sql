-- +goose up
-- Contest Table
CREATE TABLE contests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(), -- Unique identifier for the contest
    title VARCHAR(255) NOT NULL, -- The title of the contest
    created_by UUID NOT NULL REFERENCES users(id), -- The user who created this contest (foreign key)
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(), -- Timestamp when the contest was created
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(), -- Timestamp of the last update
    start_time TIMESTAMP WITH TIME ZONE, -- When the contest begins
    end_time TIMESTAMP WITH TIME ZONE NOT NULL, -- When the contest ends
    is_published BOOLEAN NOT NULL DEFAULT FALSE, -- Is the contest visible to users?
    lock_id UUID REFERENCES locks(id)
);

-- indexes for common lookup fields
CREATE INDEX idx_contest_created_by ON contests(created_by);
CREATE INDEX idx_contest_start_time ON contests(start_time);
CREATE INDEX idx_contest_end_time ON contests(end_time);

-- A trigger to automatically update 'updated_at' on every row modification for contest table
CREATE TRIGGER update_contest_updated_at BEFORE UPDATE ON contests FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();


-- Contest Problems Table (Many-to-Many relationship between contests and problems)
CREATE TABLE contest_problems (
    -- Composite Primary Key: Ensures a problem is listed only once per contest
    contest_id UUID NOT NULL REFERENCES contests(id),
    problem_id INTEGER NOT NULL REFERENCES problems(id),
    score INTEGER NOT NULL, -- The score value for this specific problem within this contest

    CONSTRAINT contest_problems_pkey PRIMARY KEY (contest_id, problem_id) -- Defines the composite primary key
);

-- indexes for foreign keys in the join table
CREATE INDEX idx_contest_problems_contest_id ON contest_problems(contest_id);
CREATE INDEX idx_contest_problems_problem_id ON contest_problems(problem_id);

-- Contest Registered Users Table
-- This table allows only specific users to submit to a contest
CREATE TABLE contest_registered_users (
    user_id UUID NOT NULL REFERENCES users(id), -- The user who is registered
    contest_id UUID NOT NULL REFERENCES contests(id), -- The contest they are registered for

    -- The combination of user_id and contest_id is the primary key,
    -- ensuring a user can only be registered for a contest once.
    PRIMARY KEY (user_id, contest_id)
);

-- indexes for foreign keys
CREATE INDEX idx_contest_registered_users_user_id ON contest_registered_users(user_id);
CREATE INDEX idx_contest_registered_users_contest_id ON contest_registered_users(contest_id);

-- +goose Down
DROP TABLE contest_registered_users;
DROP INDEX idx_contest_problems_problem_id;
DROP INDEX idx_contest_problems_contest_id;
DROP TABLE contest_problems;
DROP TRIGGER update_contest_updated_at ON contests;
DROP INDEX idx_contest_end_time;
DROP INDEX idx_contest_start_time;
DROP INDEX idx_contest_created_by;
DROP TABLE contests;