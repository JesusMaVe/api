package items

import (
	"context"
	"errors"
	"testing"

	"github.com/JesusMaVe/api/internal/db"
	"github.com/JesusMaVe/api/internal/testdb"
)

func TestPGRepository(t *testing.T) {
	ctx := context.Background()
	pool, err := db.Connect(ctx, testdb.Start(t))
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	repo := NewPGRepository(pool)

	empty, err := repo.List(ctx, 10)
	if err != nil || empty == nil || len(empty) != 0 {
		t.Fatalf("lista vacía: %v %v (debe ser [] y no nil)", empty, err)
	}

	first, err := repo.Create(ctx, NewItem{Title: "Zelda", Description: "BOTW", CreatedBy: "alice"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if first.ID == 0 || first.CreatedAt.IsZero() || first.Title != "Zelda" || first.CreatedBy != "alice" {
		t.Fatalf("item creado: %+v", first)
	}
	second, _ := repo.Create(ctx, NewItem{Title: "Tenis", CreatedBy: "bob"})
	third, _ := repo.Create(ctx, NewItem{Title: "Chamarra", CreatedBy: "alice"})

	got, err := repo.List(ctx, 2)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 2 || got[0].ID != third.ID || got[1].ID != second.ID {
		t.Fatalf("debe devolver los 2 más recientes primero: %+v", got)
	}

	// Update y Delete: solo el dueño; otro usuario recibe ErrForbidden y un id inexistente ErrNotFound.
	edited, err := repo.Update(ctx, first.ID, "alice", Changes{Title: "Zelda TOTK", Description: "secuela"})
	if err != nil || edited.Title != "Zelda TOTK" || edited.Description != "secuela" || edited.CreatedBy != "alice" || !edited.CreatedAt.Equal(first.CreatedAt) {
		t.Fatalf("Update: %+v %v", edited, err)
	}
	if _, err := repo.Update(ctx, first.ID, "bob", Changes{Title: "hackeado"}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("Update de otro usuario: %v", err)
	}
	if _, err := repo.Update(ctx, 999999, "alice", Changes{Title: "x"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Update inexistente: %v", err)
	}
	if err := repo.Delete(ctx, second.ID, "alice"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("Delete de otro usuario: %v", err)
	}
	if err := repo.Delete(ctx, first.ID, "alice"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if err := repo.Delete(ctx, first.ID, "alice"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Delete repetido: %v", err)
	}
	left, _ := repo.List(ctx, 10)
	if len(left) != 2 || left[0].ID != third.ID || left[1].ID != second.ID {
		t.Fatalf("tras borrar quedan los otros dos, y el de bob intacto: %+v", left)
	}
}
