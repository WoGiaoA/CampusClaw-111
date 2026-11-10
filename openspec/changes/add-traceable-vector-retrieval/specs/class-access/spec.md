# Token Authentication Delta

## ADDED Requirements

### Requirement: 可撤销的 Bearer Token
系统 MUST 在登录成功的现有身份字段旁返回 access_token、token_type=Bearer、expires_in=28800。Token MUST 为密码学随机不透明值，数据库只保存 HMAC 摘要，复用八小时 Session。系统 MUST 优先校验 Authorization；头存在但格式非法、过期或撤销时返回401，不回落 Cookie。无头时 MUST 兼容旧 Cookie。权限每次由 users 回表取得，保留 CSRF 与首次改密。浏览器 MUST 在后续受保护请求（包括下载）携带 Bearer，只保存在 sessionStorage；401、退出或改密成功清除。Token MUST 不出现在 URL、日志、来源或 me 响应中。

#### Scenario: 浏览器登录与请求
- **WHEN** 用户登录后刷新材料、检索、问答、下载或修改密码
- **THEN** 请求头含 Authorization: Bearer，写操作仍带有效 CSRF，Network 可检查

#### Scenario: 不可信头与撤销
- **WHEN** 有效 Cookie 同时携带非法 Bearer，或退出/改密后重放旧 Token
- **THEN** 返回401，不允许凭 Cookie 绕过失效 Token
