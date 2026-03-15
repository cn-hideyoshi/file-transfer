# file-transfer

一个用 Go 标准库实现的 HTTP 本地文件代理服务，可以把指定目录映射成 HTTP 访问地址。

## 功能

- `GET /healthz`：健康检查
- `GET /files/`：浏览器打开网页目录页
- `GET /files/<path>`：访问目录或读取文件
- `GET /files/<path>?download=1`：强制下载文件
- `GET /files/?format=json`：返回目录 JSON

目录默认响应 HTML 页面，文件响应为原始文件内容；如果带 `format=json`，则返回机器可读的 JSON。

## 启动

```bash
go run ./cmd/file-transfer -root ./shared -addr :8080
```

也可以用环境变量：

```bash
FILE_ROOT=./shared LISTEN_ADDR=:8080 go run ./cmd/file-transfer
```

## 目录结构

```text
.
|-- cmd/file-transfer/         # 程序入口
|-- internal/fileproxy/       # HTTP 服务与文件代理逻辑
|-- internal/fileproxy/templates/
|-- bin/                      # 本地编译产物
|-- README.md
`-- go.mod
```

## 示例

假设本地目录 `./shared` 下有文件 `hello.txt`：

```bash
curl http://127.0.0.1:8080/files/hello.txt
```

查看目录：

```bash
open http://127.0.0.1:8080/files/
```

查看目录 JSON：

```bash
curl http://127.0.0.1:8080/files/?format=json
```

强制下载：

```bash
curl -OJ "http://127.0.0.1:8080/files/hello.txt?download=1"
```

## 安全说明

服务只允许访问 `-root` 或 `FILE_ROOT` 指定目录内的文件，已做路径清理，不能通过 `../` 越出根目录。
