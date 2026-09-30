package items

import (
	"context"
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
}
