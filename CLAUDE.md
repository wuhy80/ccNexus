# CLAUDE.md

本文件为 Claude Code (claude.ai/code) 在此代码库中工作时提供指导。

## 项目概述

ccNexus 是一个为 Claude Code 和 Codex CLI 设计的智能 API 端点轮换代理。它提供多个 API 端点之间的自动故障转移，支持不同 AI API 格式（Claude、OpenAI、Gemini）之间的转换，并通过 WebDAV 或 S3 提供跨设备配置同步。

**两种部署模式：**
- **桌面应用** (`cmd/desktop/`): Wails v2 图形界面，带系统托盘、会话查看器和自动更新
- **服务器应用** (`cmd/server/`): 无头 HTTP 服务，适用于 Docker/服务器部署

**开发模式说明：**
- 开发模式使用独立数据库目录 `~/.ccNexus-dev/`，不会影响正式安装版本的数据
- 使用 `dev.bat`(Windows) 或 `dev.sh`(macOS/Linux) 启动开发模式
- 或者手动设置环境变量 `CCNEXUS_DEV_MODE=1` 再运行 `wails dev`

## 开发命令

### 桌面应用开发

```bash
# 安装 Wails CLI（必需）
go install github.com/wailsapp/wails/v2/cmd/wails@latest

# 检查环境配置
wails doctor

# 安装前端依赖
cd cmd/desktop/frontend && npm install && cd ../../..

# 启动开发模式（热重载）- 使用独立数据库
# Windows:
cd cmd/desktop && dev.bat
# macOS/Linux:
cd cmd/desktop && ./dev.sh

# 或者手动设置环境变量
set CCNEXUS_DEV_MODE=1  # Windows
export CCNEXUS_DEV_MODE=1  # macOS/Linux
cd cmd/desktop && wails dev

# 为当前平台构建
cd cmd/desktop && wails build

# 为特定平台/架构构建
cd cmd/desktop && wails build -platform linux/amd64
cd cmd/desktop && wails build -platform darwin/arm64
cd cmd/desktop && wails build -platform windows/amd64
```

**Linux 构建依赖：**
```bash
sudo apt-get install libgtk-3-dev libwebkit2gtk-4.1-dev pkg-config
```

### 服务器应用开发

```bash
# 构建服务器二进制文件
cd cmd/server && go build -o ccnexus-server .

# 直接运行服务器
./ccnexus-server

# 构建 Docker 镜像
cd cmd/server && docker build -t ccnexus-server .

# 使用 docker-compose 运行
cd cmd/server && docker-compose up
```

**服务器环境变量：**
- `CCNEXUS_DATA_DIR`: 数据目录（默认：`~/.ccNexus`）
- `CCNEXUS_PORT`: HTTP 端口（默认：`3003`）
- `CCNEXUS_LOG_LEVEL`: 日志级别（`DEBUG`、`INFO`、`WARN`、`ERROR`）
- `CCNEXUS_DB_PATH`: SQLite 数据库路径

### 测试

```bash
# 运行所有测试
go test ./...

# 测试特定包
go test ./internal/proxy
go test ./internal/transformer/...

# 带覆盖率的测试
go test -cover ./...

# 详细测试输出
go test -v ./internal/proxy
```

## 架构概览

### 请求流程

```
客户端 (Claude Code/Codex CLI)
    ↓
代理服务器 (internal/proxy)
    ├─ 从路径检测客户端格式 (/v1/messages, /v1/chat/completions, /v1/responses)
    ├─ 根据客户端格式 + 端点类型选择转换器
    ├─ 将请求转换为目标 API 格式
    └─ 使用重试/故障转移逻辑发送到端点
    ↓
目标 API (Claude, OpenAI, Gemini)
    ↓
响应（流式或非流式）
    ├─ 将响应转换回客户端格式
    ├─ 提取 token 使用量
    └─ 记录统计信息
    ↓
客户端接收响应
```

### 核心组件

**internal/proxy**: 核心代理引擎
- `proxy.go`: 主服务器、端点轮换、客户端检测
- `request.go`: 请求转换、HTTP/SOCKS5 代理支持
- `streaming.go`: 服务器发送事件 (SSE) 流式传输，带状态转换
- `response.go`: 非流式响应处理
- `stats.go`: 使用 SQLite 持久化的统计跟踪
- 重试逻辑：每个端点重试 2 次，然后轮换到下一个端点

**internal/transformer**: API 格式转换系统
- 类插件架构，带转换器接口
- 三个客户端系列：
  - `cc/`: Claude Code → 目标 API (claude, openai, openai2, gemini)
  - `cx/chat/`: Codex Chat → 目标 API
  - `cx/responses/`: Codex Responses → 目标 API
- `convert/`: 所有 API 格式之间的双向转换工具
- `StreamContext`: 流式转换的有状态对象（跟踪消息索引、工具调用、thinking 块）

**internal/service**: 业务逻辑层
- 通过 Wails 绑定（桌面）或 HTTP API（服务器）向 UI 暴露操作
- `endpoint.go`: 端点 CRUD、测试、模型获取
- `backup.go`: 多提供商备份编排（本地、S3、WebDAV）
- `webdav.go`: 带冲突检测的 WebDAV 同步
- `stats.go`: 统计聚合和报告
- `archive.go`: 历史统计管理

**internal/storage**: SQLite 持久化层
- Schema: `endpoints`、`daily_stats`（设备感知）、`app_config`（键值）
- 使用适配器模式实现线程安全
- 启动时进行 schema 迁移

**internal/config**: 线程安全的配置管理
- 端点、WebDAV、S3 备份、终端设置
- 区分设备特定配置和安全（可同步）配置键

**internal/session**: Claude Code 会话跟踪
- 解析 `~/.claude/sessions/` 中的会话文件
- 提供别名管理和路径编码

### 关键设计模式

1. **转换器注册表**: 全局注册表映射 (clientFormat, endpointType) → 转换器实例
2. **有状态流式传输**: `StreamContext` 在 SSE 事件之间维护状态以实现复杂转换
3. **设备感知统计**: 每个安装都有唯一的 `device_id`，统计按设备跟踪但可以聚合
4. **安全配置备份**: 仅同步平台无关的设置（排除设备特定路径、代理设置）
5. **优雅的端点轮换**: 在轮换端点之前等待活动请求完成
6. **冲突解决**: WebDAV/S3 恢复提供两种策略：`keep_local` 或 `overwrite_local`

### 添加新的 API 提供商

1. 在 `internal/transformer/types.go` 中定义类型（请求/响应结构体）
2. 在 `internal/transformer/cc/newprovider.go` 中创建转换器：
   - 实现 `TransformRequest`、`TransformResponse`、`TransformResponseWithContext`
   - 处理流式和非流式两种情况
3. 在 `internal/transformer/convert/claude_newprovider.go` 中添加转换逻辑
4. 在 `internal/transformer/registry.go` 的 init() 中注册转换器
5. 向 UI 添加转换器选项（桌面：frontend，服务器：webui）

### 特殊处理

**工具调用**: 代理通过 `cleanIncompleteToolCalls()` 自动删除不完整的工具调用，以防止 API 错误。

**思考/推理**: 扩展思考块在不同格式之间转换（`<thinking>` 标签 vs. 专用思考内容块）。

**图像支持**: Base64 图像和图像 URL 在 `convert/common.go` 中在不同 API 格式之间转换。

**Token 估算**: 当 API 不返回 token 计数时，代理使用 `internal/tokencount` 进行估算。

## 测试代理

```bash
# 启动代理（桌面或服务器）
# 桌面：打开 ccNexus.app / ccNexus.exe
# 服务器：./ccnexus-server

# 使用 curl 测试（Claude 格式）
curl -X POST http://localhost:3003/v1/messages \
  -H "Content-Type: application/json" \
  -H "x-api-key: any-key" \
  -d '{"model":"claude-3-5-sonnet-20241022","max_tokens":100,"messages":[{"role":"user","content":"Hello"}]}'

# 使用 curl 测试（OpenAI Chat 格式）
curl -X POST http://localhost:3003/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer any-key" \
  -d '{"model":"gpt-4","messages":[{"role":"user","content":"Hello"}]}'

# 使用 curl 测试（OpenAI Responses 格式）
curl -X POST http://localhost:3003/v1/responses \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer any-key" \
  -d '{"model":"gpt-4","prompt":"Hello"}'

# 健康检查
curl http://localhost:3003/health
```

## 数据库 Schema

```sql
-- 端点配置
endpoints (
  id INTEGER PRIMARY KEY,
  name TEXT UNIQUE,
  api_url TEXT,
  api_key TEXT,
  enabled INTEGER,
  transformer TEXT,
  model TEXT,
  remark TEXT,
  sort_order INTEGER,
  created_at TIMESTAMP,
  updated_at TIMESTAMP
)

-- 设备感知统计
daily_stats (
  id INTEGER PRIMARY KEY,
  endpoint_name TEXT,
  date TEXT,
  requests INTEGER,
  errors INTEGER,
  input_tokens INTEGER,
  output_tokens INTEGER,
  device_id TEXT,
  created_at TIMESTAMP,
  UNIQUE(endpoint_name, date, device_id)
)

-- 应用程序配置（键值）
app_config (
  key TEXT PRIMARY KEY,
  value TEXT,
  updated_at TIMESTAMP
)
```

## 常见陷阱

**端点轮换**: 代理在错误时轮换端点，但会等待活动请求。测试故障转移时，请确保所有请求完成后再检查轮换。

**转换器选择**: 转换器基于客户端格式（从路径检测）和端点转换器设置两者选择。如果转换失败，请验证转换器链是否有效。

**流式上下文**: 修改流式转换器时，请记住 `StreamContext` 在事件之间是有状态的。适当地重置索引/缓冲区。

**CGO 要求**: 桌面构建需要 CGO（用于 SQLite 和 WebView）。服务器构建也需要 CGO 用于 SQLite。使用 `CGO_ENABLED=1`。

**WebDAV 冲突**: 从备份合并时，如果端点名称匹配但设置不同，则会发生冲突。始终使用两种策略测试冲突解决逻辑。

**会话解析**: Claude Code 会话文件使用 base64 编码的路径。使用 `internal/session` 包方法而不是手动解析。
