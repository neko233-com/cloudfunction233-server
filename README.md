# cloudfunction233-server

Go 1.26 编写的轻量多租户云函数服务器。默认监听 `6988`，默认租户账号为 `root/root`。

默认端口：

- HTTP 云函数：`6988`
- TCP 云函数：`6989`，默认屏蔽，仅保留协议配置机制。
- UDP 云函数：`6990`，默认屏蔽，仅保留协议配置机制。

后台管理：

```text
http://localhost:6988/admin/
```

## 一键安装

Linux / macOS:

```sh
curl -fsSL https://raw.githubusercontent.com/neko233/cloudfunction233-server/main/scripts/install.sh | sh
```

Windows PowerShell:

```powershell
iwr https://raw.githubusercontent.com/neko233/cloudfunction233-server/main/scripts/install.ps1 -UseB | iex
```

如果你 fork 了仓库，可以指定 Release 来源：

```sh
CF233_REPO=your-name/cloudfunction233-server curl -fsSL https://raw.githubusercontent.com/your-name/cloudfunction233-server/main/scripts/install.sh | sh
```

## 常用命令

```sh
cloudfunction233-server serve
cloudfunction233-server start
cloudfunction233-server stop
cloudfunction233-server restart
cloudfunction233-server status
cloudfunction233-server autostart enable
cloudfunction233-server autostart disable
cloudfunction233-server update --repo neko233/cloudfunction233-server
cloudfunction233-server passwd --user root --password new-password
cloudfunction233-server init-config
```

不带参数启动时等价于：

```sh
cloudfunction233-server serve
```

## 配置

安装后默认配置文件：

- Linux / macOS：`/opt/cloudfunction233-server/config.yaml`
- Windows：`C:\ProgramData\cloudfunction233-server\config.yaml`

开发时也可以在当前目录放 `config.yaml`，或通过环境变量指定：

```sh
CF233_CONFIG=/path/to/config.yaml cloudfunction233-server serve
```

示例：

```yaml
port: "6988"
tcpPort: "6989"
udpPort: "6990"
enableTcp: false
enableUdp: false
dataDir: ./data
runtimeDir: ./runtimes
nodeBin: node
npmBin: npm
defaultUsername: root
defaultPassword: root
invocationTimeoutSeconds: 15
```

当前支持热重载的配置：

- `invocationTimeoutSeconds`
- `nodeBin`
- `npmBin`

下面这些会影响监听地址、存储位置或启动身份，需要重启生效：

- `port`
- `tcpPort`
- `udpPort`
- `enableTcp`
- `enableUdp`
- `dataDir`
- `runtimeDir`
- `defaultUsername`
- `defaultPassword`

修改 root 密码建议使用命令：

```sh
cloudfunction233-server passwd --user root --password new-password
cloudfunction233-server restart
```

## 请求路径

请求路径按租户隔离：

```text
/{username}/{project}/{function/path}
```

比如函数绑定到 root 租户、`demo` 项目的 `/demo/hello`，外部访问就是：

```text
http://localhost:6988/root/demo/hello
```

如果访问：

```text
http://localhost:6988/root/demo/hello/extra
```

函数会收到：

```text
request.cf233.path = /root/demo/hello/extra
ctx.remainingPath = /extra
```

如果创建函数时不传 `route`，服务器会按项目和函数名自动生成：

```text
/{project}/{name}
```

## 部署 npm 函数

```powershell
$body = @{
  name = "hello"
  route = "/diy/path"
  runtime = "npm"
  entrypoint = "index.js"
  handler = "fetch"
  install = $false
  files = @{
    "package.json" = '{"type":"module"}'
    "index.js" = @'
export default {
  async fetch(request, env, ctx) {
    return Response.json({
      message: "hello from cloudfunction233",
      path: request.cf233.path,
      remainingPath: ctx.remainingPath
    });
  }
}
'@
  }
} | ConvertTo-Json -Depth 8

Invoke-RestMethod `
  -Uri http://localhost:6988/api/v1/functions `
  -Method Post `
  -Headers @{ Authorization = "Basic " + [Convert]::ToBase64String([Text.Encoding]::ASCII.GetBytes("root:root")) } `
  -ContentType "application/json" `
  -Body $body
```

访问：

```powershell
Invoke-RestMethod http://localhost:6988/root/diy/path
Invoke-RestMethod http://localhost:6988/root/diy/path/extra
```

## TCP / UDP 云函数

TCP 和 UDP 入口现在默认屏蔽，暂不开发执行入口，只预留协议配置结构。后台的 TCP 结构页会显示：

- 分包模式：`length-prefix`
- 长度字段类型：`uint32`
- 长度字段字节：`4`
- 字节序：`big`
- 最大包长度：`1048576`

未来启用后，TCP 和 UDP 会使用独立端口，不走 HTTP path，而是用一条 JSON 消息选择租户和函数：

```json
{
  "tenant": "root",
  "name": "echo",
  "body": "hello tcp",
  "base64": false,
  "headers": {
    "x-demo": "1"
  }
}
```

部署 TCP 函数时设置：

```json
{
  "name": "echo",
  "type": "tcp",
  "route": "/tcp/echo",
  "runtime": "npm"
}
```

部署 UDP 函数时设置：

```json
{
  "name": "echo",
  "type": "udp",
  "route": "/udp/echo",
  "runtime": "npm"
}
```

函数里可以通过 header 区分协议：

```js
export async function fetch(request) {
  const text = await request.text()
  return Response.json({
    protocol: request.headers.get("x-cf233-protocol"),
    text
  })
}
```

TCP 每一行 JSON 是一次调用；UDP 每个 datagram 是一次调用。返回值统一为：

```json
{
  "status": 200,
  "headers": {},
  "body": "...",
  "base64": false
}
```

## 关于内置运行时

如果目标是“不要求用户提前安装 node、python、java、go”，推荐做成 runtime pack：

- Release 包里附带对应平台的 Node/Python/JRE/Go tiny runner。
- 服务启动时只把 `runtimes/<goos>-<goarch>/...` 里存在的运行时标记为可用。
- 没有内置 runtime pack 的语言会显示为不可用，不能部署，避免偷偷依赖系统环境。

完全把 Java、Python、Node、Go 编译器和运行时都静态塞进一个 Go 单文件里并不现实：体积、授权、跨平台、C ABI、JVM/JIT、标准库和依赖管理都会变复杂。生产上更常见的是“单安装包内置运行时”，对用户来说仍然是不依赖外部环境。

当前永远可用的内置运行时：

- `static`：返回配置里的静态 HTTP 响应，不依赖任何外部环境。

预留 runtime pack 名称：

- `node`
- `npm`
- `python`
- `go`
- `java`
- `kotlin`

## 自动化 HTTP API

后台和脚本共用同一套 API。脚本推荐 Basic Auth：

```sh
curl -u root:root http://localhost:6988/api/v1/functions
```

创建函数：

```sh
curl -u root:root -X POST http://localhost:6988/api/v1/functions \
  -H 'content-type: application/json' \
  -d '{
    "project": "demo",
    "name": "hello",
    "type": "http",
    "runtime": "static",
    "env": {
      "body": "hello builtin"
    }
  }'
```

查询：

```sh
curl -u root:root http://localhost:6988/api/v1/functions
curl -u root:root http://localhost:6988/api/v1/functions/hello
curl -u root:root http://localhost:6988/api/v1/projects
curl -u root:root http://localhost:6988/api/v1/runtimes
curl -u root:root http://localhost:6988/api/v1/tcp-protocol
```

修改：

```sh
curl -u root:root -X PUT http://localhost:6988/api/v1/functions/hello \
  -H 'content-type: application/json' \
  -d '{
    "project": "demo",
    "type": "http",
    "runtime": "static",
    "env": {
      "body": "updated"
    }
  }'
```

删除：

```sh
curl -u root:root -X DELETE http://localhost:6988/api/v1/functions/hello
```

## Cloudflare Workers 风格

Node 函数支持：

```js
export default {
  async fetch(request, env, ctx) {
    return new Response("ok")
  }
}
```

或：

```js
export async function fetch(request, env, ctx) {
  return Response.json({ ok: true })
}
```

`request` 是标准 Web `Request`，并额外带有：

- `request.cf233.path`：原始请求路径。
- `request.cf233.remainingPath`：匹配路由之后剩余的路径。
- `ctx.remainingPath`：同上，便于函数继续分发。

## GitHub CI / Release

CI 会执行：

```sh
go test ./...
```

当推送 `v*` tag 时，会构建这些 Release 资产：

- `cloudfunction233-server_linux_amd64.tar.gz`
- `cloudfunction233-server_linux_arm64.tar.gz`
- `cloudfunction233-server_darwin_amd64.tar.gz`
- `cloudfunction233-server_darwin_arm64.tar.gz`
- `cloudfunction233-server_windows_amd64.zip`
- `cloudfunction233-server_windows_arm64.zip`

安装脚本和 `cloudfunction233-server update --repo owner/name` 都会从 GitHub 最新 Release 下载对应平台资产。
