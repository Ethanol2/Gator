-- name: AddFeed :one
INSERT INTO feeds (id, created_at, updated_at, name, url, user_id)
VALUES (
    $1,
    $2,
    $2,
    $3,
    $4,
    $5
)
RETURNING *;

-- name: GetAllFeeds :many
SELECT * FROM feeds;

-- name: GetFeedWithURL :one
SELECT * FROM feeds WHERE url = $1;

-- name: CreateFeedFollow :one
WITH inserted_feed_follow AS (
    INSERT INTO feed_follows (id, created_at, updated_at, user_id, feed_id)
    VALUES (
        $1, $2, $2, $3, $4
    )
    RETURNING *
)

SELECT
    inserted_feed_follow.*,
    feeds.name AS feed_name,
    users.name AS user_name
FROM inserted_feed_follow
INNER JOIN feeds ON feeds.id = inserted_feed_follow.feed_id
INNER JOIN users ON users.id = inserted_feed_follow.user_id;

-- name: GetFollowing :many
SELECT feed_follows.*, feeds.name AS feed_name
    FROM feed_follows
    INNER JOIN feeds ON feeds.id = feed_follows.feed_id
    WHERE feed_follows.user_id = $1;

-- name: RemoveFollowing :exec
DELETE FROM feed_follows
    WHERE 
        feed_follows.user_id = $1 AND
        feed_follows.feed_id = $2;

-- name: MarkFeedFetched :exec
UPDATE feeds
    SET last_fetched_at = $2
    WHERE id = $1;

-- name: GetNextFeedToFetch :one
SELECT feeds.* FROM feeds
    INNER JOIN feed_follows AS followed ON feeds.id = followed.feed_id
    WHERE followed.user_id = $1
    ORDER BY feeds.last_fetched_at ASC NULLS FIRST;

-- name: CreatePost :one
INSERT INTO posts (
    id, 
    created_at, 
    updated_at, 
    title, 
    url, 
    description, 
    published_at, 
    feed_id)

VALUES (
    $1, $2, $2, $3, $4, $5, $6, $7
)
RETURNING *;

-- name: GetPosts :many
SELECT posts.* FROM posts
    INNER JOIN feed_follows AS followed ON posts.feed_id = followed.feed_id
    WHERE followed.user_id = $1
    ORDER BY posts.published_at ASC
    LIMIT $2 OFFSET $3;