# PVZ Service (github.com/Artem0405/pvz-service)

## Description

This project implements a backend service in Go for managing PVZs (Pickup Points / Пункты Выдачи Заказов), their associated Receptions (Приемки), and Products (Товары). It features user authentication and authorization with role-based access control, provides both RESTful HTTP and gRPC APIs, and includes monitoring capabilities with Prometheus and Grafana.

## Features

*   **Authentication & Authorization:**
    *   User registration (`/register`) with roles (employee, moderator).
    *   User login (`/login`) returning a JWT token.
    *   JWT-based authentication (Bearer Token) for protected endpoints.
    *   Dummy login (`/dummyLogin`) for generating test tokens.
    *   Password hashing using bcrypt.
    *   Role-based access control (e.g., moderators create PVZs, employees manage receptions/products).
*   **PVZ (Pickup Point) Management:**
    *   Create new PVZs (POST `/pvz`, requires moderator role).
        *   Mandatory `city` field (Valid: Москва, Санкт-Петербург, Казань).
    *   List PVZs (GET `/pvz`).
        *   Includes details about associated receptions and products.
        *   Implements **Keyset Pagination** for efficient loading of large datasets.
        *   Supports optional date filtering (`startDate`, `endDate`) for receptions within the listed PVZs.
*   **Reception (Приемка) Management:**
    *   Initiate a new reception for a specific PVZ (POST `/receptions`). A PVZ can only have one reception `in_progress` at a time.
    *   Add products (POST `/products`) to the last open reception of a PVZ.
        *   Supported product types: `электроника`, `одежда`, `обувь`.
    *   Delete the last added product (LIFO) from the open reception (POST `/pvz/{pvzId}/delete_last_product`).
    *   Close the last open reception for a PVZ (POST `/pvz/{pvzId}/close_last_reception`).
*   **gRPC API:**
    *   Provides a gRPC interface (`PVZService`) for listing all PVZs (`GetPVZList`). (Pagination not implemented in gRPC based on the code).
*   **Monitoring & Observability:**
    *   **Logging:** Structured logging using Go's standard `log/slog`.
    *   **Metrics:** Prometheus metrics exposed at `/metrics` (HTTP request duration/count, custom business metrics like PVZs created, receptions initiated, products added).
    *   **Health Check:** `/health` endpoint to check service and database connectivity.
    *   **Profiling:** `net/http/pprof` integrated and exposed under `/debug/pprof` for performance analysis.
*   **Database:**
    *   Uses PostgreSQL as the database.
    *   Database migrations managed by `golang-migrate/migrate`.
*   **Code Generation:**
    *   Uses `oapi-codegen` to generate Go types from the OpenAPI specification.
    *   Uses `protoc` to generate Go code from Protobuf definitions for gRPC.
*   **Testing:**
    *   Unit tests using Go's `testing` package and `stretchr/testify`.
    *   Mocking using `stretchr/testify/mock` and `mockery`.
    *   Integration tests (`tests/integration_test.go`) covering API endpoint interactions.
    *   Test coverage reporting.

## Technology Stack

*   **Language:** Go (~1.24)
*   **Web Framework / Router:** chi/v5
*   **Database:** PostgreSQL
*   **DB Driver:** jackc/pgx/v5/stdlib
*   **SQL Builder:** Masterminds/squirrel
*   **Migrations:** golang-migrate/migrate
*   **Authentication:** JWT (golang-jwt/jwt/v5), bcrypt
*   **API Specs:** OpenAPI 3.0 (REST), Protobuf (gRPC)
*   **Code Generation:** oapi-codegen, protoc (protoc-gen-go, protoc-gen-go-grpc)
*   **Logging:** log/slog (Go standard library)
*   **Monitoring:** Prometheus (client_golang), Grafana
*   **Testing:** testing, stretchr/testify (assert, require, mock), mockery
*   **Containerization:** Docker, Docker Compose

## API Overview

*   **RESTful HTTP API:** Defined in `api/openapi/swagger.yaml`. Uses JWT Bearer token for authentication. Key endpoints include:
    *   `/register`, `/login`, `/dummyLogin` (Auth)
    *   `/pvz` (POST: Create PVZ, GET: List PVZs with Keyset Pagination)
    *   `/receptions` (POST: Initiate Reception)
    *   `/products` (POST: Add Product)
    *   `/pvz/{pvzId}/delete_last_product` (POST: Delete Last Product)
    *   `/pvz/{pvzId}/close_last_reception` (POST: Close Reception)
    *   `/health` (GET: Health Check)
    *   `/metrics` (GET: Prometheus Metrics)
    *   `/debug/pprof/*` (Profiling Endpoints)
*   **gRPC API:** Defined in `proto/pvz/v1/pvz.proto`.
    *   `PVZService` with `GetPVZList` method.

## Running Locally (using Docker Compose)

1.  **Prerequisites:** Docker and Docker Compose installed.
2.  **Clone the repository.**
3.  **Environment Variables:** Ensure the required environment variables are set. You might need to create a `.env` file in the project root or export them. The crucial one is `JWT_SECRET`. See `docker-compose.yml` and `cmd/api/main.go` for required variables:
    *   `JWT_SECRET`: **REQUIRED** A strong secret key for signing JWTs. **Change the default value!**
    *   `DB_HOST=db`
    *   `DB_PORT=5432`
    *   `DB_USER=user`
    *   `DB_PASSWORD=password`
    *   `DB_NAME=pvzdb`
    *   `PORT=8080` (Optional, defaults to 8080)
    *   `METRICS_PORT=9000` (Optional, defaults to 9000 if aux metrics server is used, otherwise `/metrics` on main port)
    *   `GRPC_PORT=3000` (Optional, defaults to 3000)
    *   `LOG_LEVEL=INFO` (Optional, defaults to INFO. Supports DEBUG, WARN, ERROR)
4.  **Build and Start Services:**
    ```bash
    docker-compose up --build -d
    ```
5.  **Run Database Migrations:** The `migrate` service is defined but doesn't automatically run `up`. Run migrations manually:
    ```bash
    docker-compose run --rm migrate -path /migrations -database postgres://user:password@db:5432/pvzdb?sslmode=disable up
    ```
    *(Note: The `--rm` flag removes the container after execution)*
6.  **Access Services:**
    *   **API:** `http://localhost:8080`
    *   **Prometheus:** `http://localhost:9090`
    *   **Grafana:** `http://localhost:3001` (Default login: admin/admin)
    *   **Database (External):** `localhost:5433` (Connect using user/password/pvzdb)
    *   **gRPC:** `localhost:3000`
7.  **Stop Services:**
    ```bash
    docker-compose down
    ```

## Configuration

The application is configured primarily through environment variables (see "Running Locally" section).

## Database Migrations

Migrations are located in the `/migrations` directory and use `golang-migrate/migrate`.

*   **Apply migrations:**
    ```bash
    docker-compose run --rm migrate -path /migrations -database postgres://user:password@db:5432/pvzdb?sslmode=disable up
    ```
*   **Rollback last migration:**
    ```bash
    docker-compose run --rm migrate -path /migrations -database postgres://user:password@db:5432/pvzdb?sslmode=disable down 1
    ```
*   **Force a specific version:**
    ```bash
    docker-compose run --rm migrate -path /migrations -database postgres://user:password@db:5432/pvzdb?sslmode=disable force <version>
    ```

## Testing

*   **Run all tests:**
    ```bash
    go test ./...
    ```
*   **Run tests with coverage:**
    ```bash
    go test -coverprofile=coverage.out ./...
    ```
*   **View HTML coverage report:**
    ```bash
    go tool cover -html=coverage.out
    ```

## Code Generation

The project uses `go generate` to trigger code generation tools.

*   **OpenAPI (Types):** Uses `oapi-codegen` based on `oapi-codegen.cfg.yaml` and `api/openapi/swagger.yaml`.
*   **Protobuf (gRPC):** Uses `protoc` with Go plugins based on `proto/pvz/v1/pvz.proto`.
*   **Mocks:** Uses `mockery` (triggered via `go generate` directives in `internal/repository/repository.go`).

To regenerate code:

```bash
go generate ./...
