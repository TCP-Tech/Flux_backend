-- +goose Up
CREATE TABLE tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(), -- Or an auto-incrementing integer if preferred
    
    -- Store the HASHED token, not the plain-text token
    hashed_token VARCHAR(255) NOT NULL UNIQUE, 
    
    -- The purpose of this token (e.g., 'signup', 'password_reset')
    purpose VARCHAR(50) NOT NULL, 
    
    -- User data that we need to store to verify
    payload JSONB NOT NULL,

    email VARCHAR(50) NOT NULL,
    
    -- When this token expires
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL, 
    
    -- Timestamps for auditing
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- Add an index on hashed_token for fast lookups
CREATE INDEX idx_tokens_hashed_token ON tokens (hashed_token);
-- Add an index on expires_at for efficient cleanup
CREATE INDEX idx_tokens_expires_at ON tokens (expires_at);


-- +goose Down
DROP TABLE tokens;