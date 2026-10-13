# Repository Guidelines

## 项目范围与技术栈

青帆优先实现商品视频混剪；发布和效果回流可人工配合，不依赖 TikTok API。按 [开发计划](docs/plans/2026-10-13-qingfan-mvp-development-plan.md) 推进。M1 已能用固定时间线和预录音频渲染竖屏样片。M2 已有本地 SQLite 素材库和手动工作台。真实 SKU 运营验收、AI 和认证尚未完成。

- Go ≥ 1.25、Wails 2.16.0；Node 24、npm 11。
- React 19.3、TypeScript 7.0.2（strict）、Vite 8.3；后台使用 Ant Design 6.6.4。
- 媒体处理用 FFmpeg/ffprobe；本地业务数据使用 SQLite（`modernc.org/sqlite`，无 cgo）。
- 依赖版本以 `go.mod`、两端 `package.json` 和锁文件为准；变更依赖须同步锁文件。

## 目录与架构边界

- `main.go`、`app.go`：Wails 入口与薄绑定层；媒体业务放入 `internal/`，供桌面与 CLI 共用。
- `frontend/`：桌面工作台；`frontend/wailsjs/` 为生成绑定，不手改接口。
- `admin/`：独立管理后台；优先复用现有 Ant Design 组件，来源见 [复用说明](docs/admin-reuse.md)。
- `cmd/qingfan-render/`：`doctor` 与固定时间线 `render`；`cmd/qingfan-server/`：当前仅有 `/healthz`。
- `tests/fixtures/`：可分发样本；`tests/integration/`：真实媒体测试；`docs/validation/`：验收证据。

## 开发命令

```sh
make setup        # 安装依赖
make doctor       # 检查 FFmpeg/ffprobe
make dev          # 启动桌面端，需 Wails 平台依赖
make build        # 打包到 build/bin/
make server       # 本机服务 :8090
make admin-dev    # 后台 :5188，另开终端运行
make admin-build  # 后台产物 admin/dist/
make test         # Go 测试、后台测试、桌面组件测试、两端类型检查和构建
```

## 编码与安全规范

- Go 用 `gofmt`、小写包名、显式错误处理；TypeScript 用两空格缩进、PascalCase 组件、camelCase 函数，禁止用 `any` 绕过边界校验。尚未配置 ESLint/Prettier。
- 外部输入和 AI 响应必须校验；前端权限控制不能替代服务端认证授权。
- FFmpeg 使用 `exec.CommandContext` 和参数数组，禁止拼接 shell；保留失败原因，支持取消和超时。
- 媒体时间统一毫秒、左闭右开区间；禁止越界取镜、跨商品兜底或静默跳过必要音视频及字幕。
- 不覆盖原素材和成功导出；密钥仅存服务端，私有素材、数据库和运行产物不入 Git。

## 测试与提交

Go 使用 `testing`，文件命名 `*_test.go`；后台请求测试使用 Node 内置测试器（`npm --prefix admin test`）。桌面分段卡片使用 Vitest（`npm --prefix frontend test`）。新逻辑覆盖成功及关键失败路径；媒体验收必须检查真实输出并播放，不能用 mock 代替。无固定覆盖率门槛。

提交前运行相关测试和 `git diff --check`。缺失媒体工具可明确跳过，已安装但损坏必须报错，不能改测试掩盖环境故障。

提交格式：`feat|fix|docs|chore: 简述变更`。PR 写明里程碑、行为变化、验证结果和限制；关联计划或 issue，界面变更附截图，渲染变更附样片证据。
