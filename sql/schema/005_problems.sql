-- +goose up
-- Create a sequence that starts at 1234
CREATE SEQUENCE problems_id_seq START WITH 1234;

CREATE TYPE Platform AS ENUM (
    'codeforces'
);

-- Create the problems table using the sequence for the primary key
CREATE TABLE problems (
    id INTEGER PRIMARY KEY DEFAULT nextval('problems_id_seq'),
    title VARCHAR(255) NOT NULL,
    statement TEXT NOT NULL,
    input_format TEXT NOT NULL,
    output_format TEXT NOT NULL,
    example_testcases JSONB,
    notes TEXT,
    memory_limit_kb INTEGER NOT NULL,
    time_limit_ms INTEGER NOT NULL,
    created_by UUID NOT NULL REFERENCES users(id),
    last_updated_by UUID NOT NULL REFERENCES users(id),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    difficulty INTEGER NOT NULL,
    submission_link TEXT NULL UNIQUE,
    platform Platform NULL,
    lock_id UUID REFERENCES locks(id) ON DELETE SET NULL
);

-- indexes for common lookup fields
CREATE INDEX idx_problems_created_by ON problems(created_by);
CREATE INDEX idx_problems_platform ON problems(platform);

-- +goose StatementBegin
-- A function to automatically update 'updated_at' on every row modification
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $func$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$func$ language 'plpgsql';
-- +goose StatementEnd

-- A trigger to run the function before every update
CREATE TRIGGER update_problems_updated_at BEFORE UPDATE ON problems FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();


-- +goose Down
DROP TRIGGER update_problems_updated_at ON problems;
DROP FUNCTION update_updated_at_column();
DROP INDEX idx_problems_platform;
DROP INDEX idx_problems_created_by;
DROP TABLE problems;
DROP SEQUENCE problems_id_seq;
DROP TYPE Platform;