# Repository Guidelines（仓库指引）

## 项目结构与模块组织
ccNexus 是基于 Go 1.24 的单体仓库：`cmd/desktop` 承载 Wails 桌面客户端（`main.go`、`app.go`、`wails.json`）及其前端 `cmd/desktop/frontend`（`src/modules`、`src/i18n`、`src/themes`）。`cmd/server` 提供纯 HTTP 代理与 Docker 发布入口（`Dockerfile`、`docker-compose.yml`、`webui`）。核心能力位于 `internal/`（proxy、transformer、storage、webdav、tray 等），文档与素材位于 `docs/`，桌面构建产物默认输出到 `cmd/desktop/build/bin`。

## 构建、测试与开发命令
- `wails doctor`：检查本机 Go/Node/WebView 依赖。
- `cd cmd/desktop/frontend && npm install`：安装或更新 Vite 依赖。
- `cd cmd/desktop && wails dev`：以热重载方式同时调试 Go 与前端。
- `cd cmd/desktop && wails build -platform windows/amd64`（或 darwin/linux）：生成对应平台二进制到 `build/bin`。
- `cd cmd/server && go run .`：运行仅含 API 的代理实例。
- `npm run build`（在 `cmd/desktop/frontend`）：编译供 Wails 使用的优化静态资源。

## 代码风格与命名约定
后端遵循 Go 规范：使用制表符缩进，导出符号采用 PascalCase，私有 helper 尽量局限于所属 `internal/*` 包。提交前运行 `gofmt`/`goimports`，大文件拆分为 `internal/<feature>` 子包。前端是基于 Vite 的 ES Module：组件目录集中在 `src/modules`，组件文件名用 PascalCase，样式文件 kebab-case，所有字符串放入 `src/i18n`。配置结构体字段需与 JSON/YAML 键一一对应，避免随意重命名。无论代码还是文档，统一使用 UTF-8 编码保存，避免在 Windows 默认 UTF-16 输出导致中文乱码。

## 测试准则
后端新增功能需在同目录添加表驱动 `*_test.go`，复杂数据放置于 `testdata/`。提交前执行 `go test ./...`，重点覆盖 proxy、transformer、storage 等关键路径。前端暂无线测套件，修改 UI 时请运行 `wails dev` 做交互冒烟，并确认 `npm run build` 成功。若改动 Docker/server，请使用 `curl http://localhost:3003/health` 校验健康检查。

## 提交与 Pull Request 规范
遵循现有历史，提交信息以类型开头（`feat:`、`fix:`、`Hotfix:`、`Feature:`），后接简洁描述，可附 `(#issue)`。版本发布使用 `vX.Y.Z` 标签。PR 统一指向 `master`，说明改动范围、测试结果及平台注意事项，涉及 UI 的附上截图或 GIF。确认 GitHub Actions “Build and Release” 全平台通过后再申请评审。

## 安全与配置提示
勿提交真实 API Key 或 `~/.ccNexus` 数据，日志与截图需脱敏。端点凭据保存在本地配置（参见 `docs/configuration*.md`）或 Docker secrets，切勿硬编码。部署 server 版本时在上游终止 TLS，并通过配置禁用未使用的 transformer 以降低攻击面。

