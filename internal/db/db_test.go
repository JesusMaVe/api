package db

import (
	"context"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"

	"github.com/JesusMaVe/api/internal/testdb"
)

func tableExists(t *testing.T, pool *pgxpool.Pool, name string) bool {
	t.Helper()
	var exists bool
	if err := pool.QueryRow(context.Background(), "select to_regclass($1) is not null", name).Scan(&exists); err != nil {
		t.Fatal(err)
	}
	return exists
}

func TestMigrateUpAndDown(t *testing.T) {
	ctx := context.Background()
	pool, err := Connect(ctx, testdb.Start(t))
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer pool.Close()

	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if !tableExists(t, pool, "items") {
		t.Fatal("tras Migrate debe existir la tabla items")
	}
	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("Migrate debe ser idempotente: %v", err)
	}

	sqlDB := stdlib.OpenDBFromPool(pool)
	defer func() { _ = sqlDB.Close() }()
	p, err := provider(sqlDB)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.DownTo(ctx, 0); err != nil {
		t.Fatalf("DownTo(0): %v", err)
	}
	if tableExists(t, pool, "items") {
		t.Fatal("tras bajar las migraciones la tabla items no debe existir")
	}
}

func TestConnectFailsWithoutLeakingPassword(t *testing.T) {
	_, err := Connect(context.Background(), "postgres://u:clave-secreta@127.0.0.1:1/d?sslmode=disable&connect_timeout=1")
	if err == nil {
		t.Fatal("se esperaba error de conexión")
	}
	if strings.Contains(err.Error(), "clave-secreta") {
		t.Fatalf("el error muestra la contraseña: %v", err)
	}
}
