# Proposal

## Why

同一台本机部署上切换浏览器时，账号昵称需要保持一致。浏览器各自的站点存储无法共享。

## What Changes

- 使用本机 MySQL 保存本人昵称，顶栏和个人信息从服务端身份读取。
- 旧浏览器昵称可在账号未设置时作为弹窗草稿，用户确认保存后生效。

## Capabilities

### New Capabilities

- `account-display-name`：账号昵称持久化与工作台展示。

### Modified Capabilities

## Impact

用户表增量迁移、会话身份读取、本人昵称写入及前端回归。
