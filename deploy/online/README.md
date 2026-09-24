# 阿里云独立部署

应用发布端口为 **6666**，容器内监听 8080。PostgreSQL 和 Redis 仅在独立 Compose 网络内访问，不发布数据库端口。

这套配置用于全新安装，不复制本地数据库、Adobe Cookie 或 API 密钥。线上账号和密钥需重新创建。保留上游源码、许可证及提交历史。

## 初始化与启动

在项目根目录运行：

```sh
python3 deploy/online/init_env.py
docker compose -f deploy/online/compose.yml --env-file deploy/online/.env up -d --build
docker compose -f deploy/online/compose.yml --env-file deploy/online/.env ps
curl --fail http://127.0.0.1:6666/health
```

也可以在构建机上生成服务器架构的镜像，使用 `docker save` / `docker load` 传输，在服务器以 `up -d --no-build` 启动，避免占用小内存服务器的编译资源。

初始化脚本只在 `.env` 不存在时生成独立随机密码和固定加密密钥；文件权限为 0600，重复运行不会重置凭据。管理员邮箱默认为 `admin@sub2api.local`，初始密码见服务器的 `deploy/online/.env` 中 `ADMIN_PASSWORD`。

`.env`、运行数据、日志和镜像归档均不应提交到 GitHub。新仓库保留了上游工作流文件，但默认关闭 GitHub Actions；本次部署通过 SSH 完成，不配置 GitHub 持有服务器私钥的自动部署。

## 浏览器访问

Chromium 将 6666 列为受限端口，直接访问可能出现 `ERR_UNSAFE_PORT`，这并不代表服务器未运行。参见 [Chromium 端口列表](https://github.com/chromium/chromium/blob/main/net/base/port_util.cc)。

- 命令行 / 原生 API 客户端可使用 `http://服务器IP:6666/v1`。
- `nginx.conf` 提供独立的 **6660** 网页入口，代理同一个服务，可使用 `http://服务器IP:6660`。
- 如有域名和证书，配置 HTTPS 443 反向代理到 `127.0.0.1:6666`，统一通过 HTTPS 使用网页和 API。
- 阿里云安全组与主机防火墙须允许实际对外使用的端口。安装额外 Nginx 配置前先确认 6660 未被其他服务使用；执行 `nginx -t` 成功后再 reload。

## 维护

### 短账号登录

登录页支持省略内部邮箱的 `@sub2api.local` 后缀，例如 `pink@sub2api.local` 可直接输入 `pink`。完整邮箱仍可登录。登录页自动补全内部邮箱后使用原有认证接口，不修改数据库邮箱、密码、权限或双重验证；其他邮箱域名的账号仍需输入完整邮箱。直接调用 `/api/v1/auth/login` 的客户端仍应传完整邮箱。

```sh
# 日志
docker compose -f deploy/online/compose.yml --env-file deploy/online/.env logs --tail=100 app
# 停止，保留目录中的数据
docker compose -f deploy/online/compose.yml --env-file deploy/online/.env down
# 已加载新镜像后启动；使用固定版本镜像时需同步修改 .env 中 SUB2API_IMAGE
docker compose -f deploy/online/compose.yml --env-file deploy/online/.env up -d --no-build
```

持久数据位于此目录的 `data/`、`postgres_data/`、`redis_data/`。备份时应包含 `.env` 和应用配置；数据库应使用 `pg_dump`，或停服后复制完整数据目录。恢复已有安装时保留原 JWT/TOTP 密钥和数据库密码。

默认应用内存上限 384MiB、PostgreSQL 192MiB、Redis 96MiB，并降低连接池大小，适用于小规模试用；扩大并发前应根据实际负载调整资源配置。

## 管理员查看全站 API 密钥

- 后台新增「全站 API 密钥」，可按密钥名称、用户名或邮箱搜索，按状态筛选，并分页查看。原有「用户管理 → API 密钥」也提供查看和复制按钮。
- 系统设置中开启 TOTP 功能（部署必须设置固定的 `TOTP_ENCRYPTION_KEY`），各管理员在「个人资料 → 两步验证」用自己的身份验证器绑定。此开关仅开放绑定功能，不会替用户绑定或改变登录密码。
- 完整密钥只能经 `POST /api/v1/admin/api-keys/:id/reveal` 按需获取，传入 `purpose: view` 或 `copy`。此接口始终要求真人管理员的 TOTP 会话验证，不受可选 `step_up_enabled` 开关影响。现有验证授权有效期为 15 分钟；管理 API Key 不可调用。
- 每次返回完整密钥前同步写入 `admin.api_keys.view` 或 `admin.api_keys.copy` 审计记录，包含操作者、目标密钥 ID、所属用户 ID、时间和来源 IP。复制事件表示服务器批准获取密钥用于复制，不代表浏览器剪贴板写入一定成功。日志不包含密钥正文。审计存储不可用时拒绝披露。
- 全站、用户、分组、使用记录等常规 DTO 返回脱敏密钥；普通用户自己的密钥管理接口仍需所有权校验。已有数据库备份、服务器访问等超管能力不在此功能的权限边界内。
- 查看后的明文仅保留在当前组件内，30 秒后、离开页面或切换标签时隐藏；每次点击复制重新请求审计接口。已复制到用户剪贴板的内容不会被自动删除。
