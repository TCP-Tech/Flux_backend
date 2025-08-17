-- ========= TRIGGER FUNCTION =========
-- A reusable function to automatically update the 'updated_at' column on any row update.
-- This ensures that we always have an accurate timestamp for the last modification.

-- +goose up
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION trigger_set_timestamp()
RETURNS TRIGGER AS $$
BEGIN
  NEW.updated_at = NOW();
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd


-- ========= TOURNAMENTS TABLE =========
-- Stores the main details for a tournament event.

CREATE TABLE tournaments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title VARCHAR(255) NOT NULL UNIQUE,
    is_published BOOLEAN NOT NULL DEFAULT FALSE,
    created_by UUID NOT NULL REFERENCES users(id),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    -- updated_at is managed by the trigger below
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- +goose StatementBegin
-- Trigger to automatically update the 'updated_at' timestamp
CREATE TRIGGER set_tournaments_timestamp
BEFORE UPDATE ON tournaments
FOR EACH ROW
EXECUTE PROCEDURE trigger_set_timestamp();
-- +goose StatementEnd

-- ========= TOURNAMENT ROUNDS TABLE =========
-- Stores the individual rounds that make up a tournament.

CREATE TABLE tournament_rounds (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    -- If a tournament has rounds, it cannot be deleted.
    tournament_id UUID NOT NULL,
    CONSTRAINT fk_rounds_tournament
        FOREIGN KEY (tournament_id)
        REFERENCES tournaments(id)
        ON DELETE RESTRICT,
        
    round_number INT NOT NULL,
    title VARCHAR(100) NOT NULL,
    lock_id UUID REFERENCES locks(id) ON DELETE SET NULL, -- If a lock is deleted, this becomes NULL.
    created_by UUID NOT NULL REFERENCES users(id),
    -- updated_at is managed by the trigger below
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),

    -- A tournament cannot have two rounds with the same number (e.g., two "Round 1"s).
    UNIQUE (tournament_id, round_number)
);

-- Indexes on foreign keys for faster joins and lookups
CREATE INDEX idx_tournament_rounds_tournament_id ON tournament_rounds(tournament_id);
CREATE INDEX idx_tournament_rounds_lock_id ON tournament_rounds(lock_id);

-- +goose StatementBegin
-- Trigger to automatically update the 'updated_at' timestamp
CREATE TRIGGER set_tournament_rounds_timestamp
BEFORE UPDATE ON tournament_rounds
FOR EACH ROW
EXECUTE PROCEDURE trigger_set_timestamp();
-- +goose StatementEnd

-- ========= TOURNAMENT CONTESTS MAPPING TABLE =========
-- This table links contests to specific rounds within a tournament.

CREATE TABLE tournament_contests (
    round_id UUID NOT NULL REFERENCES tournament_rounds(id), -- If a round is deleted, its contest links are also deleted.
    contest_id UUID NOT NULL REFERENCES contests(id), -- If a contest is deleted, its links are also deleted.

    -- A contest can only be in a specific round once.
    PRIMARY KEY (round_id, contest_id)
);

-- +goose down
-- Drop in reverse dependency order

-- Drop tournament_contests
DROP TABLE IF EXISTS tournament_contests;

-- Drop tournament_rounds and its trigger
DROP TRIGGER IF EXISTS set_tournament_rounds_timestamp ON tournament_rounds;
DROP TABLE IF EXISTS tournament_rounds;

-- Drop tournaments and its trigger
DROP TRIGGER IF EXISTS set_tournaments_timestamp ON tournaments;
DROP TABLE IF EXISTS tournaments;

-- Drop trigger function
DROP FUNCTION IF EXISTS trigger_set_timestamp;