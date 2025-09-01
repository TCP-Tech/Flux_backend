-- +goose up
-- Create a sequence that starts at 1234
CREATE SEQUENCE problems_id_seq START WITH 1234;

-- Create the problems table using the sequence for the primary key
CREATE TABLE problems (
    id INTEGER PRIMARY KEY DEFAULT nextval('problems_id_seq'),
    title VARCHAR(255) NOT NULL,
    difficulty INTEGER NOT NULL,
    evaluator VARCHAR(255) NOT NULL,
    lock_id UUID REFERENCES locks(id) ON DELETE SET NULL,
    created_by UUID NOT NULL REFERENCES users(id),
    last_updated_by UUID NOT NULL REFERENCES users(id),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE TABLE standard_problem_data (
    problem_id INTEGER NOT NULL,
    statement TEXT NOT NULL,
    input_format TEXT NOT NULL,
    output_format TEXT NOT NULL,
    function_definitons JSONB,
    example_testcases JSONB,
    notes TEXT,
    memory_limit_kb INTEGER NOT NULL,
    time_limit_ms INTEGER NOT NULL,
    submission_link TEXT,
    last_updated_by UUID NOT NULL REFERENCES users(id),
    
    CONSTRAINT fk_problem
        FOREIGN KEY (problem_id) REFERENCES problems(id),
    CONSTRAINT uq_problem_id
        UNIQUE (problem_id),
    CONSTRAINT uq_submission_link
        UNIQUE (submission_link)
);

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
DROP TABLE standard_problem_data;
DROP TABLE problems;
DROP SEQUENCE problems_id_seq;