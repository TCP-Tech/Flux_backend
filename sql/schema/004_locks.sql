-- +goose Up
-- locks for locking a problem and a contest until they start
CREATE TABLE locks (
    id UUID PRIMARY KEY NOT NULL DEFAULT gen_random_uuid(),
    timeout TIMESTAMP WITH TIME ZONE NOT NULL,
    name VARCHAR(50) NOT NULL,
    created_by UUID NOT NULL REFERENCES users(id),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    description TEXT NOT NULL DEFAULT '',
    access VARCHAR(50) DEFAULT 'role_manager' 
           REFERENCES roles(role_name) 
           ON DELETE SET NULL
);

-- +goose Down
DROP TABLE locks;