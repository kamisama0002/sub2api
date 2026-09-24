# 本地运行

访问 http://localhost:8080 。管理员邮箱为 `admin@sub2api.local`，密码位于 `deploy/.env` 的 `ADMIN_PASSWORD`。

在此目录执行：

```sh
cd deploy
# 启动
docker compose -f docker-compose.local.yml -f docker-compose.override.yml up -d
# 状态
docker compose -f docker-compose.local.yml -f docker-compose.override.yml ps
# 日志
docker compose -f docker-compose.local.yml -f docker-compose.override.yml logs -f sub2api
# 停止（保留数据）
docker compose -f docker-compose.local.yml -f docker-compose.override.yml down
# 源码更新后重新构建
docker compose -f docker-compose.local.yml -f docker-compose.override.yml up -d --build
```

使用本地源码构建镜像。服务仅绑定 127.0.0.1:8080。数据保存在 deploy/data、deploy/postgres_data、deploy/redis_data；配置与密码保存在 deploy/.env。
