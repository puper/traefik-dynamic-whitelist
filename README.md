# Traefik Dynamic Whitelist

Traefik Dynamic Whitelist 是一个轻量级安全网关控制台，用来动态维护 Traefik 的 IP 白名单。

它适合这样的场景：后台服务被 Traefik 白名单保护，管理员临时更换网络后，可以打开控制台，输入固定管理 Token，把当前公网 IP 或指定 IP 加入临时/永久白名单。项目会保存白名单状态，并生成 Traefik dynamic config 文件供 Traefik 自动加载。

## 功能

- 单页控制台，无前端框架，Go 服务内嵌静态资源。
- 固定管理 Token 鉴权，所有 `/api/*` 接口都需要 `Authorization: Bearer <ADMIN_TOKEN>`。
- 展示当前访问 IP、临时白名单、永久白名单。
- 支持添加当前 IP，也支持手动输入指定 IP。
- 支持临时授权和永久授权。
- 临时授权默认 24 小时有效，可配置。
- 同一个 IP 重复授权时，以最新操作为准，保证只存在于临时或永久列表之一。
- 状态写入本地 JSON 文件，并同步生成 Traefik dynamic config YAML。
- 支持 Traefik `ipAllowList.ipStrategy.depth`，适配 Cloudflare -> Traefik 的真实客户端 IP 判断。

## 项目结构

```text
cmd/gateway/main.go       # 服务入口
internal/app/             # 后端配置、鉴权、API、状态管理、Traefik 输出
internal/app/web/         # 内嵌前端页面
examples/traefik/         # Traefik 示例动态配置
README.md                 # 使用说明
```

## 快速启动

本地启动：

```sh
ADMIN_TOKEN=change-me go run ./cmd/gateway
```

默认监听：

```text
http://127.0.0.1:8080
```

页面中输入的 Token 必须和 `ADMIN_TOKEN` 一致。

## Docker 运行

构建镜像：

```sh
docker build -t traefik-dynamic-whitelist .
```

运行容器：

```sh
docker run --rm \
  -p 8080:8080 \
  -e ADMIN_TOKEN=change-me \
  -v "$PWD/data:/data" \
  -v "$PWD/traefik-dynamic:/traefik-dynamic" \
  traefik-dynamic-whitelist
```

容器默认使用：

```text
LISTEN_ADDR=:8080
STATE_PATH=/data/state.json
TRAEFIK_DYNAMIC_PATH=/traefik-dynamic/whitelist.yml
```

也可以复制示例 compose 文件后按实际域名、Token、Traefik 网络修改：

```sh
cp docker-compose.example.yml docker-compose.yml
docker compose up -d --build
```

## 配置项

| 环境变量 | 默认值 | 说明 |
| --- | --- | --- |
| `ADMIN_TOKEN` | 无 | 必填，单个固定管理 Token |
| `LISTEN_ADDR` | `:8080` | HTTP 监听地址 |
| `STATE_PATH` | `data/state.json` | 白名单状态文件 |
| `TRAEFIK_DYNAMIC_PATH` | `data/whitelist.yml` | Traefik dynamic config 输出路径 |
| `TRAEFIK_IP_STRATEGY_DEPTH` | `0` | Traefik `ipAllowList.ipStrategy.depth`，`0` 表示不输出 |
| `TEMP_HOURS` | `24` | 临时授权有效小时数 |
| `CLEANUP_INTERVAL` | `1m` | 过期临时 IP 清理间隔，只有发现状态变化才重写配置 |
| `CLIENT_IP_HEADERS` | `X-Forwarded-For,X-Real-IP` | 可信代理场景下读取真实 IP 的请求头优先级 |
| `TRUSTED_PROXIES` | 空 | 可信代理 CIDR 或 IP，逗号分隔 |

## Cloudflare -> Traefik 部署

推荐链路：

```text
Client -> Cloudflare -> Traefik -> Traefik Dynamic Whitelist
```

### 1. Cloudflare 注入 Header

在 Cloudflare Dashboard 中创建 Request Header Transform Rule：

- Header name: `X-CDN-Token`
- Header value: 一段足够长的随机密钥
- 匹配条件：只匹配控制台域名，例如 `gateway.example.com`

### 2. Traefik 校验 Header

`X-CDN-Token` 应在 Traefik 层校验。当前项目不校验 CDN token，只负责管理 Token 和白名单状态。

可以在控制台 router 上增加 header 条件：

```yaml
http:
  routers:
    whitelist-console:
      rule: "Host(`gateway.example.com`) && Header(`X-CDN-Token`, `your-long-random-cdn-token`)"
      service: whitelist-console
```

也可以把 header 校验做成独立 middleware，再挂到需要保护的 router 上。关键点是：请求到达当前项目前，Traefik 已经确认它带有 Cloudflare 注入的 `X-CDN-Token`。

当前项目启动示例：

```sh
ADMIN_TOKEN=change-me \
CLIENT_IP_HEADERS=CF-Connecting-IP,X-Forwarded-For,X-Real-IP \
TRAEFIK_IP_STRATEGY_DEPTH=1 \
go run ./cmd/gateway
```

### 3. Traefik 使用生成的白名单

将 `TRAEFIK_DYNAMIC_PATH` 指向 Traefik file provider 会读取的文件路径。例如：

```sh
TRAEFIK_DYNAMIC_PATH=/etc/traefik/dynamic/whitelist.yml
```

Docker Compose 示例中，当前项目把动态配置写到宿主机 `./traefik-dynamic/whitelist.yml`。Traefik 容器需要把同一个宿主机目录挂载为 file provider 的动态配置目录。

项目会生成类似内容：

```yaml
http:
  middlewares:
    dynamic-whitelist:
      ipAllowList:
        sourceRange:
          - 203.0.113.10/32
        ipStrategy:
          depth: 1
```

在 Traefik router 中引用该 middleware：

```yaml
http:
  routers:
    protected-service:
      rule: "Host(`app.example.com`)"
      service: protected-service
      middlewares:
        - dynamic-whitelist
```

仓库提供了两个示例：

- `examples/traefik/whitelist-console.yml`：本控制台的 Traefik router/service 示例，包含 `X-CDN-Token` header 条件。
- `examples/traefik/bitwarden.yml`：目标项目 Bitwarden 的 Traefik router/service 示例，包含 `X-CDN-Token` header 条件，并引用 `dynamic-whitelist` middleware。

`TRAEFIK_IP_STRATEGY_DEPTH=1` 的作用是让 Traefik 从 `X-Forwarded-For` 中取真实客户端 IP 做白名单判断，而不是使用 Cloudflare 到 Traefik 的连接 IP。

## 真实 IP 获取

当前项目不会让前端自行探测公网 IP。前端展示的当前 IP 来自后端 `GET /api/info`。

后端判断当前 IP 的规则：

1. 先获取直接连接当前项目的远端地址。
2. 如果远端地址命中 `TRUSTED_PROXIES`，则按 `CLIENT_IP_HEADERS` 顺序读取请求头中的 IP。
3. 如果未命中可信代理，则使用直接连接地址。

在 IPv4/IPv6 双栈网络下，同一个访问者可能一会儿走 IPv4，一会儿走 IPv6。`GET /api/info` 会返回 `current_ips`，前端在“授权 IP”输入框留空时，会把当前检测到的 IP 一次性加入白名单。为了避免代理链里出现多个不同 IPv4 时误授权，后端最多保留一个 IPv4 和一个 IPv6，按 `CLIENT_IP_HEADERS` 配置顺序优先。手动输入指定 IP 时，只授权输入的那个 IP。

示例：

```sh
ADMIN_TOKEN=change-me \
TRUSTED_PROXIES=127.0.0.1,10.0.0.0/8 \
CLIENT_IP_HEADERS=CF-Connecting-IP,X-Forwarded-For,X-Real-IP \
go run ./cmd/gateway
```

不要无条件信任公网请求传入的 `X-Forwarded-For`，否则用户可以伪造要加入白名单的 IP。

## API

所有 `/api/*` 请求都需要：

```http
Authorization: Bearer <ADMIN_TOKEN>
```

响应格式统一为：

```json
{
  "result": {},
  "error": null
}
```

错误响应：

```json
{
  "result": null,
  "error": {
    "code": "UNAUTHORIZED",
    "message": "凭证已失效，请重新输入"
  }
}
```

### `GET /api/info`

获取当前访问 IP 和白名单状态。

```json
{
  "result": {
    "current_ip": "203.0.113.10",
    "current_ips": [
      "203.0.113.10",
      "2001:db8::10"
    ],
    "temporary_ips": [
      {
        "ip": "203.0.113.10",
        "added_at": "2026-05-12T06:30:00Z",
        "expires_at": "2026-05-13T06:30:00Z"
      }
    ],
    "permanent_ips": [
      {
        "ip": "198.51.100.5",
        "added_at": "2026-05-12T06:30:00Z"
      }
    ]
  },
  "error": null
}
```

时间使用 UTC，前端会按用户本地时区展示。

### `POST /api/add`

添加白名单。

添加当前访问 IP：

```json
{
  "type": "temp",
  "ips": [
    "203.0.113.10",
    "2001:db8::10"
  ]
}
```

添加指定 IP：

```json
{
  "type": "perm",
  "ip": "203.0.113.10"
}
```

`type` 可选值：

- `temp`：临时授权。
- `perm`：永久授权。

### `POST /api/delete`

删除指定 IP。删除接口要求明确传入 `ip`，不会默认删除当前访问 IP。

```json
{
  "ip": "203.0.113.10"
}
```

## 开发

模块路径：

```text
github.com/puper/traefik-dynamic-whitelist
```

Go 版本：

```text
1.26.3
```

Docker runtime 基础镜像：

```text
alpine:3.22.4
```

格式化：

```sh
gofmt -w cmd/gateway/main.go internal/app/*.go
```

测试：

```sh
go test ./...
```

编译：

```sh
go build -o /tmp/traefik-dynamic-whitelist-gateway ./cmd/gateway
```

## Traefik 和当前项目的职责边界

- `X-CDN-Token` 校验属于入口层安全策略，应该配置在 Traefik。
- Traefik 的白名单判断由生成的 `dynamic-whitelist` middleware 完成。
- 当前项目负责管理 Token 鉴权、展示当前 IP、维护白名单状态、输出 Traefik dynamic config。
- 当前项目的 `TRUSTED_PROXIES` 只影响它自己如何识别当前访问 IP，不会改变 Traefik 的白名单判断逻辑。
- Traefik 如何从 `X-Forwarded-For` 中取真实客户端 IP，由 `TRAEFIK_IP_STRATEGY_DEPTH` 生成的 `ipStrategy.depth` 控制。

## 安全说明

- `ADMIN_TOKEN` 必须使用足够长的随机值。
- Cloudflare 注入的 `X-CDN-Token` 也必须使用足够长的随机值，并在 Traefik 层校验。
- 不要把 token 写入 Git。
- `X-CDN-Token` 是共享密钥，不能替代所有网络层防护。
- 如果源站可以绕过 Cloudflare 直接访问，建议配合 Cloudflare Tunnel、源站防火墙或 Traefik 入口限制。
- `TRUSTED_PROXIES` 只应配置你真正信任的代理地址或网段。
