# M2 素材库与手动工作台（2026-10-13）

自制素材上的技术记录。真实 SKU 运营验收、印尼语审核和人工播放签字都还没有做。这次也没有在 Wails 桌面窗口里亲手点完一条成片。

## 环境

macOS arm64。媒体二进制与 [M1 记录](render-baseline.md) 相同：PATH 上的 `/Users/hon/.local/bin/ffmpeg` 与 `ffprobe` 为静态构建 9.0.2。没有修改 Homebrew 安装，测试没有使用无法启动的 Homebrew FFmpeg 8.1。

本地库使用 `modernc.org/sqlite` v1.38.2，纯 Go，无 cgo。桌面应用启动时打开 `data/qingfan.sqlite`，缓存放在同级 `cache/`。设置 `QINGFAN_DATA` 后，库文件和缓存改到该目录。这两个目录已被 Git 忽略。

## 命令与结果

在仓库根目录：

```sh
go test ./internal/store/ ./internal/media/ ./internal/jobs/ -count=1 -timeout 4m
npm --prefix frontend test
npm --prefix frontend run build
```

`go test` 三个包退出码为 0，摘要为 20 项通过，约 9 秒。随后 `make test` 退出码为 0：`internal/store` 9.1 秒，`internal/media` 4.3 秒，`tests/integration` 15.2 秒，后台健康检查 1 项通过，桌面 Vitest 4 项通过，两端生产构建完成。PATH 上的 ffmpeg/ffprobe 可以执行，因此 `TestIngestRealClip` 与 `TestExportReflectsShotChange` 会实际调用它们；找不到二进制才会跳过，找到但无法启动则失败。

`npm --prefix frontend test` 为 Vitest，`SegmentCard.test.tsx` 4 项通过。`npm --prefix frontend run build` 通过类型检查并生成 `frontend/dist/`。`frontend/dist/` 除占位文件外不提交。

浏览器打开 `http://127.0.0.1:5173/`（`npm --prefix frontend run dev`）检查了无 Wails 绑定的页面：

- 项目名称、商品名称可以填写。
- 页面说明当前是浏览器预览，保存、预览和导出需要桌面应用。
- 未创建商品就点导入，分段卡片提示「先创建商品」。
- 键盘在口播框输入「旧口播」。
- 「添加镜头」后出现镜头 2。
- 镜头 1 出点改为 3500 后，「将传给导出的镜头」显示 `未选 0–3500，未选 0–0`。
- 「检查环境」提示通过 `make dev` 启动桌面应用。
- 未保存草稿就点预览或导出，提示「先保存草稿」。

## 测试覆盖的行为

- `TestImportReusesProbeAndDoesNotPromoteRights`：同一内容再次导入复用探测；参考来源保持 `reference_only`，不会被标成自有可用。
- `TestRelocateAndReplacement`：移动后可重新定位；同路径换成别的内容后，旧分析失效；哈希不一致的新位置被拒绝。
- `TestManualClipAndForeignSelection`：可人工增加镜头区间；越界被拒绝。其他商品和 reference 镜头不能进入草稿。
- `TestDraftEditDoesNotChangeExportRecord`：关闭再打开数据库后，草稿和成片记录仍在；再改草稿不会改写已导出的记录。
- `TestExportReflectsShotChange`：用 `tests/fixtures/` 的红蓝镜头、预录口播、BGM 和 Lato 字体，经真实探测和 `media.Render` 导出。预览不写入成片表，第二镜是蓝素材。把第二镜改成红素材 `[2000,4000)` 后，原路径拒绝覆盖且字节不变；新路径的清单带上新镜头和新区间。字幕保留引号与特殊字符。
- `TestIngestRealClip`：对真实短视频做场景探测、整段候选切镜和缩略图。内容哈希已有探测记录时不再调用 ffmpeg。
- `SegmentCard.test.tsx`：选择可用镜头会带上镜头编号和区间；reference 选项不可选；键盘可改口播和出点；过长配音的提示显示在页面上。

卡片上的镜头编号、入点、出点、口播、配音和字幕经 `toDraftSegment` 交给 `SaveDraft`。保存时把这些区间写成 `origin=manual`。预览和导出再交给 M1 的时间线校验与渲染。

## 限制

- 工作台本次打开时是空表单。关闭应用后草稿和成片仍在 SQLite，界面没有项目列表，不会自动把上次草稿填回来。恢复能力由关闭再打开数据库的测试证明。
- `Relocate` 和 `AddManualClip` 有桌面绑定和存储测试。界面没有单独的重新定位或新增切镜按钮；修正区间走卡片上的入点、出点和保存草稿。
- `frontend/wailsjs` 没有重新生成。新方法由 `frontend/src/api.ts` 通过 `window.go.main.App` 调用，不手改生成绑定。
- 浏览器里不能完成导入和导出。桌面窗口内的点击、预览播放和运营确认留待真人验收。
- 替换镜头后的断言看的是成片清单里的素材编号和区间，没有人眼播放签字。
- 只在这台开发机上验证，不代表其他系统已通过。
- M3 的脚本、配音和配镜另记在 [生成试点记录](generation-pilot.md)。M4 另记在 [试用记录](pilot-report.md)。本记录不包含 M4。
