# ensis-ui 已迁出

公共 UI 组件库已独立为仓库：

- 本地：`E:\shangijan\ensis-ui`（与本仓库同级）
- 远程：https://github.com/Blair-Shang/ensis-ui（Private）
- 版本 tag：`v1.1.11`

桌面端通过 `web/package.json` 依赖接入（本地联调）：

```json
"ensis-ui": "link:../../ensis-ui"
```

日常 `pnpm dev` 继续用同级 `link`。GitHub 打 ensis-ops 安装包时 checkout `ensis-ui` 的 **git tag**（当前默认 `v1.1.11`），不要用 ensis-ops 的 `version:sync` 去改 ui 仓版本号。

本地联调请直接修改 `../ensis-ui` 源码。Playground：

```bash
pnpm --dir ../ensis-ui dev
```

本目录仅保留迁移说明，不再作为 pnpm workspace 包。
