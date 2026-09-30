package items

import (
	"context"
	"errors"
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

// Update y Delete filtran por id Y dueño en la misma sentencia (sin carrera entre leer y escribir).
// Si no tocan ninguna fila, missing distingue "no existe" de "es de otro".
func (r *PGRepository) Update(ctx context.Context, id int64, owner string, c Changes) (Item, error) {
	rows, err := r.pool.Query(ctx,
		`UPDATE items SET title = $3, description = $4 WHERE id = $1 AND created_by = $2
		 RETURNING id, title, description, created_by, created_at`,
		id, owner, c.Title, c.Description)
	if err != nil {
		return Item{}, fmt.Errorf("items: editar: %w", err)
	}
	it, err := pgx.CollectExactlyOneRow(rows, scanItem)
	if errors.Is(err, pgx.ErrNoRows) {
		return Item{}, r.missing(ctx, id)
	}
	if err != nil {
		return Item{}, fmt.Errorf("items: editar: %w", err)
	}
	return it, nil
}

func (r *PGRepository) Delete(ctx context.Context, id int64, owner string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM items WHERE id = $1 AND created_by = $2`, id, owner)
	if err != nil {
		return fmt.Errorf("items: borrar: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return r.missing(ctx, id)
	}
	return nil
}

func (r *PGRepository) missing(ctx context.Context, id int64) error {
	var exists bool
	if err := r.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM items WHERE id = $1)`, id).Scan(&exists); err != nil {
		return fmt.Errorf("items: buscar: %w", err)
	}
	if exists {
		return ErrForbidden
	}
	return ErrNotFound
}

func scanItem(row pgx.CollectableRow) (Item, error) {
	var it Item
	err := row.Scan(&it.ID, &it.Title, &it.Description, &it.CreatedBy, &it.CreatedAt)
	return it, err
}
