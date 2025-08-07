-- +goose Up

CREATE TYPE lock_type AS ENUM ('manual', 'timer');

-- locks for locking a problem and a contest until they start
CREATE TABLE locks (
    id UUID PRIMARY KEY NOT NULL DEFAULT gen_random_uuid(),
    name VARCHAR(50) NOT NULL,
    created_by UUID NOT NULL REFERENCES users(id),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    description TEXT NOT NULL DEFAULT '',
    access VARCHAR(50) NOT NULL DEFAULT 'role_manager'
           REFERENCES roles(role_name),
    lock_type  lock_type NOT NULL,
    timeout    TIMESTAMP WITH TIME ZONE,

    -- Enforce mutual exclusivity using CHECK constraints
    -- A manual lock must have a 'locked' value and no 'timeout'
    CHECK (
        (lock_type = 'manual' AND timeout IS NULL)
        OR
    -- A timer lock must have a 'timeout' value and no 'locked' value
        (lock_type = 'timer' AND timeout IS NOT NULL)
    )
);

-- +goose Down
DROP TABLE locks;
DROP TYPE lock_type;