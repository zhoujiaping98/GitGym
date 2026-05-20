package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"gitgym/services/api/internal/config"
	httpx "gitgym/services/api/internal/http"
	"gitgym/services/api/internal/runner"
	"gitgym/services/api/internal/service"
	"gitgym/services/api/internal/store"
)

func main() {
	cfg := config.LoadRuntime()
	runnerClient := runner.NewClient(cfg.RunnerBaseURL, http.DefaultClient)

	var (
		authStore service.UserStore
		practice  service.PracticeService
		dbCloser  interface{ Close() error }
	)

	if strings.TrimSpace(cfg.MySQLDSN) != "" {
		db, err := store.OpenMySQL(cfg.MySQLDSN)
		if err != nil {
			log.Fatal(err)
		}
		dbCloser = db
		authStore = store.NewMySQLStore(db)
	}
	if dbCloser != nil {
		defer func() {
			_ = dbCloser.Close()
		}()
	}

	if practiceStore, ok := authStore.(service.PracticeSessionStore); ok {
		practice = service.NewPracticeService(
			practiceStore,
			runnerClient,
			service.NewFallbackPracticeCatalog(),
			time.Now,
		)
	}
	if practice == nil {
		practice = service.NewPracticeService(
			service.NewInMemoryPracticeSessionStore(),
			runnerClient,
			service.NewFallbackPracticeCatalog(),
			time.Now,
		)
	}

	router := httpx.NewRouter(httpx.Dependencies{
		AuthStore:       authStore,
		AuthConfig:      cfg,
		RunnerClient:    runnerClient,
		PracticeService: practice,
	})

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if practice != nil {
		go service.StartPracticeSessionExpiryLoop(ctx, practice, time.Minute, log.Default())
	}

	server := &http.Server{
		Addr:    ":8080",
		Handler: router,
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
