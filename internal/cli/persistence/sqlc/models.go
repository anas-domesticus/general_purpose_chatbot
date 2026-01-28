// Simple SQLC models

package sqlc

import (
	"database/sql/driver"
	"fmt"
	"github.com/jackc/pgx/v5/pgtype"
)

type UsersStatus string

const (
	UsersStatusActive   UsersStatus = "active"
	UsersStatusInactive UsersStatus = "inactive"
)

func (e *UsersStatus) Scan(src interface{}) error {
	switch s := src.(type) {
	case string:
		*e = UsersStatus(s)
	case []byte:
		*e = UsersStatus(s)
	default:
		return fmt.Errorf("unsupported Scan, storing driver.Value type %T into type %T", src, e)
	}
	return nil
}

func (e UsersStatus) Value() (driver.Value, error) {
	return string(e), nil
}

type User struct {
	ID        int64              `json:"id"`
	Uuid      pgtype.UUID        `json:"uuid"`
	Email     string             `json:"email"`
	Name      string             `json:"name"`
	Status    UsersStatus        `json:"status"`
	CreatedAt pgtype.Timestamptz `json:"created_at"`
	UpdatedAt pgtype.Timestamptz `json:"updated_at"`
}