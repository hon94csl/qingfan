# 青帆本机试用包

这个目录由当前机器上的 `make trial` 生成。它只供这台机器试用，没有签名，也没有公证。

## 里面有什么

- `bin/ffmpeg` 和 `bin/ffprobe`：从当时 PATH 上的可执行文件复制，版本写在 `ffmpeg-version.txt` 和 `ffprobe-version.txt`。
- `fonts/Lato-Regular.ttf`：样片字幕用的字体，许可见同目录的 `OFL.txt`。
- 没有桌面安装器，没有 Windows 包，没有自动更新，也没有远程崩溃上报。

把这份目录交给别人之前，先确认本机 FFmpeg 构建的分发许可。这里只是把本机工具复制出来，不是一次独立的发行审核。

## 怎么用

在终端里把 `bin` 放到 PATH 前面，再运行原来的命令：

```sh
export PATH="/绝对路径/build/trial/bin:$PATH"
ffmpeg -version
ffprobe -version
```

渲染仍使用仓库里的 `qingfan-render`。这个试用包不包含 Go 工具链，也不代替 `make dev`。

## 日志

错误留在启动该进程的终端里。配音步骤失败时记录步骤和素材标识，不写供应商密钥，也不把整段口播写进步骤错误。没有单独的日志上传。

## 还没有验证的事

没有在一台不带开发工具的干净机器上安装。没有测量内存占用和界面响应。默认成片仍使用 libx264；只有设置 `QINGFAN_ENCODER=auto` 时才探测 VideoToolbox，探测失败就回到 libx264。不同硬件编码器不保证字节相同。
