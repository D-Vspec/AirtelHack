// Command seed loads the case CSVs into Postgres. Idempotent: drops and recreates the
// reference tables, then COPYs every file, all in one transaction.
//
//	go run ./cmd/seed -data ../data
package main

import (
	"context"
	_ "embed"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jackc/pgx/v5"

	"udyam/pkg/config"
)

//go:embed schema.sql
var schemaSQL string

//go:embed views.sql
var viewsSQL string

var files = []struct{ table, file string }{
	{"leads.smb_leads", "smb_leads.csv"},
	{"catalogue.product_catalogue", "product_catalogue.csv"},
	{"registry.gstin", "mock_gstin_registry.csv"},
	{"registry.udyam", "mock_udyam_registry.csv"},
}

func main() {
	dataDir := flag.String("data", "../data", "directory with the case CSVs")
	flag.Parse()
	if err := run(context.Background(), config.String("DATABASE_URL", "postgres://udyam:udyam@localhost:5432/udyam?sslmode=disable"), *dataDir); err != nil {
		fmt.Fprintln(os.Stderr, "seed:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, url, dataDir string) error {
	conn, err := pgx.Connect(ctx, url)
	if err != nil {
		return err
	}
	defer conn.Close(ctx)
	tx, err := conn.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, schemaSQL); err != nil {
		return fmt.Errorf("schema: %w", err)
	}
	for _, f := range files {
		fh, err := os.Open(filepath.Join(dataDir, f.file))
		if err != nil {
			return err
		}
		tag, err := tx.Conn().PgConn().CopyFrom(ctx, fh, "COPY "+f.table+" FROM STDIN WITH (FORMAT csv, HEADER true)")
		fh.Close()
		if err != nil {
			return fmt.Errorf("%s: %w", f.file, err)
		}
		fmt.Printf("%-30s %6d rows\n", f.table, tag.RowsAffected())
	}
	if _, err := tx.Exec(ctx, viewsSQL); err != nil {
		return fmt.Errorf("views: %w", err)
	}
	return tx.Commit(ctx)
}
