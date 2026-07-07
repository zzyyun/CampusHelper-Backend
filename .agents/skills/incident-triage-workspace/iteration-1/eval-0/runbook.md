# Runbook — Incident Report

## 基本信息

| 字段 | 值 |
|------|-----|
| 标题 | incident-2026-06-27-login-502 |
| 严重度 | P1 |
| 发现时间 | 2026-06-27T12:30:00+08:00 |
| 服务/模块 | user-service / gateway |
| 报告人 | on-call engineer |

## 症状描述

用户登录接口返回 HTTP 502，Gateway 无法连接 user-service gRPC 端口。

## 版本信息

| 字段 | 值 |
|------|-----|
| 当前版本 | 5bd49d3 (feat: AI 智能审核) |
| 上一正常版本 | 8e59b81 (merge 前) |
| 最近部署 | 2026-06-27 ~12:00 |

## 时间线

| 时间 | 事件 |
|------|------|
| 12:20 | user-service 异常退出，端口 50051 未释放 |
| 12:30 | 用户反馈登录失败，on-call 接到告警 |
| 12:32 | 确认问题为端口冲突 |
| 12:35 | 执行 taskkill 释放端口 + 重启 user-service |
| 12:36 | 登录恢复，验证通过 |

## 根因

- **根因文件**: cmd/user/main.go:61
- **根因描述**: net.Listen 前没有等待旧进程释放端口
- **触发条件**: user-service 异常退出后立即重启

## 后续动作

- [P1] 添加端口健康检查监控
- [P1] 启动脚本增加 taskkill 清理
- [P2] 改用 supervisord 管理服务进程
