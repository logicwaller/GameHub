# GameHub

GameHub is a web game discovery platform. The repository is organized as a small monorepo:

- `backend/` Go + Gin API (in-memory users for phase one)
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

The backend now uses MySQL for users and games. Create the `gamehub` database first, then set the variables from `backend/.env.example` in your shell. Go does not load `.env` files automatically, so PowerShell example:

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
