# 食堂窗口叫号

学校食堂纯排队叫号三端系统（用餐者 / 员工 / 管理员 + 窗口大屏）。一期不做点餐与支付。

规划与合同见 [`docs/dev/bgx/INDEX.md`](docs/dev/bgx/INDEX.md)。

## 技术栈

- API：Go + chi + PostgreSQL + Redis（Redis 在实时推送阶段接入）
- Web：Vue 3 + Vite（用餐者 / 员工 / 管理 / 大屏同一应用，按路由隔离）
- 合同：[`api/openapi.yaml`](api/openapi.yaml)

## 本地开发

数据库使用**远程 PostgreSQL**，本仓库不安装、不启动本地数据库。

1. 复制 `.env.example` 为 `.env`，填入 `DATABASE_URL` 与 `JWT_SECRET`。远端库若禁止在 `public` 建表，设置 `DATABASE_SCHEMA=jiaohao`。
2. 迁移：

```powershell
cd apps/api
go run ./cmd/migrate up
```

3. 启动 API：

```powershell
cd apps/api
go run ./cmd/api
```

健康检查：<http://127.0.0.1:8080/health>

4. 启动 Web：

```powershell
cd apps/web
npm install
# 若 registry.npmjs.org 超时，可改用：
# npm install --registry https://registry.npmmirror.com
npm run dev
```

浏览器打开 Vite 提示的本地地址（默认 <http://localhost:5173>）。`/api` 会代理到 API。

### 种子账号（库中不存在时由 API 启动写入）

| 角色 | 学号 | 默认密码来源 |
| --- | --- | --- |
| admin | `ADMIN_STUDENT_ID` | `ADMIN_PASSWORD` |
| diner | `SEED_DINER_STUDENT_ID` | `SEED_DINER_PASSWORD` |
| staff | `SEED_STAFF_STUDENT_ID` | `SEED_STAFF_PASSWORD` |

## 常用命令

```powershell
# 回滚最近一批迁移会删表。不要对这套共享远程库执行。
cd apps/api
go run ./cmd/migrate down

# API 单测（不连库）
cd apps/api
go test ./...
```

构建：

```powershell
cd apps/api
go build -o ../../bin/api.exe ./cmd/api
cd ../web
npm run build
```

功能回归（API 需已在 127.0.0.1:8080，不打印密码或令牌）：

```powershell
cd apps/api
go run ./cmd/check
go run ./cmd/wsload -n 5
```

超过 100 路连接会被拒绝。2000–3000 的压测需要另一次授权，不要把 `ALLOW_STRESS=1` 当成已经批准。

## 备份与回滚

数据库在远程 PostgreSQL，不要在这台机器安装数据库。备份使用本机 `.env` 里的 `DATABASE_URL`，不要把连接串写进命令历史之外的文件：

```powershell
pg_dump --schema=jiaohao --file=jiaohao-backup.sql "$env:DATABASE_URL"
```

回滚程序是换回上一版 API 和 Web 构建。回滚数据库会丢掉备份之后的取号，只有明确要恢复快照时才做。不要对这套共享库运行 `migrate down`。单实例不依赖 Redis；Redis 只在以后多实例推送时需要。

演示步骤见 [`docs/DEMO.md`](docs/DEMO.md)。

## 安全

- `.env` 已被忽略，禁止把数据库密码、JWT 密钥写入源码或文档。
- 日志不记录密码与令牌明文。
