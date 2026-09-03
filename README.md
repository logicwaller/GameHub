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

The backend now uses MySQL for users and games. Create the `gamehub` database first, then copy `backend/.env.example` to `backend/.env` and fill in your values:

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

On first startup the API creates all tables (`users`, `games`, favorites, likes, comments, posts, replies, and play events) if they do not exist. Check `http://localhost:8080/api/health`; a healthy response includes `"database":"mysql"`.

The complete schema for favorites, likes, game comments, forum posts, post replies, and play-time events is in `backend/schema.sql`. Execute it with your MySQL password if you want to create the tables before starting the API:

```powershell
mysql -u root -p gamehub < backend\schema.sql
```

The API startup migration also creates these tables automatically after a successful database connection.

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
```

Create the Kafka topic once after the broker starts:

```powershell
docker exec gamehub-kafka /opt/kafka/bin/kafka-topics.sh --create --if-not-exists --topic game.play --bootstrap-server localhost:9092
```

Phase-three endpoints:

- `GET /api/games?q=keyword&sort=plays|likes` searches and sorts games.
- `GET /api/games/:id` uses a one-hour Redis detail cache.
- `GET /api/games/hot` returns the Redis hot-game ranking.
- `POST /api/games/:id/play` publishes a `game.play` event; the Kafka consumer asynchronously increments play count and updates the ranking. If Kafka is unavailable, the API automatically falls back to synchronous MySQL update.
- Requests are limited to 120 per IP per minute when Redis is available.

Redis failures fail open, so MySQL-backed pages continue to work. Kafka is connected directly through the Go client; the backend does not need access to the Docker CLI. The broker only needs to be reachable at `KAFKA_BROKERS`.
