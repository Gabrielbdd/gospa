-- +goose Up
-- Seed data for local development. Run with: mise run seed
-- Safe to re-run: uses ON CONFLICT DO NOTHING.

INSERT INTO posts (title, slug, body, status, author_id)
VALUES ('Hello World', 'hello-world', 'Welcome to your new Gofra app.', 'published', 1)
ON CONFLICT (slug) DO NOTHING;
