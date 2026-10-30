# ensis-ops

Desktop operations console for Windows (primary), with build scripts for Linux, Kylin, and macOS. It connects to remote machines and databases: SSH, SFTP, FTP, RDP, and SQL/NoSQL/API clients. The window is a C++ CEF shell, the UI is Vue 3, and each capability runs as its own process. Day-to-day development docs below are in Chinese.

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](./LICENSE)

桌面运维控制台。在一个窗口里管理远程主机和数据库：SSH 终端、SFTP / FTP 文件、RDP 远程桌面，以及 MySQL、PostgreSQL、Oracle、SQL Server、SQLite、Redis、MongoDB、ClickHouse、达梦、人大金仓、Vastbase 和 API 调试。官网 [niuma007.com](https://www.niuma007.com)。版本以根目录 `package.json` 为准，变更见 [CHANGELOG.md](./CHANGELOG.md)。

壳层进程是 `ensis-ops.exe`，用户数据在 `%LOCALAPPDATA%\ensis-ops`。界面必须跑在 CEF 里（`cefQuery`）。`pnpm dev:web` 只调 Vue，不是桌面应用。

## 快速开始

当前桌面开发目标是 **Windows x64**。Linux / 麒麟 / macOS 在对应系统上构建，脚本不支持交叉打包，见 [scripts/README.md](./scripts/README.md)。

```powershell
pnpm install
pnpm setup:desktop    # 首次：CMake、MSVC、Rust、CEF
pnpm dev              # 编译并打开 ensis-ops.exe
```

第一次 `pnpm dev` 会编译全部能力服务。其中 RDP 若还没有本地 OpenSSL 静态库，会多花几十分钟，只此一次。见下方常见问题。

成功时出现 CEF 窗口，加载 `app://ensis-ops/`，状态栏显示 **CEF Shell**。

## 常用命令

| 场景 | 命令 |
|------|------|
| 桌面应用 | `pnpm dev` |
| 改 Vue 时热更新 | `pnpm dev:hot` |
| 只编后端服务 | `pnpm build:services` |
| 只编壳 | `pnpm build:shell` |
| 绿色目录 | `pnpm pack:win` → `output/windows-x64/dir/` |
| 安装程序 | `pnpm pack:win:setup` |
| 一键发布（构建 + 绿色目录 + Setup） | `pnpm release:win` |
| 组件库 Playground | `pnpm dev:ui`（端口 5180） |
| Web 单元测试 | `pnpm test:web` |
| UI 单元测试 | `pnpm test:ui` |

其它平台的 `build:*` / `pack:*` / `release:*`，以及 GitHub Actions「Pack and Release」，见 [scripts/README.md](./scripts/README.md)。推送 `main` 且 changelog 有未发布条目时会自动发版，也可以推 `v*` tag 或手动运行。

## 环境

| 工具 | 版本 | 谁用 |
|------|------|------|
| Node.js | ≥ 24（`package.json` 的 `engines`） | 前端与 pnpm 脚本。[nodejs.org](https://nodejs.org/) |
| pnpm | 9.15（`packageManager`） | `corepack enable` 后 `corepack prepare pnpm@9.15.0 --activate` |
| PowerShell | 5.1+ | Windows 脚本入口 |
| Visual Studio 2022 | 带 **C++ 桌面开发** 或 **Build Tools + VC 工具链** | 壳、Oracle、RDP。[Build Tools](https://visualstudio.microsoft.com/visual-cpp-build-tools/) |
| CMake | ≥ 3.20 | 壳与 C++ 服务。[cmake.org](https://cmake.org/download/) |
| Go | 1.25.x | `platform/` 与各 Go 服务。[go.dev/dl](https://go.dev/dl/) |
| Rust | stable（根目录 `rust-toolchain.toml`） | SSH、SFTP、Redis。[rustup.rs](https://rustup.rs/) |
| Git for Windows | 默认目录 `C:\Program Files\Git` | 克隆仓库；其中的 `perl.exe` 只在第一次为 RDP 编译 OpenSSL 时用到 |
| winget | 可选 | `pnpm setup:desktop` 用来装 CMake、MSVC、Rust |

前端依赖版本在 `pnpm-workspace.yaml` 的 catalog（Vue 3.5、Vite 8 等）。CEF 标准包由脚本下载到 `third_party/cef/`，约 200MB。

OpenSSL 不用单独安装。RDP 第一次构建会把 **OpenSSL 3.5.4** 静态编到 `services/rdp-service/third_party/openssl`（不进 git）。Oracle、RDP 缺少工具链时可以跳过，见常见问题。

## 第一次准备

### 1. Node 与依赖

```powershell
node -v    # >= 24
corepack enable
corepack prepare pnpm@9.15.0 --activate
pnpm install
```

### 2. 桌面工具链

```powershell
pnpm setup:desktop
```

脚本会检查并在缺失时安装：CMake、VS 2022 Build Tools（C++）、Rust stable，然后下载 CEF 到 `third_party/cef/`。结束后若提示找不到 Git perl，安装 [Git for Windows](https://git-scm.com/download/win) 到默认目录。

只重下 CEF：删掉 `third_party/cef/` 再执行 `pnpm cef:download`。

已准备好环境时，`pnpm dev -- -SkipSetup` 可跳过这次检查。

### 3. 确认 Go 与 Rust

`setup:desktop` 会装 Rust，不会装 Go。

```powershell
go version       # 1.25.x
cargo --version
```

### 4. 启动

```powershell
pnpm dev
```

热更新（Vite 5173，CEF 直接加载开发服，改 `web/` 保存即刷新）：

```powershell
pnpm dev:hot
```

## 改哪一块

每项能力是独立进程，清单在 `services/manifests/`。改行为前先读 [docs/README.md](docs/README.md) 里对应模块文档。

| 要改的东西 | 目录 |
|------------|------|
| 界面、连接表单、会话 | `web/` |
| 窗口与 CEF，不含业务 | `shell/` |
| 凭据、SQLite、拉起服务 | `platform/` |
| 某一项远程或数据库能力 | `services/<name>-service/` |
| 公共库 | `packages/cpp`、`packages/go`、`packages/rust`、`packages/ui` |
| 构建与打包入口 | `scripts/entry/`（实现在 `scripts/shared/`、`scripts/platforms/`） |

`pnpm build:services` 默认编入 Go 服务、Rust 的 SSH / SFTP / Redis，以及 C++ 的 Oracle、RDP。本机 Windows 产物在 `services/bin/windows-x64/`，并同步一份到 `services/bin/*.exe`。

暂时不编 C++ 服务（界面入口还在，对应 exe 缺失就连不上）：

```powershell
powershell -ExecutionPolicy Bypass -File scripts/shared/build/build-services.ps1 -SkipOracle -SkipRdp
```

## 本地路径

| 内容 | 路径 |
|------|------|
| 壳 | `build/shell-windows-x64/Release/ensis-ops.exe` |
| 壳加载的 Web | `build/shell-windows-x64/Release/resources/web/` |
| 服务二进制 | `services/bin/windows-x64/`（兼容目录 `services/bin/*.exe`） |
| CEF | `third_party/cef/` |
| RDP 的 OpenSSL（本机生成） | `services/rdp-service/third_party/openssl/` |
| 用户数据库 | `%LOCALAPPDATA%\ensis-ops\data\ensis-ops.db` |
| 绿色包 / 安装包 | `output/windows-x64/dir/` 、 `output/windows-x64/setup/` |

## 常见问题

**`cmake not found`**  
安装 CMake 并保证在 PATH 中，或新开终端后执行 `pnpm setup:desktop`。

**`MSVC not found`**  
用 Visual Studio Installer 加上 **「使用 C++ 的桌面开发」** 或 **VC++ x64/x86 生成工具**。脚本通过 `vswhere` 找带 `VC.Tools.x86.x64` 的 VS 2022。

**CEF 版本不对**  
删除 `third_party/cef/`，执行 `pnpm cef:download`。

**窗口空白或秒退**  
看 `build/shell/Release/debug.log`，并确认 `resources/web/index.html` 存在（先 `pnpm dev` 或 `pnpm build:web`）。

**`cefQuery unavailable`**  
页面开在了普通浏览器里。用 `pnpm dev` 启动桌面窗口。

**第一次停在 `Building OpenSSL`，或 RDP 编译很久**  
正常。只在 `services/rdp-service/third_party/openssl/lib/libcrypto.lib` 还不存在时编译 OpenSSL 3.5.4。编完会跳过。不要删这个目录，除非打算重编。

**`perl not found`，路径是 `C:\Program Files\Git\usr\bin\perl.exe`**  
Git for Windows 要装在默认目录。装在其它盘符时脚本认不到。可以先 `-SkipRdp`。

**`cargo not found`**  
安装 Rustup，并让 `%USERPROFILE%\.cargo\bin` 进入 PATH。`pnpm setup:desktop` 会装 stable。

**`Missing manifest in toolchain 'stable-x86_64-pc-windows-msvc'`**  
本机 Rust toolchain 不完整。依次执行：

```powershell
rustup self uninstall -y
Remove-Item "$env:USERPROFILE\.rustup" -Recurse -Force -ErrorAction SilentlyContinue
Remove-Item "$env:USERPROFILE\.cargo" -Recurse -Force -ErrorAction SilentlyContinue
winget install Rustlang.Rustup --accept-package-agreements --accept-source-agreements
$env:Path = "$env:USERPROFILE\.cargo\bin;$env:Path"
rustup toolchain install stable
rustup default stable
cargo --version
```

`winget install` 若一直无输出，先结束残留的 `winget` / `rustup-init` 再重试。

## 参与

1. 先看 [docs/README.md](docs/README.md) 里和你要改的模块对应的那篇，再改代码。架构图在 [docs/architecture.md](docs/architecture.md)。
2. 用户能感知的变化写进 [CHANGELOG.md](./CHANGELOG.md) 的 `[Unreleased]`，只记功能。
3. 用本仓库的 Issue / Pull Request。构建必须在目标操作系统上执行。
4. 第三方组件许可证见 [docs/compliance/NOTICES.txt](./docs/compliance/NOTICES.txt)。

## 仓库结构

```
ensis-ops/
├── docs/                 # 设计文档索引：docs/README.md
├── scripts/
│   ├── entry/            # pnpm 调用的入口
│   ├── shared/           # 服务构建、打包拼装
│   ├── platforms/        # Windows / Linux / 麒麟 / macOS
│   └── sql/sqlite/       # SQLite 迁移
├── shell/                # C++ CEF 壳
├── web/                  # Vue 3
├── platform/             # Platform Core（Go）
├── services/             # 每项能力一个进程 + manifests/
├── packages/             # cpp / go / rust / ui
├── third_party/cef/      # setup 后生成
├── build/                # 壳的构建目录
└── output/               # 打包结果
```

## 架构

```
Vue 3  ── CEF IPC ──>  C++ Shell  ── Named Pipe / UDS + JSON ──>  Platform Core ──> 各能力进程
```

壳不定业务 TCP 端口。进程间契约见 [docs/03-ipc-protocol.md](docs/03-ipc-protocol.md)。

## 许可证

本仓库源码以及发行包中由 ensis-ops 编写的部分，按 [Apache License 2.0](./LICENSE) 授权。版权与署名见 [NOTICE](./NOTICE)。

第三方组件（含捆绑的 Chromium Embedded Framework、FreeRDP 等）按其自身许可证分发，清单见 [docs/compliance/NOTICES.txt](./docs/compliance/NOTICES.txt)。名称与图标为商标，Apache 2.0 **不授予**商标权。

官方免责见 [docs/legal/DISCLAIMER.zh-CN.txt](./docs/legal/DISCLAIMER.zh-CN.txt)：在法律允许范围内，开发者对使用后果不承担责任；免费获得时责任上限为零。安装向导附加声明不得缩小该范围，也不削减 Apache 2.0 已授予的复制、修改与再分发权利。
