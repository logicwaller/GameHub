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
