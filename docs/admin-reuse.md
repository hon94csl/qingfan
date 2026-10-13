# 管理后台复用说明

来源：同一工作区的 `tk-flywheel`，提交 `2357d922a15cb8a4543519c7e993be0cb7b92d2d`。

- `frontend/src/main.tsx`：沿用 Ant Design Layout、侧栏、页头、卡片和青色主题，提取到青帆 `admin/src/main.tsx`。
- `frontend/src/style.css`：复用布局基础样式，移除业务页面样式。
- `frontend/src/api.ts`：沿用同源请求和 HTTP 错误处理方式，适配为健康检查，并验证返回的服务身份。
- 沿用 React 19.3.0、Ant Design 6.6.4、TypeScript 7.0.2、Vite 8.3.0，分别提交桌面和后台锁文件。

当前后台只有服务状态页。旧项目的商家、店铺、投放、计费、数据库和凭据不迁入青帆。登录页面必须与青帆服务端认证、授权、会话及 CSRF 防护一起实施；不能只复制前端路由守卫。

用量库在服务端 SQLite，由 `cmd/qingfan-server` 持有。管理后台仍然只检查健康状态，页面不保存供应商密钥，也不提供令牌表单。登录页面必须与服务端认证、会话及 CSRF 防护一起实施。当前服务默认监听本机；对外监听需要 HTTPS 证书。
