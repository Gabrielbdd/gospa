-- +goose Up
CREATE TABLE posts (
    id          BIGSERIAL PRIMARY KEY,
    title       VARCHAR(255) NOT NULL,
    slug        VARCHAR(255) NOT NULL UNIQUE,
    body        TEXT NOT NULL,
    status      VARCHAR(20) NOT NULL DEFAULT 'draft',
    author_id   BIGINT NOT NULL,
    create_time TIMESTAMPTZ NOT NULL DEFAULT now(),
    update_time TIMESTAMPTZ NOT NULL DEFAULT now(),
    delete_time TIMESTAMPTZ
);

CREATE INDEX idx_posts_slug ON posts(slug);
CREATE INDEX idx_posts_status ON posts(status) WHERE delete_time IS NULL;

-- +goose Down
DROP TABLE IF EXISTS posts;
