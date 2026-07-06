# Bili-Up 轻量版

功能：

- 扫码登录
- 多账号管理
- Cookie 自动补全和持久化
- 每日登录校验
- 观看视频
- 分享视频
- 投币
- JSON 文件存储账号

不包括推送、漫画、直播、大会员、充电、取关等功能。

## 环境说明

需要：

- Docker
- Docker Compose v2

检查命令：

```powershell
docker --version
docker compose version
```

## Docker Compose 使用

### 1. 准备配置和数据目录

```powershell
New-Item -ItemType Directory -Force config, logs
Copy-Item .env.example .env
```

目录结构：

```text
biliUp
├── .env
├── config
│   └── accounts.json
├── logs
├── Dockerfile
└── docker-compose.yml
```

`accounts.json` 可以扫码登录后自动生成，也可以复制模板后手动编辑：

```powershell
Copy-Item .\config\accounts.example.json .\config\accounts.json
notepad .\config\accounts.json
```

### 2. 修改 `.env`

常用配置：

```dotenv
BILI_UP_TASK_CRON=0 15 * * *
BILI_UP_WATCH_VIDEO=true
BILI_UP_SHARE_VIDEO=true
BILI_UP_NUMBER_OF_COINS=5
BILI_UP_PROTECTED_COINS=0
BILI_UP_SAVE_COINS_WHEN_LV6=false
BILI_UP_SELECT_LIKE=true
BILI_UP_SUPPORT_UP_IDS=
BILI_UP_REQUEST_INTERVAL_SECONDS=3
BILI_UP_TIMEOUT_SECONDS=30
BILI_UP_ACCOUNTS_FILE=config/accounts.json
BILI_UP_LOG_RETENTION_DAYS=90
```

说明：

- `BILI_UP_TASK_CRON`：定时执行时间，默认每天 15:00，时区为 `Asia/Shanghai`。
- `BILI_UP_WATCH_VIDEO`：是否执行观看视频任务。
- `BILI_UP_SHARE_VIDEO`：是否执行分享视频任务。
- `BILI_UP_NUMBER_OF_COINS`：每日目标投币数，范围 `0-5`；设为 `0` 表示不投币。
- `BILI_UP_PROTECTED_COINS`：保留硬币数，余额小于等于该值时不投币。
- `BILI_UP_SAVE_COINS_WHEN_LV6`：账号达到 Lv6 后是否跳过投币。
- `BILI_UP_SELECT_LIKE`：投币时是否同时点赞。
- `BILI_UP_SUPPORT_UP_IDS`：优先支持的 UP 主 UID，多个用英文逗号分隔；为空时使用热门/排行榜视频。
- `BILI_UP_ACCOUNTS_FILE`：账号 Cookie 保存文件。
- `BILI_UP_LOG_RETENTION_DAYS`：按天日志文件保留天数，默认 `90`；设为 `0` 或负数表示不自动清理。

### 3. 构建并启动定时任务

```powershell
docker compose up -d --build
```

默认启动命令：

```powershell
bili-up scheduler
```

### 4. 查看日志

程序会同时把日志输出到容器控制台和按天日志文件：

```text
logs/bili-up-YYYY-MM-DD.log
```

例如：

```text
logs/bili-up-2026-07-06.log
```

也可以继续查看 Docker 日志：

```powershell
docker logs -f bili-up
docker compose logs -f
```

### 5. 扫码登录

首次使用可以扫码登录：

```powershell
docker compose run --rm bili-up login
```

终端会显示紧凑二维码，并在下方打印登录 URL 作为兜底。使用 B 站 App 扫码确认后，账号 Cookie 会写入：

```text
config/accounts.json
```

登录完成后，重新启动定时容器：

```powershell
docker compose up -d
```

如果已经有 B 站 Cookie，也可以直接编辑账号文件：

```powershell
notepad .\config\accounts.json
```

格式如下：

```json
[
  {
    "cookie": "DedeUserID=xxx; SESSDATA=xxx; bili_jct=xxx"
  }
]
```

`accounts.json` 只需要保存 Cookie。`cookie` 必须至少包含 `DedeUserID`、`SESSDATA`、`bili_jct`，程序会自动从 Cookie 里识别 UID。

### 6. 查看账号

```powershell
docker compose run --rm bili-up accounts
```

输出会显示账号 UID 和 Cookie 字段是否完整。

### 7. 手动执行一次任务

真实执行每日任务：

```powershell
docker compose run --rm bili-up run
```

只检查账号读取，不调用 B 站任务接口：

```powershell
docker compose run --rm bili-up run --dry-run
```

注意：真实 `run` 会执行观看、分享和投币，可能消耗硬币。

### 8. 停止、重启、更新

```powershell
docker compose stop
docker compose up -d
docker compose up -d --build
docker compose down
```

## 本地 Go 运行

需要 Go 环境。

```powershell
go version
Copy-Item .env.example .env
```

在仓库根目录执行：

```powershell
go run ./cmd/bili-up accounts
go run ./cmd/bili-up login
go run ./cmd/bili-up run
go run ./cmd/bili-up scheduler
```

## 常见问题

### 1. `accounts.json` 在哪里？

默认路径：

```text
config/accounts.json
```

Docker 中程序工作目录是 `/app`，所以容器内等价路径是：

```text
/app/config/accounts.json
```

`docker-compose.yml` 会把宿主机 `./config` 挂载到容器 `/app/config`，所以手动编辑 `config/accounts.json` 后容器可以直接读取。

### 2. 如何禁止投币？

把 `.env` 改成：

```dotenv
BILI_UP_NUMBER_OF_COINS=0
```

### 3. 如何保留硬币？

例如至少保留 10 个硬币：

```dotenv
BILI_UP_PROTECTED_COINS=10
```

### 4. 如何指定优先投给某些 UP？

填写 UP 主 UID，多个用英文逗号分隔：

```dotenv
BILI_UP_SUPPORT_UP_IDS=123456,987654
```

程序会优先从这些 UP 的投稿中选视频。失败或为空时，会回退到热门/排行榜视频。

### 5. Cookie 安全

Cookie 等同于登录凭据：

- 不要提交到 Git。
- 不要贴到公开渠道。
- 如果泄露，建议立即在 B 站退出登录或重新登录，让旧 Cookie 失效。

## 验证

运行测试：

```powershell
go test ./...
```
