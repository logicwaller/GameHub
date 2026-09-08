# GameHub

GameHub is a web game discovery platform. The repository is organized as a small monorepo:

- `backend/` Go + Gin API（MySQL 主库、Redis 缓存、Kafka 异步事件）
- `frontend/` Vue 3 + Vite single-page application

## Run locally

```powershell
cd backend
go run .

cd ..\frontend
npm install
npm run dev
```

The API listens on `http://localhost:8080`; Vite serves the UI on `http://localhost:5173` and proxies `/api` requests to the API.

## MySQL configuration

The backend uses MySQL for users and games. Import the complete schema before
starting the API, then copy `backend/.env.example` to `backend/.env` and fill
in your values:

```powershell
cd backend
Copy-Item .env.example .env
```

The backend uses `godotenv` to load `.env` automatically when it starts. Run `go run .` from the `backend` directory so the file can be found. Environment variables already set in the shell take precedence over values loaded from `.env`.

If you do not want to use a `.env` file, you can set the variables directly in PowerShell:

```powershell
cd backend
go mod tidy
$env:MYSQL_HOST = "127.0.0.1"
$env:MYSQL_PORT = "3306"
$env:MYSQL_USER = "root"
$env:MYSQL_PASSWORD = "your-password"
$env:MYSQL_DATABASE = "gamehub"
$env:JWT_SECRET = "use-a-long-random-value"
go run .
```

`backend/schema.sql` is the only place that creates or upgrades the database
schema. It contains all required tables, including the Kafka, notification,
and analytics tables. Execute it before starting the API:

```powershell
cd backend
mysql -u root -p < schema.sql
```


## Redis and Kafka

Start the phase-three middleware with Docker Compose:

```powershell
cd GameHub
docker compose up -d redis kafka
```

Compose is configured for a single-node Kafka broker (including replication settings for the internal consumer-offset topic). After changing Kafka settings, recreate only the Kafka container and its data volume:

```powershell
docker compose stop kafka
docker rm gamehub-kafka
docker volume rm gamehub_kafka_data
docker compose up -d kafka
```

The backend uses these environment variables (the defaults point to the local containers):

```text
REDIS_ADDR=127.0.0.1:6379
REDIS_PASSWORD=
REDIS_DB=0
KAFKA_BROKERS=127.0.0.1:9092
KAFKA_GAME_PLAY_TOPIC=game.play
KAFKA_SEARCH_TOPIC=search.sync
KAFKA_INTERACTION_TOPIC=interaction.event
KAFKA_NOTIFICATION_TOPIC=notification
```

The development Compose configuration enables automatic topic creation. The backend also ensures the business topics and their `.dlq` dead-letter topics exist at startup. Restart Kafka once after pulling the updated configuration:

```powershell
docker compose up -d --force-recreate kafka
```

Phase-three endpoints:

- `GET /api/games?q=keyword&sort=plays|likes` searches and sorts games.
- `GET /api/games/:id` uses a one-hour Redis detail cache.
- `GET /api/games/hot` returns the Redis hot-game ranking.
- `POST /api/games/:id/play` publishes a `game.play` event; the Kafka consumer asynchronously increments play count and updates the ranking. If Kafka is unavailable, the API automatically falls back to synchronous MySQL update.
- Requests are limited to 120 per IP per minute when Redis is available.

Additional phase-three behaviour:

- `schema.sql` creates the phase-three tables: processed Kafka events, search documents, notifications, daily game analytics, and the one-time analytics-backfill marker.
- Kafka consumers update the local search-document index, store notifications, and aggregate interaction metrics by day.
- `GET /api/games/favorites/rank` returns the top 20 games ranked by favorite count, using Redis with a MySQL fallback.
- On startup, the backend performs a one-time historical backfill of daily analytics from existing play, like, favorite, and comment records. Its completion is recorded in `analytics_backfill_state`.
- `POST /api/auth/logout` blacklists the current JWT in Redis; login/register endpoints have a stricter 10-per-minute IP limit.
- Authenticated write requests may provide an `Idempotency-Key` header to reject accidental retries for two minutes.
- `GET /api/me/relations`, `GET /api/notifications` and `GET /api/games/analytics?days=7&game_id={id}` provide cached interaction state, notifications and real daily analytics data.

Administrators can inspect and replay failed Kafka messages from the “Kafka 死信队列” section of the admin page. The corresponding APIs are `GET /api/admin/kafka/dlq/{topic}` and `POST /api/admin/kafka/dlq/{topic}/{eventID}/replay`.

Redis failures fail open, so MySQL-backed pages continue to work. Kafka is connected directly through the Go client; the backend does not need access to the Docker CLI. The broker only needs to be reachable at `KAFKA_BROKERS`.
