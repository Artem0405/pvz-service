package main

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net" // Для net.Listen (gRPC)
	"net/http"

	// --- ДОБАВЛЕНО: Импорт _ "net/http/pprof" ---
	_ "net/http/pprof" // Регистрирует обработчики pprof в http.DefaultServeMux
	"os"
	"time"

	// --- Внешние зависимости ---
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	_ "github.com/jackc/pgx/v5/stdlib"                        // Драйвер PostgreSQL (важен _ импорт)
	"github.com/prometheus/client_golang/prometheus/promhttp" // Обработчик для /metrics
	"google.golang.org/grpc"                                  // Для gRPC сервера

	// --- Внутренние пакеты ---
	"github.com/Artem0405/pvz-service/internal/api"                 // HTTP обработчики и middleware
	"github.com/Artem0405/pvz-service/internal/domain"              // Для констант ролей в роутере
	grpcServer "github.com/Artem0405/pvz-service/internal/grpc"     // Наш gRPC сервер
	_ "github.com/Artem0405/pvz-service/internal/metrics"           // Импорт для регистрации метрик (побочный эффект)
	"github.com/Artem0405/pvz-service/internal/repository/postgres" // Реализация репозиториев
	"github.com/Artem0405/pvz-service/internal/service"             // Сервисы бизнес-логики
	pb "github.com/Artem0405/pvz-service/pkg/pvz/v1"                // Сгенерированный код protobuf/grpc
)

// initDB инициализирует и проверяет соединение с базой данных.
// Использует slog для логирования.
func initDB() (*sql.DB, error) {
	dbHost := os.Getenv("DB_HOST")
	dbPort := os.Getenv("DB_PORT")
	dbUser := os.Getenv("DB_USER")
	dbPassword := os.Getenv("DB_PASSWORD")
	dbName := os.Getenv("DB_NAME")

	if dbHost == "" || dbPort == "" || dbUser == "" || dbName == "" {
		return nil, fmt.Errorf("одна или несколько переменных окружения БД не установлены (DB_HOST, DB_PORT, DB_USER, DB_NAME)")
	}
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", dbUser, dbPassword, dbHost, dbPort, dbName)
	slog.Info("Подключение к базе данных...", "host", dbHost, "port", dbPort, "database", dbName)
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		slog.Error("Не удалось инициализировать пул соединений с БД", "error", err)
		return nil, fmt.Errorf("не удалось открыть соединение с БД: %w", err)
	}
	pingCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = db.PingContext(pingCtx)
	if err != nil {
		db.Close()
		slog.Error("Не удалось подключиться к БД (ping failed)", "error", err)
		return nil, fmt.Errorf("не удалось подключиться к БД (ping failed): %w", err)
	}
	slog.Info("Соединение с базой данных успешно установлено!")
	maxOpenConns := 80
	db.SetMaxOpenConns(maxOpenConns)
	db.SetMaxIdleConns(maxOpenConns)
	db.SetConnMaxLifetime(time.Hour)
	slog.Info("Настроен пул соединений с БД", "MaxOpenConns", maxOpenConns, "MaxIdleConns", maxOpenConns)
	return db, nil
}

// --- ОСНОВНАЯ ФУНКЦИЯ ---
func main() {
	// 0. Настройка логгера slog
	logLevel := slog.LevelInfo
	if levelStr := os.Getenv("LOG_LEVEL"); levelStr != "" {
		var lvl slog.Level
		if err := lvl.UnmarshalText([]byte(levelStr)); err == nil {
			logLevel = lvl
		} else {
			slog.Warn("Некорректный LOG_LEVEL, используется Info", "input", levelStr)
		}
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel, AddSource: true}))
	slog.SetDefault(logger)
	slog.Info("PVZ Service starting...")

	// 1. Чтение конфигурации
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		slog.Error("Переменная окружения JWT_SECRET не установлена!")
		os.Exit(1)
	}
	apiPort := os.Getenv("PORT")
	if apiPort == "" {
		apiPort = "8080"
		slog.Warn("Переменная окружения PORT не установлена, используется порт по умолчанию", "port", apiPort)
	}
	apiAddr := ":" + apiPort
	metricsPort := os.Getenv("METRICS_PORT")
	if metricsPort == "" {
		metricsPort = "9000"
		slog.Warn("Переменная окружения METRICS_PORT не установлена, используется порт по умолчанию", "port", metricsPort)
	}
	metricsAddr := ":" + metricsPort
	grpcPort := os.Getenv("GRPC_PORT")
	if grpcPort == "" {
		grpcPort = "3000"
		slog.Warn("Переменная окружения GRPC_PORT не установлена, используется порт по умолчанию", "port", grpcPort)
	}
	grpcListenAddr := ":" + grpcPort

	// 2. Инициализация зависимостей
	db, err := initDB()
	if err != nil {
		slog.Error("Ошибка инициализации базы данных", "error", err)
		os.Exit(1)
	}
	defer func() {
		slog.Info("Закрытие пула соединений с БД...")
		if err := db.Close(); err != nil {
			slog.Error("Ошибка при закрытии пула соединений с БД", "error", err)
		} else {
			slog.Info("Пул соединений с БД успешно закрыт.")
		}
	}()
	slog.Info("Пул соединений с БД инициализирован.")

	pvzRepo := postgres.NewPVZRepo(db)
	receptionRepo := postgres.NewReceptionRepo(db)
	userRepo := postgres.NewUserRepo(db)
	slog.Info("Репозитории инициализированы (PVZ, Reception, User).")

	authService := service.NewAuthService(jwtSecret, userRepo)
	pvzService := service.NewPVZService(pvzRepo, receptionRepo)
	receptionService := service.NewReceptionService(receptionRepo)
	slog.Info("Сервисы инициализированы (Auth, PVZ, Reception).")

	apiHandler := api.NewHandler(db, authService, pvzService, receptionService)
	slog.Info("API Handler инициализирован.")

	// 3. Настройка роутера chi для HTTP API
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(api.SlogMiddleware(logger))
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))
	r.Use(api.PrometheusMiddleware) // Prometheus middleware должен идти до Timeout? Проверить порядок.

	// --- ДОБАВЛЕНО: Регистрация pprof обработчиков ---
	// Монтируем стандартные обработчики pprof на /debug/pprof
	// Важно: НЕ оборачивайте эту группу в AuthMiddleware!
	r.Mount("/debug", middleware.Profiler()) // Предоставляет /debug/pprof/* пути
	slog.Info("Pprof handlers mounted on /debug/pprof")
	// -----------------------------------------------

	slog.Info("Роутер и базовые middleware для HTTP API настроены.")

	// 4. Регистрация HTTP маршрутов
	slog.Info("Регистрация HTTP маршрутов...")
	r.Get("/health", apiHandler.HandleHealthCheck)
	r.Post("/dummyLogin", apiHandler.HandleDummyLogin)
	r.Post("/register", apiHandler.HandleRegister)
	r.Post("/login", apiHandler.HandleLogin)

	// Маршрут для метрик Prometheus - оставляем, т.к. он нужен для Prometheus сервера
	r.Handle("/metrics", promhttp.Handler())

	r.Group(func(r chi.Router) {
		r.Use(api.AuthMiddleware(authService))
		r.Get("/pvz", apiHandler.HandleListPVZ)
		r.Post("/receptions", apiHandler.HandleInitiateReception)
		r.Post("/products", apiHandler.HandleAddProduct)
		r.Post("/pvz/{pvzId}/delete_last_product", apiHandler.HandleDeleteLastProduct)
		r.Post("/pvz/{pvzId}/close_last_reception", apiHandler.HandleCloseLastReception)
		r.Group(func(r chi.Router) {
			r.Use(api.RoleMiddleware(domain.RoleModerator))
			r.Post("/pvz", apiHandler.HandleCreatePVZ)
		})
	})
	slog.Info("HTTP маршруты успешно зарегистрированы.")

	errChan := make(chan error, 3)

	// 5. Запуск HTTP-сервера для метрик Prometheus (в горутине)
	//    Теперь он не нужен, если /metrics доступен через основной роутер.
	//    НО: Оставляем его, если хотим метрики на отдельном порту.
	//    Если /metrics обрабатывается основным роутером r, эту горутину можно удалить.
	//    Я оставлю ее, так как у вас был отдельный metricsPort.
	//    НО УДАЛИЛ ИЗ НЕГО r.Handle("/metrics",...), т.к. он уже есть в основном роутере.
	go func() {
		// --- УБРАЛИ регистрацию /metrics здесь, она теперь в роутере r ---
		metricsMux := http.NewServeMux()
		// Можно оставить его пустым или добавить свой /health для сервера метрик
		metricsMux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("Metrics server auxiliary handler"))
		}) // Добавим заглушку

		metricsServer := &http.Server{
			Addr:              metricsAddr,
			Handler:           metricsMux, // Используем пустой mux, т.к. /metrics в основном роутере
			ReadHeaderTimeout: 5 * time.Second,
		}
		slog.Info("Starting auxiliary metrics server", "address", metricsServer.Addr)
		err := metricsServer.ListenAndServe()
		if err != http.ErrServerClosed { // Логируем только реальные ошибки
			slog.Error("Metrics server failed", "error", err)
			errChan <- fmt.Errorf("metrics server error: %w", err)
		} else {
			slog.Info("Metrics server stopped gracefully.")
		}
	}()

	// 6. Запуск gRPC Сервера (в горутине) - без изменений
	go func() {
		lis, err := net.Listen("tcp", grpcListenAddr)
		if err != nil {
			slog.Error("Failed to listen for gRPC", "address", grpcListenAddr, "error", err)
			errChan <- fmt.Errorf("gRPC listen error: %w", err)
			return
		}
		defer func() { /* ... закрытие lis ... */ }()

		pvzGrpcServerImpl := grpcServer.NewPVZServer(pvzRepo)
		grpcSrv := grpc.NewServer()
		pb.RegisterPVZServiceServer(grpcSrv, pvzGrpcServerImpl)

		slog.Info("Starting gRPC server", "address", lis.Addr().String())
		err = grpcSrv.Serve(lis)
		if err != nil {
			slog.Error("gRPC server failed", "error", err)
			errChan <- fmt.Errorf("gRPC serve error: %w", err)
		} else {
			slog.Info("gRPC server stopped gracefully")
		}
	}()

	// 7. Запуск основного HTTP-сервера API (в горутине) - без изменений
	go func() {
		httpServer := &http.Server{
			Addr:              apiAddr,
			Handler:           r, // Используем chi роутер с pprof и /metrics
			ReadTimeout:       10 * time.Second,
			ReadHeaderTimeout: 5 * time.Second,
			WriteTimeout:      10 * time.Second,
			IdleTimeout:       120 * time.Second,
		}
		slog.Info("Starting API server", "address", httpServer.Addr)
		err := httpServer.ListenAndServe()
		if err != http.ErrServerClosed {
			slog.Error("API server failed", "error", err)
			errChan <- fmt.Errorf("API server error: %w", err)
		} else {
			slog.Info("API server stopped gracefully")
		}
	}()

	// 8. Ожидание ошибки от любого из серверов - без изменений
	slog.Info("Application started. Waiting for errors...")
	serverErr := <-errChan
	slog.Error("Shutting down due to server error", "error", serverErr)
	os.Exit(1)
}
