---
title: 状态
---

[首页](index.md) · [功能](features.md)

| 项目 | 当前情况 |
| --- | --- |
| 版本 | 3.0.0 |
| 发布 | [v3.0.0](https://github.com/leiz11904/CClear/releases/tag/v3.0.0) |
| 安装包 | `CClear-Setup-3.0.0.exe` |
| 安装包 SHA-256 | `67e10f4b294e8501c89dc47eebb6337b31db2113229394c07c6938c5644c2d4b` |
| 代码签名 | 未签名。没有证书时不会签名，也不会伪造签名 |
| 构建检查 | [CI](https://github.com/leiz11904/CClear/actions/workflows/ci.yml) |

![CI](https://github.com/leiz11904/CClear/actions/workflows/ci.yml/badge.svg)

安装包不放进 git。`dist/` 被忽略。对外使用上面的 Release；Actions 会构建并上传产物，但不会自动创建 Release。

Windows 第一次打开未签名安装包时，SmartScreen 可能提示「Windows 已保护你的电脑」。可以选择「更多信息」，再选择「仍要运行」。

## 模块

| 模块 | 状态 |
| --- | --- |
| 总览与智能扫描 | 可用。只读，不自动清理 |
| 垃圾清理 | 可用。沿用原有安全白名单，需确认后执行 |
| 磁盘分析 | 可用。只读 |
| 重复文件 | 可用。只移入回收站，一组至少保留一份 |
| 软件管理 | 可用。只启动已登记的卸载程序 |
| 系统占用 | 可用。只读，不删除系统文件 |
| 清理历史 | 可用 |

## 尚未完成

- 软件图标还没有从可执行文件提取，列表使用首字占位。
- 卸载程序启动后不会自动等待它退出，需要手动刷新软件列表。
- 智能扫描里的重复文件阶段会遍历所选系统盘，磁盘较大时会比较慢。
- 普通 `go test ./...` 因构建标签不会编译 `cmd/cclear-modern`。该包使用 `go test -tags dev`。
