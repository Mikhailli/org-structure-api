package main

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/org-structure-api/internal/config"
	"github.com/org-structure-api/internal/handler"
	"github.com/org-structure-api/internal/repository"
	"github.com/org-structure-api/internal/service"
	"github.com/pressly/goose/v3"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

var embedMigrations embed.FS

func main() {
	// Инициализация логгера
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	// Загрузка конфигурации
	cfg := config.Load()

	// Подключение к БД
	db, err := connectDB(cfg.Database)
	if err != nil {
		logger.Error("failed to connect to database", slog.Any("error", err))
		os.Exit(1)
	}

	sqlDB, err := db.DB()
	if err != nil {
		logger.Error("failed to get sql.DB", slog.Any("error", err))
		os.Exit(1)
	}
	defer sqlDB.Close()

	// Запуск миграций
	if err := runMigrations(sqlDB); err != nil {
		logger.Error("failed to run migrations", slog.Any("error", err))
		os.Exit(1)
	}

	// Инициализация репозиториев
	deptRepo := repository.NewDepartmentRepository(db)
	empRepo := repository.NewEmployeeRepository(db)

	// Инициализация сервисов
	deptService := service.NewDepartmentService(deptRepo, empRepo)
	empService := service.NewEmployeeService(empRepo, deptRepo)

	// Инициализация хендлеров
	deptHandler := handler.NewDepartmentHandler(deptService, empService, logger)

	// Настройка роутера
	router := handler.NewRouter(deptHandler, logger)
	httpHandler := router.Setup()

	// Настройка HTTP сервера
	server := &http.Server{
		Addr:         ":" + cfg.Server.Port,
		Handler:      httpHandler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	done := make(chan bool)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-quit
		logger.Info("server is shutting down...")

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			logger.Error("could not gracefully shutdown the server", slog.Any("error", err))
		}
		close(done)
	}()

	logger.Info("server is starting", slog.String("port", cfg.Server.Port))
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("could not listen on port", slog.String("port", cfg.Server.Port), slog.Any("error", err))
		os.Exit(1)
	}

	<-done
	logger.Info("server stopped")
}

func connectDB(cfg config.DatabaseConfig) (*gorm.DB, error) {
	var db *gorm.DB
	var err error

	for range 30 {
		db, err = gorm.Open(postgres.Open(cfg.DSN()), &gorm.Config{
			Logger: gormlogger.Default.LogMode(gormlogger.Warn),
		})
		if err == nil {
			sqlDB, _ := db.DB()
			if sqlDB.Ping() == nil {
				return db, nil
			}
		}
		time.Sleep(time.Second)
	}

	return nil, fmt.Errorf("failed to connect to database after 30 attempts: %w", err)
}

func runMigrations(db *sql.DB) error {
	goose.SetBaseFS(embedMigrations)

	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("failed to set dialect: %w", err)
	}

	if err := goose.Up(db, "migrations"); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	return nil
}
