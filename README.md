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
# 回滚最近一批迁移（会删表，仅开发库）
cd apps/api
go run ./cmd/migrate down

# API 单测（不连库）
cd apps/api
go test ./...
```

## 安全

- `.env` 已被忽略，禁止把数据库密码、JWT 密钥写入源码或文档。
- 日志不记录密码与令牌明文。
