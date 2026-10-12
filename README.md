# 青帆 Qingfan

面向 TikTok Shop 运营的商品视频混剪工具。首版聚焦印尼市场：导入自有素材，参考视频结构，生成和修改口播，自动配镜、配音、加字幕并在本地导出。

## 当前状态

工程骨架已初始化：Wails 桌面端、媒体环境检查 CLI、Go 健康检查服务，以及复用 tk-flywheel 的独立管理后台。M1 可以用固定时间线和预录音频在本地渲染 1080×1920、30fps 的竖屏样片。AI、素材库和工作台业务尚未实现。首版不依赖 TikTok API；发布与回流通过人工发布、链接绑定和 CSV 导入完成。真实 SKU 的运营验收仍未进行。

- [架构设计](<docs/TK 电商素材生产与效果回流系统 · 架构设计.md>)：范围、媒体流水线、时间线协议和后续方向。
- [开发计划](docs/plans/2026-10-13-qingfan-mvp-development-plan.md)：M0–M4 的实现顺序、文件边界和验收条件。

## 技术方向

桌面端使用 Wails v2、Go 和 React/TypeScript；媒体处理使用 FFmpeg/ffprobe，本地业务数据使用 SQLite。模型调用经 Go 服务完成。流程参考 [NarratoAI](https://github.com/linyqh/NarratoAI)，已研究的提交和具体借鉴边界记录在架构文档中。

## 开发起点

先执行开发计划的 M0、M1：准备一个真实 SKU 的素材和人工基线，用固定时间线验证配镜、配音、字幕、BGM 和本地导出，再开发桌面工作台与 AI 链路。

## 本地开发

要求 Go ≥ 1.25、Node 24、npm 11、能实际运行的 FFmpeg/ffprobe（需带 libx264），以及 [Wails 平台依赖](https://wails.io/docs/gettingstarted/installation/)。Wails 固定为 2.16.0，由 Makefile 调用，不需全局安装。

`ffmpeg` 和 `ffprobe` 必须能执行 `-version`。只在 PATH 里找到文件但进程无法启动时，`make doctor` 和媒体测试会失败，不会跳过。本机 Homebrew FFmpeg 8.1 曾因缺少 `libx265.215.dylib` 无法启动；当时改用独立的静态构建，没有修改 Homebrew 安装。样片记录里写明实际使用的二进制。

```sh
make setup       # 安装 Go 和两个前端的锁定依赖
make doctor      # 检查 FFmpeg / ffprobe，缺失时非零退出
make dev         # 启动 Wails 桌面应用
make build       # 桌面发布构建，输出 build/bin/
make test        # Go 测试、后台请求测试、两个前端类型检查及构建
```

管理后台分别在两个终端启动：

```sh
make server      # http://127.0.0.1:8090/healthz
make admin-dev   # http://127.0.0.1:5188，Vite 代理健康检查
```

`make admin-build` 生成 `admin/dist/`。当前服务不托管该目录；生产部署及认证留到 M3。桌面浏览器预览没有 Wails 绑定，实际环境检查需在 `make dev` 启动的桌面窗口操作。

## 代码结构

- `main.go`、`app.go`、`wails.json`：桌面入口与 Go 绑定。
- `frontend/`：桌面 React 工作台；`wailsjs/` 为生成的绑定。
- `admin/`：独立 React + Ant Design 管理后台，见[复用说明](docs/admin-reuse.md)。
- `cmd/qingfan-render/`：`doctor` 检查媒体工具；`render` 按固定时间线导出样片。
- `cmd/qingfan-server/`：当前仅提供 `/healthz`。
- `internal/timeline/`：时间线 v1 的类型、校验、时长和 30fps 边界。
- `internal/media/`：探测、裁切装配、字幕、混音、libx264 合成和基础质检。
- `tests/integration/`：真实 FFmpeg/ffprobe 检查；工具不在 PATH 时跳过，找到但无法启动时失败。
- `tests/fixtures/`：自制短视频、音频、字体和时间线。真实商品素材不在这里。

## 渲染与测试

在仓库根目录运行。时间线使用预录音频，不调用模型。

```sh
go test ./internal/timeline/ ./internal/media/ ./cmd/qingfan-render/ ./tests/integration/
```

`make test` 还会跑后台请求测试和两个前端的类型检查与构建。`make doctor` 检查当前 PATH 上的 FFmpeg/ffprobe。

生成自制样片（两次使用同一时间线和资源，输出路径不同，便于比较内容）：

```sh
go run ./cmd/qingfan-render render \
  -timeline tests/fixtures/timelines/sample.json \
  -assets tests/fixtures/timelines/sample.assets.json \
  -output outputs/m1-sample-a.mp4

go run ./cmd/qingfan-render render \
  -timeline tests/fixtures/timelines/sample.json \
  -assets tests/fixtures/timelines/sample.assets.json \
  -output outputs/m1-sample-b.mp4
```

成功时标准输出是成片清单 JSON，同目录还会写下 `m1-sample-a.manifest.json`。清单含原素材、配音、BGM、字体的内容哈希，以及渲染器、FFmpeg 和编码器配置。`outputs/` 已忽略，不提交导出的 MP4。比较两次结果时解码画面和音频，不用整个文件哈希判断是否一致。命令、ffprobe 和已知限制见 [样片验证记录](docs/validation/render-baseline.md)。

时间线、持久化、任务、AI 和回流包随各里程碑创建。贡献与技术规范见 [AGENTS.md](AGENTS.md)，工程状态和运行说明见本 README。

真实商品素材、密钥、本地数据库和导出文件放在 Git 忽略的运行目录中；测试仅提交自制或明确允许分发的最小样本。依赖锁文件随应用代码提交。

本次构建、测试结果及本机 FFmpeg 依赖问题见[工程验证记录](docs/validation/2026-10-13-scaffold.md)。
