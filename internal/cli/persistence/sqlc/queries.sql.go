package sqlc

import (
	"context"
	"github.com/jackc/pgx/v5/pgtype"
)

const createUser = `-- name: CreateUser :one
INSERT INTO users (email, name, status) 
VALUES ($1, $2, $3) 
RETURNING id, uuid, email, name, status, created_at, updated_at
`

type CreateUserParams struct {
	Email  string      `json:"email"`
	Name   string      `json:"name"`
	Status UsersStatus `json:"status"`
}

func (q *Queries) CreateUser(ctx context.Context, arg CreateUserParams) (User, error) {
	row := q.db.QueryRow(ctx, createUser, arg.Email, arg.Name, arg.Status)
	var i User
	err := row.Scan(
		&i.ID,
		&i.Uuid,
		&i.Email,
		&i.Name,
		&i.Status,
		&i.CreatedAt,
		&i.UpdatedAt,
	)
	return i, err
}

const getUserByUUID = `-- name: GetUserByUUID :one
SELECT id, uuid, email, name, status, created_at, updated_at FROM users WHERE uuid = $1
`

func (q *Queries) GetUserByUUID(ctx context.Context, uuid pgtype.UUID) (User, error) {
	row := q.db.QueryRow(ctx, getUserByUUID, uuid)
	var i User
	err := row.Scan(
		&i.ID,
		&i.Uuid,
		&i.Email,
		&i.Name,
		&i.Status,
		&i.CreatedAt,
		&i.UpdatedAt,
	)
	return i, err
}

const getUserByEmail = `-- name: GetUserByEmail :one
SELECT id, uuid, email, name, status, created_at, updated_at FROM users WHERE email = $1
`

func (q *Queries) GetUserByEmail(ctx context.Context, email string) (User, error) {
	row := q.db.QueryRow(ctx, getUserByEmail, email)
	var i User
	err := row.Scan(
		&i.ID,
		&i.Uuid,
		&i.Email,
		&i.Name,
		&i.Status,
		&i.CreatedAt,
		&i.UpdatedAt,
	)
	return i, err
}

const listUsers = `-- name: ListUsers :many
SELECT id, uuid, email, name, status, created_at, updated_at FROM users 
ORDER BY created_at DESC 
LIMIT $1
`

func (q *Queries) ListUsers(ctx context.Context, limit int32) ([]User, error) {
	rows, err := q.db.Query(ctx, listUsers, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []User
	for rows.Next() {
		var i User
		if err := rows.Scan(
			&i.ID,
			&i.Uuid,
			&i.Email,
			&i.Name,
			&i.Status,
			&i.CreatedAt,
			&i.UpdatedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const updateUserStatus = `-- name: UpdateUserStatus :one
UPDATE users SET status = $2 
WHERE uuid = $1 
RETURNING id, uuid, email, name, status, created_at, updated_at
`

type UpdateUserStatusParams struct {
	Uuid   pgtype.UUID `json:"uuid"`
	Status UsersStatus `json:"status"`
}

func (q *Queries) UpdateUserStatus(ctx context.Context, arg UpdateUserStatusParams) (User, error) {
	row := q.db.QueryRow(ctx, updateUserStatus, arg.Uuid, arg.Status)
	var i User
	err := row.Scan(
		&i.ID,
		&i.Uuid,
		&i.Email,
		&i.Name,
		&i.Status,
		&i.CreatedAt,
		&i.UpdatedAt,
	)
	return i, err
}