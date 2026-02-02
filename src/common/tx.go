package common

import (
	"context"

	"github.com/jmoiron/sqlx"
)

func WithTransaction(ctx context.Context, fn func(tx *sqlx.Tx) error) error {
	tx, err := instance.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}

	if err := fn(tx); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}
