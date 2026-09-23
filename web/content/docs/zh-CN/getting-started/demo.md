---
title: 演示模式
order: 5
category: 快速开始
description: 使用预设数据展示功能并保持数据只读
---

# 演示模式

通过 `--demo` 启动，首次启动时生成独立的演示数据库，随后以只读方式展示。登录表单自动填写 `admin` / `12345678`
。预设覆盖账号、会话、团队、Maven/Cargo/npm/Docker 软件包、文件元数据、审核、工单、配额、通知、统计、审计日志和发件记录。

```bash
./renop --demo
./renop --demo --demo-temp
```

使用 `--demo --demo-temp` 可保存系统设置和存储库定义。修改会保留。重新启动时只传 `--demo` 即恢复只读展示。单独使用
`--demo-temp` 会被拒绝。两种模式都禁止其他账号或内容修改，也禁止写入日志。

默认使用 `renop-demo.db` 和 `renop-demo-settings.db`，分别可由 `RENOP_DEMO_DATABASE` 和 `RENOP_DEMO_SETTINGS_DB`
指定路径，与正常实例数据独立。已有演示数据会保留，包括空的存储库集合。只创建数据库，不生成软件包、索引或日志文件，因此不提供软件包下载和文档预览。登录会话保存在有界进程内存中，预设会话记录不能用于认证。

`GET /api/demo` 返回二进制 protobuf `DemoInfo`，包含 `enabled`、`temporary` 和公开演示凭据。正常模式不返回凭据。禁止的修改返回
`403` 和 `demo_read_only`，不可用的文件返回 `demo_file_unavailable`。不启动邮件、镜像、维护和更新后台任务。
