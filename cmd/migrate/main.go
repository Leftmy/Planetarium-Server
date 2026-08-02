// Command migrate applies the embedded SQL migrations.
//
// It is deliberately a separate binary rather than a step in the API startup:
// several replicas starting at once would otherwise race to migrate the same
// database.
//
//	go run ./cmd/migrate up
//	go run ./cmd/migrate status
//	go run ./cmd/migrate down
package main

import (
	"context"
	"database/sql"
	"flag"
	"log"
	"os"
	"strconv"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/leftmy/planetarium-server/internal/adapter/postgres"
	"github.com/leftmy/planetarium-server/migrations"
)

func main() {
	flag.Parse()

	command := flag.Arg(0)
	if command == "" {
		command = "up"
	}

	cfg, err := configFromEnv()
	if err != nil {
		log.Fatalf("read config: %v", err)
	}

	db, err := sql.Open("pgx", cfg.BuildURL())
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Printf("close database: %v", err)
		}
	}()

	goose.SetBaseFS(migrations.FS)

	if err := goose.SetDialect("postgres"); err != nil {
		log.Fatalf("set dialect: %v", err)
	}

	if err := goose.RunContext(context.Background(), command, db, ".", flag.Args()[1:]...); err != nil {
		log.Fatalf("goose %s: %v", command, err)
	}
}

// configFromEnv reads the same DB_* variables as .env.example. Once
// config.InitConfig parses the environment for real, this should call it
// instead of duplicating the lookups.
func configFromEnv() (postgres.Config, error) {
	port, err := strconv.ParseUint(env("DB_PORT", "5432"), 10, 16)
	if err != nil {
		return postgres.Config{}, err
	}

	return postgres.Config{
		User:     env("DB_USER", "postgres"),
		Password: env("DB_PASSWORD", "secret"),
		Host:     env("DB_HOST", "localhost"),
		Port:     uint(port),
		Name:     env("DB_NAME", "planetarium_db"),
	}, nil
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}

	return fallback
}
