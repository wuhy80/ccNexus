# Repository Guidelines（仓库指引）

## 项目结构与模块组织

ccNexus 是基于 Go 1.24 的单体仓库，包含桌面客户端和服务器端两个主要组件：

- `cmd/desktop` - Wails 桌面客户端（Go + Vite 前端）
  - `main.go` - 程序入口，Wails 应用配置
  - `app.go` - 应用逻辑和前后端绑定
  - `wails.json` - Wails 构建配置
  - `frontend/` - Vite + ES Module 前端
    - `src/modules/` - Vue/React 组件（PascalCase 命名）
    - `src/i18n/` - 国际化字符串
    - `src/themes/` - 主题样式（kebab-case 命名）
- `cmd/server` - 纯 HTTP 代理服务（Docker 部署入口）
  - `main.go` - 服务器入口
  - `Dockerfile` / `docker-compose.yml` - 容器化配置
  - `webui/` - 服务器管理界面
- `internal/` - 核心业务逻辑（Go 模块）
  - `proxy/` - 代理核心逻辑
  - `config/` - 配置管理
  - `storage/` - SQLite 存储
  - `transformer/` - API 格式转换器
  - `cache/` / `ratelimit/` / `tokencount/` - 辅助功能
- `docs/` - 文档与素材

桌面构建产物默认输出到 `cmd/desktop/build/bin/`。

## 构建、测试与开发命令

### 环境检查与初始化
```bash
# 检查本机 Go/Node/WebView 依赖
wails doctor

# 安装前端依赖
cd cmd/desktop/frontend && npm install
```

### 桌面应用开发
```bash
cd cmd/desktop

# 热重载开发模式（同时调试 Go 和前端）
wails dev

# 构建桌面应用（指定平台）
wails build -platform windows/amd64
wails build -platform darwin/amd64
wails build -platform darwin/arm64
wails build -platform linux/amd64

# 仅构建前端
npm run build  # 在 frontend/ 目录下执行
```

### 服务器端开发
```bash
cd cmd/server

# 运行服务器（默认端口 3003）
go run .

# 健康检查
curl http://localhost:3003/health

# Docker 构建
docker build -t ccnexus .
docker-compose up
```

### Go 测试
```bash
# 运行所有测试
go test ./...

# 运行单个包的测试
go test ./internal/proxy

# 运行单个测试函数
go test ./internal/proxy -run TestFunctionName

# 运行测试并显示详细输出
go test -v ./internal/config

# 运行测试并生成覆盖率报告
go test -cover ./...
go test -coverprofile=coverage.out ./... && go tool cover -html=coverage.out
```

### 代码质量与格式化
```bash
# 格式化 Go 代码（必须使用制表符缩进）
gofmt -w .

# 自动管理 imports（按标准库、第三方库、内部包分组）
goimports -w .

# 运行 go vet 静态检查
go vet ./...

# 检查常见错误（需安装: go install golang.org/x/lint/golint@latest）
golint ./...

# 完整检查（格式化 + vet）
gofmt -w . && go vet ./...
```

## 代码风格与命名约定

### Go 后端规范

#### 格式化与缩进
- **使用制表符（Tab）缩进**，不是空格
- 每行最大长度建议 120 字符
- 大括号格式：`if condition {`（同行）
- 所有文件必须 UTF-8 编码

#### 命名约定
- 导出符号：PascalCase（`GetConfig`, `ProxyServer`）
- 私有符号：camelCase（`getConfig`, `proxyServer`）
- 包名：小写，简短，无下划线（`proxy`, `config`, `tokencount`）
- 文件命名：snake_case（`session_affinity.go`, `token_count.go`）
- 接口名：方法名 + `er`（`Transformer`, `Storage`）或名词
- 常量：PascalCase（`EndpointStatusAvailable`）

#### 导入组织
导入必须按以下顺序分组，每块之间空一行：
```go
import (
    // 1. 标准库（按字母排序）
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "time"

    // 2. 第三方库（按字母排序）
    "github.com/wailsapp/wails/v2"
    "modernc.org/sqlite"

    // 3. 内部包（按字母排序）
    "github.com/lich0821/ccNexus/internal/config"
    "github.com/lich0821/ccNexus/internal/logger"
)
```

#### 错误处理
- 错误处理尽早返回，避免深层嵌套
- 使用 `fmt.Errorf()` 添加上下文：
  ```go
  if err != nil {
      return fmt.Errorf("failed to open database: %w", err)
  }
  ```
- 自定义错误类型在包内定义，使用 `errors.Is()` 和 `errors.As()` 检查
- HTTP 处理函数中使用 `http.Error()` 返回错误响应

#### 结构体与接口
- 结构体字段顺序：导出字段在前，私有在后
- JSON 标签必须与字段名对应：
  ```go
  type Endpoint struct {
      Name   string `json:"name"`
      APIKey string `json:"apiKey"`
  }
  ```
- 接口定义简短，方法名清晰（`TransformRequest`, `TransformResponse`）

#### 并发安全
- 共享状态使用 `sync.RWMutex` 保护
- 使用 `atomic` 包进行简单计数器操作
- 通道使用有缓冲通道避免阻塞

### 前端规范（Vite + ES Module）

#### 文件组织
- 组件目录：`src/modules/`（PascalCase 命名，如 `EndpointList.vue`）
- 样式文件：`kebab-case.css`（如 `endpoint-list.css`）
- 所有 UI 字符串放入 `src/i18n/`
- 工具函数：`camelCase.js`

#### 代码风格
- 使用 ES6+ 语法
- 缩进：2 个空格
- 字符串使用单引号
- 末尾加分号

## 测试准则

### Go 测试
- 使用表驱动测试模式
- 测试文件：`*_test.go`，与被测试文件同目录
- 复杂测试数据放入 `testdata/` 子目录
- 测试函数命名：`TestFunctionName`（公开）或 `testHelper`（私有）
- 使用 `_ = json.NewDecoder(r.Body).Decode(&resp)` 忽略非关键错误

### 前端测试
- 前端暂无线测套件
- 修改 UI 后使用 `wails dev` 做交互冒烟测试
- 确认 `npm run build` 成功无警告

### 测试覆盖率要求
- 核心逻辑（proxy、transformer、storage）重点覆盖
- 提交前执行 `go test ./...` 确保全部通过

## 提交与 Pull Request 规范

### 提交信息格式
```
<类型>: <简短描述> (#issue)

[可选的详细描述]
```

- 类型：`feat:`（新功能）、`fix:`（修复）、`Hotfix:`（紧急修复）、`Feature:`（大功能）
- 示例：`feat: add endpoint priority routing (#42)`

### PR 规范
- 目标分支：`master`
- PR 描述包含：改动范围、测试结果、平台注意事项
- UI 改动附截图或 GIF
- 确认 GitHub Actions "Build and Release" 通过

### 版本发布
- 使用 `vX.Y.Z` 标签触发自动构建
- 遵循语义化版本规范

## 安全与配置提示

- **绝不提交**真实 API Key 或 `~/.ccNexus` 目录数据
- 日志和截图需脱敏处理（隐藏 API keys、tokens）
- 端点凭据保存在本地配置或 Docker secrets，**切勿硬编码**
- 部署 server 版本时在上游终止 TLS
- 通过配置禁用未使用的 transformer 降低攻击面

## 调试技巧

### 环境变量
```bash
# 开发模式（使用独立数据库）
export CCNEXUS_DEV_MODE=1

# 无代理模式（使用生产数据库，仅测试 UI）
export CCNEXUS_NO_PROXY=1

# Wails 开发模式（由 wails dev 自动设置）
export WAILS_ENVIRONMENT=development
```

### 数据库位置
- Windows: `%USERPROFILE%\.ccNexus\ccnexus.db`
- macOS/Linux: `~/.ccNexus/ccnexus.db`
- 开发模式: `~/.ccNexus-dev/ccnexus.db`

### 日志级别
通过配置或环境变量设置：`debug`, `info`, `warn`, `error`
