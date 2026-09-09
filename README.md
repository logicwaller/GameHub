# GameHub

GameHub 是一个网页游戏发现与交流平台。本项目采用前后端分离的单体仓库结构：

- `backend/` Go + Gin API（MySQL 主库、Redis 缓存、Kafka 异步事件）
- `frontend/` Vue 3 + Vite 前端单页应用

## 本地运行

```powershell
cd backend
go run .

cd ..\frontend
npm install
npm run dev
```

后端 API 默认监听 `http://localhost:8080`；Vite 前端默认运行在
`http://localhost:5173`，并将 `/api` 请求代理到后端。

## MySQL 配置

后端使用 MySQL 保存用户、游戏及其他业务数据。启动 API 前请先导入完整数据库
结构，然后将 `backend/.env.example` 复制为 `backend/.env` 并填写配置：

```powershell
cd backend
Copy-Item .env.example .env
```

后端启动时通过 `godotenv` 自动加载 `.env`。请在 `backend` 目录执行
`go run .`，这样程序才能找到同目录下的配置文件。命令行中已经设置的环境变量
优先级高于 `.env` 中的同名变量。

`backend/schema.sql` 是创建或升级数据库结构的唯一文件，其中包含所有必需的表，
包括 Kafka 事件、通知和数据分析相关表。启动 API 前执行：

```powershell
cd backend
Get-Content -Raw .\schema.sql | mysql -u root -p
```

每个游戏必须选择一个主类型（`ARG/WIG`、`现实互动解谜`、`网页互动游戏`、
`网页解谜`或`互动叙事`），并可添加最多 12 个标签。数据库通过 `tags` 和
`game_tags` 表保存标签关系。如果需要删除全部游戏数据但保留用户和论坛数据，
请先停止后端，再执行：

```powershell
Get-Content -Raw .\reset_game_data.sql | mysql -u root -p
```


## Redis 与 Kafka

使用 Docker Compose 启动第三阶段所需的中间件：

```powershell
cd GameHub
docker compose up -d redis kafka
```

Compose 已配置为单节点 Kafka Broker，并设置了内部消费者偏移量主题所需的副本参数。
修改 Kafka 配置后，只需重新创建 Kafka 容器及其数据卷：

```powershell
docker compose stop kafka
docker rm gamehub-kafka
docker volume rm gamehub_kafka_data
docker compose up -d kafka
```

开发环境的 Compose 配置已开启自动创建主题。后端启动时还会检查业务主题及其
`.dlq` 死信主题是否存在。拉取最新配置后，重新启动一次 Kafka：

```powershell
docker compose up -d --force-recreate kafka
```

第三阶段相关接口：

- `GET /api/games?q=keyword&sort=plays|likes`：搜索并排序游戏。
- `GET /api/games/:id`：使用 Redis 缓存游戏详情，缓存时间为一小时。
- `GET /api/games/hot`：返回 Redis 中的热门游戏排行榜。
- `POST /api/games/:id/play`：发布 `game.play` 事件，由 Kafka 消费者异步增加游玩量并更新排行榜。Kafka 不可用时，API 会自动降级为同步写入 MySQL。
- Redis 可用时，每个 IP 每分钟最多请求 120 次。

第三阶段的其他功能：

- `schema.sql` 创建 Kafka 已处理事件、搜索文档、通知、游戏日统计和一次性统计回填标记等表。
- Kafka 消费者会更新本地搜索文档、保存通知，并按天汇总互动数据。
- `GET /api/games/favorites/rank`：返回收藏量最高的 20 个游戏；Redis 不可用时使用 MySQL 查询。
- 后端启动时会根据已有游玩、点赞、收藏和评论记录回填历史日统计，完成后写入 `analytics_backfill_state`。
- `POST /api/auth/logout`：将当前 JWT 加入 Redis 黑名单；登录和注册接口限制为每个 IP 每分钟 10 次。
- 已登录用户的写请求可以携带 `Idempotency-Key` 请求头，避免两分钟内的重复提交。
- `GET /api/me/relations`、`GET /api/notifications` 和 `GET /api/games/analytics?days=7&game_id={id}` 分别提供用户互动状态、通知和真实的日统计数据。

## AI 攻略助手与问答历史

AI 攻略助手要求用户登录，并通过 `ZHIPU_API_KEY` 调用智谱模型。执行最新
`schema.sql` 后，问答历史会持久化到 MySQL：`agent_conversations` 保存会话，
`agent_messages` 保存用户问题和 AI 回复，且每个用户只能访问自己的会话。

相关接口：

- `GET /api/agent/conversations`：读取当前用户的会话列表。
- `POST /api/agent/conversations`：创建新会话。
- `GET /api/agent/conversations/:id/messages`：读取会话消息。
- `DELETE /api/agent/conversations/:id`：删除会话及其消息。
- `POST /api/agent/chat`：提交问题并通过 SSE 流式返回回答，同时写入 MySQL。

当前 Agent 已实现问答和历史记录持久化，但尚未实现方案中的游戏攻略知识库、
Embedding 和向量检索（当前模型只接收用户本轮问题）。

管理员可以在管理页面的“Kafka 死信队列”区域查看并重放失败的 Kafka 消息。
对应接口为 `GET /api/admin/kafka/dlq/{topic}` 和
`POST /api/admin/kafka/dlq/{topic}/{eventID}/replay`。

Redis 故障时系统会自动降级，因此依赖 MySQL 的页面仍可正常使用。Kafka 由 Go
客户端直接连接，后端不需要访问 Docker 命令行；只要 `KAFKA_BROKERS` 指向的
Broker 可访问即可。
