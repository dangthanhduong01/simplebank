package db

import (
	"database/sql"
)

const (
	ForeignKeyViolation = "23503"
	UniqueViolation     = "23505"
)

var ErrRecordNotFound = sql.ErrNoRows

// var ErrUniqueViolation = &pgconn.PgError{
// 	Code: UniqueViolation,
// }

func ErrorCode(err error) string {
	// var pgErr *pgconn.PgError
	// if errors.As(err, &pgErr) {
	// 	return pgErr.Code
	// }
	return ""
}
