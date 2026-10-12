# 青帆 Qingfan

面向 TikTok Shop 运营的商品视频混剪工具。首版聚焦印尼市场：导入自有素材，参考视频结构，生成和修改口播，自动配镜、配音、加字幕并在本地导出。

## 当前状态

仓库已初始化，当前交付为架构和开发计划，应用代码尚未实现。首版不依赖 TikTok API；发布与回流通过人工发布、链接绑定和 CSV 导入完成。

- [架构设计](<docs/TK 电商素材生产与效果回流系统 · 架构设计.md>)：范围、媒体流水线、时间线协议和后续方向。
- [开发计划](docs/plans/2026-10-13-qingfan-mvp-development-plan.md)：M0–M4 的实现顺序、文件边界和验收条件。

## 技术方向

桌面端使用 Wails v2、Go 和 React/TypeScript；媒体处理使用 FFmpeg/ffprobe，本地业务数据使用 SQLite。模型调用经 Go 服务完成。流程参考 [NarratoAI](https://github.com/linyqh/NarratoAI)，已研究的提交和具体借鉴边界记录在架构文档中。

## 开发起点

先执行开发计划的 M0、M1：准备一个真实 SKU 的素材和人工基线，用固定时间线验证配镜、配音、字幕、BGM 和本地导出，再开发桌面工作台与 AI 链路。

后续初始化应用时记录并固定 Go、Node、Wails 与 FFmpeg 版本。Wails 环境要求以[官方安装文档](https://wails.io/docs/gettingstarted/installation/)为准。当前没有可运行的应用命令或自动化测试。

真实商品素材、密钥、本地数据库和导出文件放在 Git 忽略的运行目录中；测试仅提交自制或明确允许分发的最小样本。依赖锁文件随应用代码提交。
