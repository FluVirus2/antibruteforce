CREATE TABLE IF NOT EXISTS list_types (
    id   INT PRIMARY KEY,
    name TEXT UNIQUE NOT NULL
);

INSERT INTO list_types (id, name)
VALUES (1, 'whitelist'), (2, 'blacklist')
ON CONFLICT (id) DO NOTHING;

CREATE TABLE IF NOT EXISTS subnets (
    id           BIGSERIAL PRIMARY KEY,
    subnet_type  INT NOT NULL REFERENCES list_types(id) ON DELETE RESTRICT,
    subnet       CIDR NOT NULL,
    comment      TEXT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (subnet_type, subnet)
);

CREATE INDEX IF NOT EXISTS idx_subnets_subnet_gist ON subnets USING GIST (subnet inet_ops);
