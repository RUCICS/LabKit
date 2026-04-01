package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	oauthcfg "labkit.local/apps/api/internal/config"
	httpapi "labkit.local/apps/api/internal/http"
	adminsvc "labkit.local/apps/api/internal/service/admin"
	authsvc "labkit.local/apps/api/internal/service/auth"
	labsvc "labkit.local/apps/api/internal/service/labs"
	boardsvc "labkit.local/apps/api/internal/service/leaderboard"
	personalsvc "labkit.local/apps/api/internal/service/personal"
	submissionsvc "labkit.local/apps/api/internal/service/submissions"
	websession "labkit.local/apps/api/internal/service/websession"
	"labkit.local/apps/api/internal/storage"
	db "labkit.local/packages/go/db"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	addr := os.Getenv("LABKIT_API_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}
	adminToken := os.Getenv("LABKIT_ADMIN_TOKEN")
	if adminToken == "" {
		log.Fatal("LABKIT_ADMIN_TOKEN is required")
	}
	artifactRoot := os.Getenv("LABKIT_ARTIFACT_ROOT")
	if artifactRoot == "" {
		artifactRoot = "artifacts"
	}

	pool, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()
	if err := pool.Ping(context.Background()); err != nil {
		log.Fatal(err)
	}

	oauthConfig, err := oauthcfg.ResolveOAuthConfigFromEnv()
	if err != nil {
		log.Fatal(err)
	}
	devMode := strings.EqualFold(strings.TrimSpace(os.Getenv("LABKIT_DEV_MODE")), "true")

	server := &http.Server{
		Addr: addr,
		Handler: httpapi.NewRouter(
			httpapi.WithAuthService(authsvc.NewService(authsvc.NewRepository(pool), authsvc.NewOAuthHTTPClient(oauthConfig), oauthConfig)),
			httpapi.WithLabsService(labsvc.NewService(db.New(pool))),
			httpapi.WithAdminService(adminsvc.NewService(adminsvc.NewRepository(pool))),
			httpapi.WithLeaderboardService(boardsvc.NewService(boardsvc.NewRepository(pool))),
			httpapi.WithPersonalService(personalsvc.NewService(personalsvc.NewRepository(pool))),
			httpapi.WithWebSessionService(websession.NewPersistentService(websession.NewRepository(pool))),
			httpapi.WithSubmissionsService(submissionsvc.NewService(submissionsvc.NewRepository(pool), storage.NewLocalArtifactStore(artifactRoot))),
			httpapi.WithAdminToken(adminToken),
			httpapi.WithDevMode(devMode),
		),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		errCh <- server.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
		return
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil && !errors.Is(err, context.Canceled) {
		log.Fatal(err)
	}

	if err := <-errCh; err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
