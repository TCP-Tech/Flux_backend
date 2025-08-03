-- +goose up
-- Tournaments Table
CREATE TABLE tournaments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(), -- Unique identifier for the tournament
    title VARCHAR(255) NOT NULL UNIQUE, -- The title of the tournament
    rounds INTEGER NOT NULL, -- Number of rounds in the tournament
    created_by UUID NOT NULL REFERENCES users(id), -- The user who created this tournament (foreign key)
    updated_by UUID NOT NULL REFERENCES users(id), -- The last user to update this tournament (foreign key)
    start_time TIMESTAMP WITH TIME ZONE NOT NULL, -- When the tournament begins
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- indexes for common lookup fields
CREATE INDEX idx_tournaments_created_by ON tournaments(created_by);
CREATE INDEX idx_tournaments_updated_by ON tournaments(updated_by);
CREATE INDEX idx_tournaments_start_time ON tournaments(start_time);

-- +goose StatementBegin
-- A trigger to automatically update 'updated_at' on every row modification for tournament table
CREATE OR REPLACE FUNCTION update_tournaments_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';
-- +goose StatementEnd

CREATE TRIGGER update_tournaments_updated_at BEFORE UPDATE ON tournaments FOR EACH ROW EXECUTE FUNCTION update_tournaments_updated_at_column();


-- Tournament Contests Table (Many-to-Many relationship between tournaments and contests)
CREATE TABLE tournament_contests (
    contest_id UUID NOT NULL REFERENCES contest(id),
    tournament_id UUID NOT NULL REFERENCES tournaments(id),
    round INTEGER NOT NULL, -- The round number within the tournament

    -- Composite Primary Key: Ensures a contest is listed only once per tournament
    PRIMARY KEY (contest_id, tournament_id) -- Defines the composite primary key
);

-- indexes for foreign keys in the join table
CREATE INDEX idx_tournament_contests_contest_id ON tournament_contests(contest_id);
CREATE INDEX idx_tournament_contests_tournament_id ON tournament_contests(tournament_id);

-- +goose Down
DROP INDEX idx_tournament_contests_tournament_id;
DROP INDEX idx_tournament_contests_contest_id;
DROP TABLE tournament_contests;
DROP TRIGGER update_tournaments_updated_at ON tournaments;
DROP INDEX idx_tournaments_start_time;
DROP INDEX idx_tournaments_updated_by;
DROP INDEX idx_tournaments_created_by;
DROP TABLE tournaments;