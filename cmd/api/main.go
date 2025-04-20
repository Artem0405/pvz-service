package main

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net" // Для net.Listen (gRPC)
	"net/http"
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

	// Проверяем, что все необходимые переменные (кроме пароля) заданы
	if dbHost == "" || dbPort == "" || dbUser == "" || dbName == "" {
		return nil, fmt.Errorf("одна или несколько переменных окружения БД не установлены (DB_HOST, DB_PORT, DB_USER, DB_NAME)")
	}

	// Формируем строку подключения (Data Source Name)
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		dbUser, dbPassword, dbHost, dbPort, dbName)

	slog.Info("Подключение к базе данных...", "host", dbHost, "port", dbPort, "database", dbName)

	// Открываем соединение с БД
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		slog.Error("Не удалось инициализировать пул соединений с БД", "error", err)
		return nil, fmt.Errorf("не удалось открыть соединение с БД: %w", err)
	}

	// Проверяем реальное соединение с базой данных с таймаутом
	pingCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = db.PingContext(pingCtx)
	if err != nil {
		db.Close() // Закрываем нерабочий пул, если пинг не прошел
		slog.Error("Не удалось подключиться к БД (ping failed)", "error", err)
		return nil, fmt.Errorf("не удалось подключиться к БД (ping failed): %w", err)
	}

	slog.Info("Соединение с базой данных успешно установлено!")

	// --- НАСТРОЙКА ПУЛА СОЕДИНЕНИЙ ---
	// Устанавливаем лимит чуть меньше, чем max_connections в PostgreSQL (обычно 100).
	// Оставляем небольшой запас (~10-20%) для других возможных подключений.
	maxOpenConns := 80 // Начнем с 80
	db.SetMaxOpenConns(maxOpenConns)

	// Устанавливаем количество соединений, которые могут простаивать в пуле.
	db.SetMaxIdleConns(maxOpenConns) // Часто ставят равным MaxOpenConns

	// Опционально: Установить максимальное время жизни соединения.
	db.SetConnMaxLifetime(time.Hour)

	// Опционально: Установить максимальное время простоя.
	// db.SetConnMaxIdleTime(10 * time.Minute)

	slog.Info("Настроен пул соединений с БД", "MaxOpenConns", maxOpenConns, "MaxIdleConns", maxOpenConns)
	// --- КОНЕЦ НАСТРОЙКИ ПУЛА ---

	return db, nil // Возвращаем настроенный объект db
}

// --- ОСНОВНАЯ ФУНКЦИЯ ---
func main() {
	// 0. Настройка логгера slog
	logLevel := slog.LevelInfo // Уровень по умолчанию
	// Опционально: читаем уровень из переменной окружения для гибкости
	if levelStr := os.Getenv("LOG_LEVEL"); levelStr != "" {
		var lvl slog.Level
		if err := lvl.UnmarshalText([]byte(levelStr)); err == nil {
			logLevel = lvl
		} else {
			slog.Warn("Некорректный LOG_LEVEL, используется Info", "input", levelStr)
		}
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level:     logLevel,
		AddSource: true, // Добавляем источник (файл:строка) в логи
	}))
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
		apiPort = "8080" // Порт API по умолчанию
		slog.Warn("Переменная окружения PORT не установлена, используется порт по умолчанию", "port", apiPort)
	}
	apiAddr := ":" + apiPort

	metricsPort := os.Getenv("METRICS_PORT")
	if metricsPort == "" {
		metricsPort = "9000" // Порт метрик по умолчанию
		slog.Warn("Переменная окружения METRICS_PORT не установлена, используется порт по умолчанию", "port", metricsPort)
	}
	metricsAddr := ":" + metricsPort

	grpcPort := os.Getenv("GRPC_PORT")
	if grpcPort == "" {
		grpcPort = "3000" // Порт gRPC по умолчанию
		slog.Warn("Переменная окружения GRPC_PORT не установлена, используется порт по умолчанию", "port", grpcPort)
	}
	grpcListenAddr := ":" + grpcPort

	// 2. Инициализация зависимостей
	db, err := initDB() // initDB теперь возвращает настроенный пул
	if err != nil {
		slog.Error("Ошибка инициализации базы данных", "error", err) // Добавим логирование ошибки из initDB
		os.Exit(1)
	}
	// db будет закрыт в defer ниже, после инициализации всех зависимостей

	// Репозитории
	pvzRepo := postgres.NewPVZRepo(db)
	receptionRepo := postgres.NewReceptionRepo(db)
	userRepo := postgres.NewUserRepo(db)
	slog.Info("Репозитории инициализированы (PVZ, Reception, User).")

	// Сервисы
	authService := service.NewAuthService(jwtSecret, userRepo)
	pvzService := service.NewPVZService(pvzRepo, receptionRepo)
	receptionService := service.NewReceptionService(receptionRepo)
	slog.Info("Сервисы инициализированы (Auth, PVZ, Reception).")

	// Главный обработчик API (хендлер)
	apiHandler := api.NewHandler(db, authService, pvzService, receptionService)
	slog.Info("API Handler инициализирован.")

	// Отложенное закрытие БД - выполняется при выходе из main
	defer func() {
		slog.Info("Закрытие пула соединений с БД...")
		if err := db.Close(); err != nil {
			slog.Error("Ошибка при закрытии пула соединений с БД", "error", err)
		} else {
			slog.Info("Пул соединений с БД успешно закрыт.")
		}
	}()

	// 3. Настройка роутера chi для HTTP API
	r := chi.NewRouter()
	// Middleware в порядке их выполнения
	r.Use(middleware.RequestID)                 // Добавляет RequestID в контекст
	r.Use(middleware.RealIP)                    // Определяет реальный IP клиента (учитывая прокси)
	r.Use(api.SlogMiddleware(logger))           // Логирование запросов с использованием slog
	r.Use(middleware.Recoverer)                 // Перехватывает паники и возвращает 500
	r.Use(middleware.Timeout(60 * time.Second)) // Устанавливает таймаут на обработку запроса
	r.Use(api.PrometheusMiddleware)             // Собирает метрики Prometheus для HTTP запросов
	slog.Info("Роутер и базовые middleware для HTTP API настроены.")

	// 4. Регистрация HTTP маршрутов
	slog.Info("Регистрация HTTP маршрутов...")
	// Публичные
	r.Get("/health", apiHandler.HandleHealthCheck)
	r.Post("/dummyLogin", apiHandler.HandleDummyLogin)
	r.Post("/register", apiHandler.HandleRegister)
	r.Post("/login", apiHandler.HandleLogin)

	// Маршрут для метрик Prometheus
	// Важно: он должен быть вне группы с AuthMiddleware!
	r.Handle("/metrics", promhttp.Handler())

	// Защищенные маршруты
	r.Group(func(r chi.Router) {
		r.Use(api.AuthMiddleware(authService)) // Middleware аутентификации

		// Маршруты, доступные всем аутентифицированным пользователям
		r.Get("/pvz", apiHandler.HandleListPVZ)
		r.Post("/receptions", apiHandler.HandleInitiateReception)
		r.Post("/products", apiHandler.HandleAddProduct)
		r.Post("/pvz/{pvzId}/delete_last_product", apiHandler.HandleDeleteLastProduct)
		r.Post("/pvz/{pvzId}/close_last_reception", apiHandler.HandleCloseLastReception)

		// Маршруты, доступные только модераторам
		r.Group(func(r chi.Router) {
			r.Use(api.RoleMiddleware(domain.RoleModerator)) // Middleware проверки роли
			r.Post("/pvz", apiHandler.HandleCreatePVZ)
		})
	})
	slog.Info("HTTP маршруты успешно зарегистрированы.")

	// Канал для получения ошибок из горутин серверов
	errChan := make(chan error, 3) // Буфер на 3 ошибки (API, Metrics, gRPC)

	// 5. Запуск HTTP-сервера для метрик Prometheus (в горутине)
	go func() {
		metricsServer := &http.Server{
			Addr:              metricsAddr,
			Handler:           http.NotFoundHandler(), // Основной обработчик не нужен, т.к. /metrics регистрируется выше
			ReadHeaderTimeout: 5 * time.Second,        // Добавим таймауты
		}
		// Создаем отдельный ServeMux ТОЛЬКО для метрик,
		// чтобы middleware основного роутера не применялись к /metrics
		metricsMux := http.NewServeMux()
		metricsMux.Handle("/metrics", promhttp.Handler()) // Регистрируем обработчик метрик
		metricsServer.Handler = metricsMux                // Используем этот mux

		slog.Info("Starting metrics server", "address", metricsServer.Addr)
		// Отправляем ошибку в канал, если сервер упал
		errChan <- metricsServer.ListenAndServe()
	}()

	// 6. Запуск gRPC Сервера (в горутине)
	go func() {
		lis, err := net.Listen("tcp", grpcListenAddr)
		if err != nil {
			slog.Error("Failed to listen for gRPC", "address", grpcListenAddr, "error", err)
			errChan <- fmt.Errorf("gRPC listen error: %w", err) // Оборачиваем ошибку
			return
		}
		// Отложенное закрытие листенера
		defer func() {
			if err := lis.Close(); err != nil {
				slog.Error("Error closing gRPC listener", "error", err)
			} else {
				slog.Info("gRPC listener closed")
			}
		}()

		pvzGrpcServerImpl := grpcServer.NewPVZServer(pvzRepo)   // Создаем реализацию сервиса
		grpcSrv := grpc.NewServer()                             // Создаем gRPC сервер
		pb.RegisterPVZServiceServer(grpcSrv, pvzGrpcServerImpl) // Регистрируем сервис

		slog.Info("Starting gRPC server", "address", lis.Addr().String())
		// Отправляем ошибку в канал, если сервер упал
		err = grpcSrv.Serve(lis)
		if err != nil {
			slog.Error("gRPC server failed", "error", err)
			errChan <- fmt.Errorf("gRPC serve error: %w", err)
		} else {
			// Если Serve завершился без ошибки (например, через GracefulStop),
			// можно отправить nil или специальный маркер, если нужно.
			// Пока просто логируем.
			slog.Info("gRPC server stopped gracefully")
		}
	}()

	// 7. Запуск основного HTTP-сервера API (в горутине)
	go func() {
		httpServer := &http.Server{
			Addr:              apiAddr,
			Handler:           r,                // Используем настроенный chi роутер
			ReadTimeout:       10 * time.Second, // Добавим таймауты
			ReadHeaderTimeout: 5 * time.Second,
			WriteTimeout:      10 * time.Second,
			IdleTimeout:       120 * time.Second,
		}
		slog.Info("Starting API server", "address", httpServer.Addr)
		// Отправляем ошибку в канал, если сервер упал
		err := httpServer.ListenAndServe()
		// ListenAndServe всегда возвращает не-nil ошибку
		if err != http.ErrServerClosed {
			slog.Error("API server failed", "error", err)
			errChan <- fmt.Errorf("API server error: %w", err)
		} else {
			slog.Info("API server stopped gracefully")
		}
	}()

	// 8. Ожидание ошибки от любого из серверов
	slog.Info("Application started. Waiting for errors...")
	serverErr := <-errChan // Блокируемся до получения первой ошибки
	slog.Error("Shutting down due to server error", "error", serverErr)

	// Здесь можно было бы добавить логику graceful shutdown для *остальных*
	// серверов перед выходом, но для простоты пока опускаем.

	// Выход с ошибкой (если сервер упал не штатно)
	os.Exit(1)

	// Этот код ниже не будет достигнут из-за os.Exit(1)
	// slog.Info("PVZ Service stopped.")
}
