# M1 固定时间线样片（2026-10-13）

自制素材的技术出片记录。真实 SKU 运营验收、印尼语母语审核和人工播放签字都还没有做。

## 环境

macOS arm64。实际使用的二进制：

- `/Users/hon/.local/bin/ffmpeg`：`ffmpeg version 9.0.2-https://www.martin-riedl.de`
- `/Users/hon/.local/bin/ffprobe`：同一静态构建

本机 Homebrew FFmpeg 8.1 与 ffprobe 仍因缺少 `libx265.215.dylib` 无法启动。没有修改 Homebrew 安装，样片和测试都没有使用那一对二进制。`qingfan-render doctor` 报告的是 PATH 上能执行 `-version` 的上述静态构建。

## 命令

在仓库根目录，对同一时间线和资源清单启动两次独立进程：

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

两次都成功。标准输出是成片清单。时间线是一段 4000 毫秒、两个镜头（红 `[0,2000)`、蓝 `[500,2500)`），口播 3500 毫秒，两条字幕，BGM 增益 -18 dB。编码器是 libx264、veryfast、crf 23、yuv420p、1080×1920、30fps、threads=1。

## 产物

| 路径 | 说明 |
| --- | --- |
| `outputs/m1-sample-a.mp4` | 第一次导出，123036 字节 |
| `outputs/m1-sample-a.manifest.json` | 对应清单，时长 4000 毫秒 |
| `outputs/m1-sample-b.mp4` | 第二次独立导出，123036 字节 |
| `outputs/m1-sample-b.manifest.json` | 对应清单 |
| `tests/fixtures/timelines/sample.json` | 固定时间线 |
| `tests/fixtures/timelines/sample.assets.json` | 资源清单与 SHA-256 |

`outputs/` 被 Git 忽略，MP4 和清单不提交。

ffprobe 两次结果相同：视频 h264，宽 1080，高 1920，`avg_frame_rate` 与 `r_frame_rate` 都是 `30/1`，120 帧；一条 aac 音轨；一条 `mov_text` 字幕。容器时长 `4.000000` 秒，与清单 `duration_ms` 4000 相差 0，不超过一帧（1/30 秒）。

从成片抽出 1 秒处的一帧为 1080×1920 PNG，原始 rgb24 长度为 1080×1920×3。抽样像素不止一种颜色。画面上下有黑边，因为 320×180 的横屏色块被缩放到竖屏画布。

读回的字幕：

```
00:00:00,000 --> 00:00:02,000  Susah membuka tutup botol?
00:00:02,000 --> 00:00:03,500  Coba "cara" ini: 100% & ok
```

引号、百分号和 `&` 留在字幕文本里。两条字幕都落在 4000 毫秒的段内。

清单记录了镜头、口播、BGM 和 Lato 字体的 SHA-256，以及渲染器 `qingfan-render` `m1-libx264-1`、FFmpeg 版本与路径、编码器配置。质检字段 `black_frames` 和 `silence` 都存在，这次是空数组：色块加黑边没有达到整帧黑场阈值，-18 dB 的循环 BGM 也没有触发静音检测。空记录本身不判失败。

## 一致性

比较方法不是整个 MP4 的文件哈希。对两个文件分别计算：

- `ffmpeg -map 0:v:0 -f framemd5` 的 SHA-256
- 解码为 48 kHz、双声道、s16le 的 SHA-256

两次画面哈希都是 `ddb70451b310600b516a80062c3b36846216cc508908a92d3d4b8ac4c4078412`。两次音频哈希都是 `fbc130fce70a623f9f191765d099c6f8a121ba86f95f5954dc6ecc85038b2c00`。解码后的画面和音频一致。该结论只针对这一对相同构建、相同输入的 libx264 输出。

## 已知限制

- 素材是 FFmpeg 纯色和正弦波，加上 SIL OFL 1.1 的 Lato Regular。不能代替真实商品镜头，也没有人工观看签字。
- 原素材音轨策略只接受 `mute`。这两条样片镜头本身没有音轨，渲染记录了该说明并继续出片。
- 口播短于片段时补静音到段尾。样片口播 3500 毫秒，片段 4000 毫秒。
- 横屏画面按画布缩小并加黑边，不裁成满屏。可变帧率会在其他检查里被规整到时间线帧率并留下说明；这条样片的源是恒定 30fps。
- 黑场和静音只记录，不单独导致失败。故意的留白不会因此被拒。
- 内容一致只在同一 FFmpeg/libx264 构建上比较解码结果。不承诺不同编码器或不同机器的字节级相同。
- 未做 EBU R128、逐词字幕、转场、变速、硬件编码和 TikTok 发布。
