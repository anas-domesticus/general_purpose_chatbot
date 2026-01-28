package sqlc

import (
	"context"
	"github.com/jackc/pgx/v5/pgtype"
)

type Querier interface {
	CreateUser(ctx context.Context, arg CreateUserParams) (User, error)
	GetUserByUUID(ctx context.Context, uuid pgtype.UUID) (User, error)
	GetUserByEmail(ctx context.Context, email string) (User, error)
	ListUsers(ctx context.Context, limit int32) ([]User, error)
	UpdateUserStatus(ctx context.Context, arg UpdateUserStatusParams) (User, error)
}

var _ Querier = (*Queries)(nil)