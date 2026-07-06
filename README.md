# Bili-Up 轻量版

功能：

- 扫码登录
- 多账号管理
- Cookie 自动补全和持久化
- 每日登录校验
- 观看视频
- 分享视频
- 投币
- JSON 文件存储

不包括推送、漫画、直播、大会员、充电、取关等功能。

## 环境说明

需要：

- Docker
- Docker Compose v2

检查命令：

```bash
docker --version
docker compose version
```

如果没有安装 Docker，可以参考 Docker 官方 Debian 安装文档。安装完成后，确保当前用户能执行 `docker` 命令，或使用 `sudo docker ...`。

## Docker Compose 使用

### 1. 准备配置目录

```bash
mkdir -p config logs
cp config.example.json config/config.json
```

目录结构：

```text
go
├── config
│   ├── config.json
│   └── accounts.json
├── logs
├── Dockerfile
└── docker-compose.yml
```

`accounts.json` 可以扫码登录后自动生成，也可以复制模板后手动编辑：

```bash
cp config/accounts.example.json config/accounts.json
nano config/accounts.json
```

### 2. 修改配置

编辑配置文件：

```bash
nano config/config.json
```

常用配置：

```jsonc
{
  "task": {
    "cron": "0 15 * * *",
    "watchVideo": true,
    "shareVideo": true,
    "numberOfCoins": 5,
    "protectedCoins": 0,
    "saveCoinsWhenLv6": false,
    "selectLike": true,
    "supportUpIds": []
  },
  "storage": {
    "accountsFile": "config/accounts.json"
  }
}
```

说明：

- `task.cron`：定时执行时间，默认每天 15:00，时区为 `Asia/Shanghai`。
- `task.watchVideo`：是否执行观看视频任务。
- `task.shareVideo`：是否执行分享视频任务。
- `task.numberOfCoins`：每日目标投币数，范围 `0-5`；设为 `0` 表示不投币。
- `task.protectedCoins`：保留硬币数，余额小于等于该值时不投币。
- `task.saveCoinsWhenLv6`：账号达到 Lv6 后是否跳过投币。
- `task.selectLike`：投币时是否同时点赞。
- `task.supportUpIds`：优先支持的 UP 主 UID；为空时使用热门/排行榜视频。
- `storage.accountsFile`：账号 Cookie 保存文件。

配置文件支持 `//` 和 `/* */` 注释，可以直接使用 `config.example.json` 作为模板。

### 3. 构建并启动定时任务

```bash
docker compose up -d --build
```

默认启动命令：

```bash
bili-up --config /app/config/config.json scheduler
```

容器会读取：

```text
/app/config/config.json
```

对应宿主机文件：

```text
go/config/config.json
```

### 4. 查看日志

```bash
docker logs -f bili-up
```

或：

```bash
docker compose logs -f
```

### 5. 扫码登录

首次使用可以扫码登录：

```bash
docker compose run --rm bili-up --config /app/config/config.json login
```

终端会显示紧凑二维码，并在下方打印登录 URL 作为兜底。使用 B 站 App 扫码确认后，账号 Cookie 会写入：

```text
go/config/accounts.json
```

登录完成后，重新启动定时容器：

```bash
docker compose up -d
```

如果已经有 B 站 Cookie，也可以直接编辑宿主机上的账号文件：

```bash
nano config/accounts.json
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

```bash
docker compose run --rm bili-up --config /app/config/config.json accounts
```

输出会显示账号 UID 和 Cookie 字段是否完整。

### 7. 手动执行一次任务

真实执行每日任务：

```bash
docker compose run --rm bili-up --config /app/config/config.json run
```

只检查账号读取，不调用 B 站任务接口：

```bash
docker compose run --rm bili-up --config /app/config/config.json run --dry-run
```

注意：真实 `run` 会执行观看、分享和投币，可能消耗硬币。

### 8. 停止、重启、更新

停止：

```bash
docker compose stop
```

重启：

```bash
docker compose up -d
```

重新构建并启动：

```bash
docker compose up -d --build
```

停止并删除容器：

```bash
docker compose down
```

## 本地 Go 运行

如果不使用 Docker，也可以直接运行。

需要 Go 环境。

```bash
go version
```

在 `go/` 目录执行：

```bash
go run ./cmd/bili-up --config ./config.example.json accounts
go run ./cmd/bili-up --config ./config.example.json login
go run ./cmd/bili-up --config ./config.example.json run
go run ./cmd/bili-up --config ./config.example.json scheduler
```

## 常见问题

### 1. `accounts.json` 在哪里？

宿主机路径：

```text
config/accounts.json
```

Docker 容器内路径：

```text
/app/config/accounts.json
```

`docker-compose.yml` 会把宿主机 `./config` 挂载到容器 `/app/config`，所以手动编辑 `config/accounts.json` 后容器可以直接读取。

### 2. 如何禁止投币？

把配置改成：

```jsonc
"numberOfCoins": 0
```

### 3. 如何保留硬币？

例如至少保留 10 个硬币：

```jsonc
"protectedCoins": 10
```

### 4. 如何指定优先投给某些 UP？

填写 UP 主 UID：

```jsonc
"supportUpIds": [123456, 987654]
```

程序会优先从这些 UP 的投稿中选视频。失败或为空时，会回退到热门/排行榜视频。

### 5. Cookie 安全

Cookie 等同于登录凭据：

- 不要提交到 Git。
- 不要贴到公开渠道。
- 如果泄露，建议立即在 B 站退出登录或重新登录，让旧 Cookie 失效。

## 验证

运行测试：

```bash
go test ./...
```
