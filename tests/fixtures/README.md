# 测试素材

这里只放自制或允许再分发的最小样本。真实商品素材放在根目录 `data/`，不提交到 Git。

## M1 固定样片

`media/` 与 `timelines/sample.json` 用来验证固定时间线渲染，不是真实 SKU，也不能当作运营验收。

| 文件 | 来源 | 用途 |
| --- | --- | --- |
| `media/clip_red.mp4` | FFmpeg `color=0xCC3322:s=320x180:r=30`，120 帧，无音轨 | 第一镜，取 `[0,2000)` 毫秒 |
| `media/clip_blue.mp4` | FFmpeg `color=0x2266CC:s=320x180:r=30`，120 帧，无音轨 | 第二镜，取 `[500,2500)` 毫秒 |
| `media/voice_demo.wav` | FFmpeg `sine=frequency=440:sample_rate=48000:duration=3.5`，PCM | 预录口播，时间线声明 3500 毫秒 |
| `media/bgm_demo.wav` | FFmpeg `sine=frequency=196:sample_rate=48000:duration=2`，PCM | BGM，短于成片，渲染时循环并按 -18 dB 混入 |
| `fonts/Lato-Regular.ttf` | [Lato](https://github.com/google/fonts/tree/main/ofl/lato)，SIL OFL 1.1，见 `fonts/OFL.txt` | 烧录字幕的字体 |

时间线和资源清单是 `timelines/sample.json`、`timelines/sample.assets.json`。清单里的 SHA-256 对应当前文件字节。重新生成样本后必须更新哈希，否则渲染会在启动 FFmpeg 之前拒绝。

重新生成画面和音频（在仓库根目录，需可运行的 ffmpeg）：

```sh
ffmpeg -hide_banner -nostdin -y -f lavfi -i "color=c=0xCC3322:s=320x180:r=30:d=4" -frames:v 120 -c:v libx264 -pix_fmt yuv420p -preset veryfast -crf 28 -an tests/fixtures/media/clip_red.mp4
ffmpeg -hide_banner -nostdin -y -f lavfi -i "color=c=0x2266CC:s=320x180:r=30:d=4" -frames:v 120 -c:v libx264 -pix_fmt yuv420p -preset veryfast -crf 28 -an tests/fixtures/media/clip_blue.mp4
ffmpeg -hide_banner -nostdin -y -f lavfi -i "sine=frequency=440:sample_rate=48000:duration=3.5" -t 3.5 -c:a pcm_s16le tests/fixtures/media/voice_demo.wav
ffmpeg -hide_banner -nostdin -y -f lavfi -i "sine=frequency=196:sample_rate=48000:duration=2" -t 2 -c:a pcm_s16le tests/fixtures/media/bgm_demo.wav
```
