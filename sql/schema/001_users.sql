-- migrations/001_create_users_table.sql
-- +goose Up
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    roll_no VARCHAR(255) NOT NULL,
    user_name VARCHAR(255) NOT NULL,
    first_name VARCHAR(255) NOT NULL,
    last_name VARCHAR(255) NOT NULL,
    email VARCHAR(255) NOT NULL,
    password_hash TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_users_roll_no UNIQUE (roll_no),
    CONSTRAINT uq_users_user_name UNIQUE (user_name),
    CONSTRAINT uq_users_email UNIQUE (email)
);


-- +goose Down
DROP TABLE users;