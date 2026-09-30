// Package testdb arranca nuestra imagen postgres (make la construye y exporta POSTGRES_TEST_IMAGE)
// para tests de integración. Solo lo importan tests.
package testdb

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// Start devuelve la URL de una base vacía; el contenedor se borra al terminar el test.
func Start(t *testing.T) string {
	t.Helper()
	image := os.Getenv("POSTGRES_TEST_IMAGE")
	if image == "" {
		t.Skip("POSTGRES_TEST_IMAGE no definida: corre `make test-go`")
	}
	pw := filepath.Join(t.TempDir(), "pw")
	if err := os.WriteFile(pw, []byte("test-pw"), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	ctr, err := testcontainers.Run(ctx, image,
		testcontainers.WithEnv(map[string]string{ // #nosec G101 -- helper de tests: POSTGRES_PASSWORD_FILE es una ruta, no una contraseña
			"POSTGRES_USER":          "u",
			"POSTGRES_DB":            "d",
			"POSTGRES_PASSWORD_FILE": "/run/secrets/pg",
		}),
		testcontainers.WithFiles(testcontainers.ContainerFile{
			HostFilePath: pw, ContainerFilePath: "/run/secrets/pg", FileMode: 0o644,
		}),
		testcontainers.WithExposedPorts("5432/tcp"),
		testcontainers.WithWaitStrategy(wait.ForHealthCheck().WithStartupTimeout(90*time.Second)),
	)
	testcontainers.CleanupContainer(t, ctr)
	if err != nil {
		t.Fatalf("arrancar postgres: %v", err)
	}
	hostPort, err := ctr.PortEndpoint(ctx, "5432/tcp", "")
	if err != nil {
		t.Fatal(err)
	}
	return "postgres://u:test-pw@" + hostPort + "/d?sslmode=disable"
}
