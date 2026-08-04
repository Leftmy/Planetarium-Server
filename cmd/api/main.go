package main

import (
	"context"
	"log"
	"net/http"

	"github.com/leftmy/planetarium-server/config"
	"github.com/leftmy/planetarium-server/internal/adapter/postgres"
	httptransport "github.com/leftmy/planetarium-server/internal/transport/http"
	"github.com/leftmy/planetarium-server/internal/user/login"
	"github.com/leftmy/planetarium-server/internal/user/register"
	"github.com/leftmy/planetarium-server/pkg/httpserver"
	"github.com/leftmy/planetarium-server/pkg/token"
)

func main() {
	cfg, err := config.InitConfig()
	if err != nil {
		log.Fatalf("read config: %v", err)
	}

	// Connecting here rather than lazily means a wrong DSN stops the process
	// at startup instead of surfacing as a failed request later.
	db, err := postgres.New(context.Background(), cfg.Postgres)
	if err != nil {
		log.Fatalf("connect to database: %v", err)
	}
	defer db.Close()

	users := postgres.NewUserRepo(db)
	identities := postgres.NewUserIdentityRepo(db)
	sessions := postgres.NewRefreshTokenRepo(db)

	if cfg.Auth.JWTSecret == config.DevJWTSecret {
		log.Println("WARN: JWT_SECRET is not set, using the published development secret. " +
			"Anyone can mint valid access tokens. Set JWT_SECRET before deploying.")
	}

	tokens := token.NewManager(cfg.Auth.JWTSecret, cfg.Auth.AccessTTL, cfg.Auth.RefreshTTL)

	handlers := httptransport.Handlers{
		Register: register.NewHandler(register.NewUsecase(db, users, identities)),
		Login:    login.NewHandler(login.NewUsecase(db, users, sessions, tokens)),
	}

	mux := http.NewServeMux()
	httptransport.SetupRoutes(mux, handlers)

	httpserver.Start(mux, cfg.AppPort)
}
