package items

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PGRepository struct{ pool *pgxpool.Pool }

func NewPGRepository(pool *pgxpool.Pool) *PGRepository { return &PGRepository{pool: pool} }

func (r *PGRepository) List(ctx context.Context, limit int) ([]Item, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, title, description, created_by, created_at
		   FROM items ORDER BY created_at DESC, id DESC LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("items: listar: %w", err)
	}
	list, err := pgx.CollectRows(rows, scanItem)
	if err != nil {
		return nil, fmt.Errorf("items: listar: %w", err)
	}
	if list == nil {
		list = []Item{}
	}
	return list, nil
}

func (r *PGRepository) Create(ctx context.Context, in NewItem) (Item, error) {
	rows, err := r.pool.Query(ctx,
		`INSERT INTO items (title, description, created_by) VALUES ($1, $2, $3)
		 RETURNING id, title, description, created_by, created_at`,
		in.Title, in.Description, in.CreatedBy)
	if err != nil {
		return Item{}, fmt.Errorf("items: crear: %w", err)
	}
	it, err := pgx.CollectExactlyOneRow(rows, scanItem)
	if err != nil {
		return Item{}, fmt.Errorf("items: crear: %w", err)
	}
	return it, nil
}

func scanItem(row pgx.CollectableRow) (Item, error) {
	var it Item
	err := row.Scan(&it.ID, &it.Title, &it.Description, &it.CreatedBy, &it.CreatedAt)
	return it, err
}
