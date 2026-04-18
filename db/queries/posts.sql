-- name: GetPost :one
SELECT * FROM posts
WHERE slug = $1 AND delete_time IS NULL;

-- name: ListPosts :many
SELECT * FROM posts
WHERE delete_time IS NULL
ORDER BY create_time DESC
LIMIT $1 OFFSET $2;

-- name: CreatePost :one
INSERT INTO posts (title, slug, body, status, author_id, create_time, update_time)
VALUES ($1, $2, $3, $4, $5, now(), now())
RETURNING *;
