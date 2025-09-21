-- +goose up
-- Bots Table
CREATE TABLE bots (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    account_name VARCHAR(255) NOT NULL, -- The username or ID of the bot on the external platform
    platform VARCHAR(255) NOT NULL, -- The name of the external platform (e.g., 'codeforces')
    website_data JSONB, -- Stores platform-specific data like API keys, session cookies, etc.
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- index for bot platform lookup
CREATE INDEX idx_bots_platform ON bots(platform);

-- +goose StatementBegin
-- Trigger to update 'updated_at' column
CREATE OR REPLACE FUNCTION update_bots_updated_at_column()
RETURNS TRIGGER AS $func$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$func$ language 'plpgsql';
-- +goose StatementEnd

CREATE TRIGGER update_bots_updated_at BEFORE UPDATE ON bots FOR EACH ROW EXECUTE FUNCTION update_bots_updated_at_column();

-- +goose Down
DROP TRIGGER update_bots_updated_at ON bots;
DROP INDEX idx_bots_platform;
DROP TABLE bots;