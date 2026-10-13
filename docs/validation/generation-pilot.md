# 生成试点记录

日期：2026-10-13

里程碑：M3 的调用边界、固定响应和按配音时长配镜。真实供应商、真实 SKU 和印尼语母语审核未完成。

## 这次证明了什么

在这台开发机上，`go test ./internal/ai ./internal/timeline ./internal/credentials ./internal/store ./cmd/qingfan-server ./tests/integration` 通过。其中 `tests/integration/generation_test.go` 使用仓库里的自制红、蓝镜头、BGM 和 Lato 字体，不使用真实商品素材。

- 未带令牌和已撤销令牌访问 `POST /v1/scripts` 得到 401。日志里没有引导密钥和令牌明文。
- 固定响应仍经过同一套校验：候选为 1 到 3 条，未知镜头不会被换成目录里的第一个镜头，画面不够时返回缺口并且不产出镜头。
- 改口播后中文回译失效；重新回译后再合成。合成音频按探测到的时长配镜，镜头区间之和等于该时长。
- 导出文件可由 ffprobe 读取，画面为 1080×1920、约 30fps，并且有音轨。成片时长与配音时长的差不超过一帧。
- 桌面组件测试覆盖了回译失效、镜头修正和多段草稿保留。浏览器里没有 Wails 绑定时，点击「生成候选」会提示需要在桌面应用里打开。

固定响应的印尼语句子是 `Lihat barang ini.`，中文是「这是固定回译，需要人工核对。」这不是模型翻译，也不能当作音色盲测结果。

## 命令

```sh
go test ./internal/ai ./internal/timeline ./internal/credentials ./internal/store ./cmd/qingfan-server ./tests/integration
npm --prefix frontend test
```

服务入口：

```sh
make server
```

默认监听 `127.0.0.1:8090`。用量库默认是 `data/usage.sqlite`，可用 `-usage` 改路径。供应商地址和密钥来自 `QINGFAN_PROVIDER_URL`、`QINGFAN_PROVIDER_KEY`；没有地址时使用固定响应。发放令牌需要请求头 `X-Qingfan-Bootstrap`，其值来自环境变量 `QINGFAN_BOOTSTRAP`，不写入日志。`-public` 且没有 `-tls-cert` 与 `-tls-key` 时拒绝启动。本机试用可以不开启 `-public`。

桌面端在第一次生成时把工作空间令牌放进系统凭据库。测试使用内存凭据，不调用 macOS 钥匙串。

## 限制

- 没有供应商凭据，没有做过真实印尼语 TTS 盲测，也没有选定可上线的音色。
- 抽帧失败会挡住生成。固定响应不读取这些图片，因此这不是视觉模型验收。
- 没有真实 SKU，没有母语者核对自然度、商品名或数字，也没有运营采用或拒绝记录。不能把生成出的文件当成可用成片。
- 没有人眼播放签字。桌面窗口里的完整点击路径未走通。
- 管理后台仍然只检查 `/healthz`。用量库在服务端，页面上没有令牌表单，也不保存供应商密钥。
- 只在这台开发机上验证。M4 的恢复、编码回退、CSV 和试用包另记在 [试用记录](pilot-report.md)。本记录不包含那些检查。
