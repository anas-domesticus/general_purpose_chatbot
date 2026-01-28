package persistence

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/lewisedginton/general_purpose_chatbot/internal/cli/persistence/sqlc"
	"github.com/lewisedginton/general_purpose_chatbot/pkg/logger"
)

// UserRepository demonstrates youcolour repository patterns
type UserRepository struct {
	db      *pgxpool.Pool
	queries *sqlc.Queries
	logger  logger.Logger
}

// NewUserRepository creates a new user repository
func NewUserRepository(db *pgxpool.Pool, logger logger.Logger) *UserRepository {
	return &UserRepository{
		db:      db,
		queries: sqlc.New(db),
		logger:  logger,
	}
}

// WithTx creates a new repository instance with a transaction
func (r *UserRepository) WithTx(tx pgx.Tx) *UserRepository {
	return &UserRepository{
		db:      r.db,
		queries: r.queries.WithTx(tx),
		logger:  r.logger,
	}
}

// Domain types (simple examples)
type User struct {
	ID        int64     `json:"id"`
	UUID      uuid.UUID `json:"uuid"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type CreateUserRequest struct {
	Email  string `json:"email"`
	Name   string `json:"name"`
	Status string `json:"status,omitempty"`
}

// CreateUser demonstrates domain model conversion
func (r *UserRepository) CreateUser(ctx context.Context, req CreateUserRequest) (*User, error) {
	if req.Status == "" {
		req.Status = "active"
	}

	params := sqlc.CreateUserParams{
		Email:  req.Email,
		Name:   req.Name,
		Status: sqlc.UsersStatus(req.Status),
	}

	result, err := r.queries.CreateUser(ctx, params)
	if err != nil {
		r.logger.Error("failed to create user", logger.ErrorField(err), logger.StringField("email", req.Email))
		return nil, fmt.Errorf("create user: %w", err)
	}

	user, err := r.convertSQLCToUser(result)
	if err != nil {
		return nil, fmt.Errorf("convert created user: %w", err)
	}

	r.logger.Info("created user", logger.StringField("email", result.Email))
	return &user, nil
}

// GetUserByUUID demonstrates UUID handling
func (r *UserRepository) GetUserByUUID(ctx context.Context, userUUID uuid.UUID) (*User, error) {
	pgUUID := pgtype.UUID{
		Bytes: userUUID,
		Valid: true,
	}

	sqlcUser, err := r.queries.GetUserByUUID(ctx, pgUUID)
	if err != nil {
		r.logger.Error("failed to get user by UUID", logger.StringField("uuid", userUUID.String()), logger.ErrorField(err))
		return nil, fmt.Errorf("get user by UUID: %w", err)
	}

	user, err := r.convertSQLCToUser(sqlcUser)
	if err != nil {
		return nil, fmt.Errorf("convert user: %w", err)
	}

	return &user, nil
}

// ListUsers demonstrates simple listing
func (r *UserRepository) ListUsers(ctx context.Context, limit int32) ([]User, error) {
	if limit <= 0 {
		limit = 10
	}

	sqlcUsers, err := r.queries.ListUsers(ctx, limit)
	if err != nil {
		r.logger.Error("failed to list users", logger.ErrorField(err))
		return nil, fmt.Errorf("list users: %w", err)
	}

	users := make([]User, 0, len(sqlcUsers))
	for _, sqlcUser := range sqlcUsers {
		user, err := r.convertSQLCToUser(sqlcUser)
		if err != nil {
			return nil, fmt.Errorf("convert user: %w", err)
		}
		users = append(users, user)
	}

	return users, nil
}

// convertSQLCToUser demonstrates domain model conversion from youcolour
func (r *UserRepository) convertSQLCToUser(sqlcUser sqlc.User) (User, error) {
	userUUID, err := uuid.FromBytes(sqlcUser.Uuid.Bytes[:])
	if err != nil {
		return User{}, fmt.Errorf("convert UUID: %w", err)
	}

	return User{
		ID:        sqlcUser.ID,
		UUID:      userUUID,
		Email:     sqlcUser.Email,
		Name:      sqlcUser.Name,
		Status:    string(sqlcUser.Status),
		CreatedAt: sqlcUser.CreatedAt.Time,
		UpdatedAt: sqlcUser.UpdatedAt.Time,
	}, nil
}