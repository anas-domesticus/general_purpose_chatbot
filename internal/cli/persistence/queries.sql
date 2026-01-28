-- Simple SQLC queries demonstrating patterns

-- name: CreateUser :one
INSERT INTO users (email, name, status) 
VALUES ($1, $2, $3) 
RETURNING *;

-- name: GetUserByUUID :one
SELECT * FROM users WHERE uuid = $1;

-- name: GetUserByEmail :one  
SELECT * FROM users WHERE email = $1;

-- name: ListUsers :many
SELECT * FROM users 
ORDER BY created_at DESC 
LIMIT $1;

-- name: UpdateUserStatus :one
UPDATE users SET status = $2 
WHERE uuid = $1 
RETURNING *;