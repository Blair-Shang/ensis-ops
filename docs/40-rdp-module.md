# 40 — RDP 远程桌面

独立能力服务 `rdp-service`（C++，链 FreeRDP 3.32）加 Web 画布。控制面走现有 JSON Bridge，画面不进 JSON。

## 范围

已接入：

- 连接站点 `connection_kind = rdp`，默认端口 3389
- 安全层 `auto` / `nla` / `tls` / `rdp`
- 域，或写在账号里的 `DOMAIN\user`
- 证书指纹：首次连接确认后写入 `accepted_fingerprint`
- 桌面画布、键鼠（浏览器 `KeyboardEvent.code` 转扫描码）
- 软件画面：经典位图（NSCodec / RemoteFX）。不走 Graphics Pipeline，避免没有 H.264 时被服务端拆掉
- 文件：每个会话把本机一个目录映射成远端磁盘 Ensis。上传把选中的文件复制进该目录；在远端把文件复制到 Ensis，即下载到本机

未接入：剪贴板、音频、RD Gateway、代理、SSH 隧道、动态分辨率。

## 画面通道

`session.open` 之后，页面再调 `rdp.session.info`，用返回的 `frameHost` / `framePort` / `frameToken` 连接 `ws://127.0.0.1:<port>/?token=...`。

二进制帧（小端）：

| type | 内容 |
|------|------|
| 1 | `x,y,w,h` 各 uint16，随后 RGBA |
| 2 | `width,height` 各 uint16 |

令牌只在本机回环上使用。关 Tab 由 Session Registry 调 `session.close`。

## 构建

需要 CMake、Visual Studio（与桌面壳相同，`pnpm setup:desktop` 会装），以及装在默认目录的 Git for Windows（`C:\Program Files\Git\usr\bin\perl.exe`）。不要单独装 OpenSSL。`build-services.ps1` 和 `pnpm dev:hot` 默认会编它，并经 `stage-services` 拷到壳层 `services/bin/`。本机还没有静态库时，脚本会下载并静态编译 OpenSSL 3.5.4 到 `services/rdp-service/third_party/openssl`（不进 git），每台机器只这一次。没有这套工具链时：

```powershell
.\scripts\shared\build\build-services.ps1 -SkipRdp
```

单独编译：

```powershell
.\scripts\shared\build\build-rdp-service.ps1
```

产物在本机架构下会放到 `services/bin/ensis-rdp-service.exe`，同时复制到 `services/bin/windows-x64/`。若 OpenSSL 是动态库，把 `libcrypto` / `libssl` 放到同一目录。安装包需带上 Apache 2.0 许可证文本和 FreeRDP 版权声明。
