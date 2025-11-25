package postgres

import (
	"context"
	"database/sql"
	"fmt"
)

// TxManager управляет транзакциями базы данных
type TxManager struct {
	db *sql.DB
}

// NewTxManager создаёт новый менеджер транзакций
func NewTxManager(db *sql.DB) *TxManager {
	return &TxManager{db: db}
}

// WithinTransaction выполняет функцию внутри транзакции
func (tm *TxManager) WithinTransaction(ctx context.Context, fn func(tx *sql.Tx) error) error {
	tx, err := tm.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
	}()

	if err := fn(tx); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("transaction error: %v, rollback error: %w", err, rbErr)
		}
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}
