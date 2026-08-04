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

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/leftmy/planetarium-server/config"
	"github.com/leftmy/planetarium-server/migrations"
)

func main() {
	flag.Parse()

	command := flag.Arg(0)
	if command == "" {
		command = "up"
	}

	cfg, err := config.InitConfig()
	if err != nil {
		log.Fatalf("read config: %v", err)
	}

	db, err := sql.Open("pgx", cfg.Postgres.BuildURL())
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
