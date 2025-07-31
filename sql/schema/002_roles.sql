-- migrations/002_create_roles_tables.sql

-- +goose Up
CREATE TABLE roles (
    role_name VARCHAR(50) PRIMARY KEY NOT NULL UNIQUE
);

CREATE TABLE user_roles (
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    role_name VARCHAR(50) REFERENCES roles(role_name) ON DELETE CASCADE,
    PRIMARY KEY (user_id, role_name)
);

-- +goose Down
DROP TABLE user_roles;
DROP TABLE roles;