# Sub2API 升级指南

## 快速升级（推荐）

使用提供的升级脚本，安全地升级 sub2api 而不影响数据库：

```bash
cd /home/ubuntu/sub2api/deploy
./upgrade.sh
```

脚本会自动：
- 拉取最新的 custom 镜像
- 只重启 sub2api 容器（不动 postgres 和 redis）
- 等待健康检查
- 显示升级前后的版本信息

## 手动升级

如果你想手动升级，请严格按照以下步骤操作：

```bash
cd /home/ubuntu/sub2api/deploy

# 1. 拉取新镜像
docker compose pull sub2api

# 2. 升级 sub2api 容器（关键：必须加 --no-deps）
docker compose up -d sub2api --no-deps

# 3. 查看日志确认启动成功
docker compose logs -f sub2api
```

**⚠️ 重要：必须使用 `--no-deps` 参数**

`--no-deps` 告诉 Docker Compose **只更新 sub2api 容器，不要重建 postgres 和 redis**。

如果不加这个参数，postgres 容器会被重建，虽然数据卷挂载正确，但可能触发数据库初始化逻辑。

## 错误示范

❌ **不要这样做：**

```bash
# 错误：会重建所有容器，包括数据库
docker compose up -d

# 错误：会重建 sub2api 的依赖服务
docker compose up -d sub2api
```

## 升级到特定版本

如果你想升级到特定的 commit 版本：

```bash
# 使用脚本
./upgrade.sh custom-abc1234

# 或手动修改 docker-compose.yml
# 将 image: ghcr.io/lusya123/sub2api:custom
# 改为 image: ghcr.io/lusya123/sub2api:custom-abc1234
# 然后执行升级
```

## 回滚

如果升级后出现问题，可以回滚到之前的版本：

```bash
# 1. 修改 docker-compose.yml，指定旧版本的 tag
# 2. 执行升级脚本
./upgrade.sh custom-old-commit-hash
```

## 数据备份

虽然正常升级不会影响数据，但建议定期备份：

```bash
# 备份数据库
docker exec sub2api-postgres pg_dump -U sub2api sub2api > backup_$(date +%Y%m%d).sql

# 备份 Redis
docker exec sub2api-redis redis-cli save
docker cp sub2api-redis:/data/dump.rdb backup_redis_$(date +%Y%m%d).rdb

# 备份应用数据
docker run --rm -v deploy_sub2api_data:/data -v $(pwd):/backup alpine \
  tar czf /backup/sub2api_data_$(date +%Y%m%d).tar.gz -C /data .
```

## 故障排查

### 服务启动失败

```bash
# 查看详细日志
docker compose logs --tail=100 sub2api

# 检查容器状态
docker compose ps -a

# 检查健康状态
docker inspect sub2api --format='{{.State.Health.Status}}'
```

### 数据库连接失败

```bash
# 检查 postgres 是否运行
docker compose ps postgres

# 测试数据库连接
docker exec sub2api-postgres psql -U sub2api -d sub2api -c 'SELECT 1;'
```

### 配置问题

```bash
# 查看当前配置
docker exec sub2api cat /app/data/config.yaml

# 检查环境变量
docker exec sub2api env | grep -E 'DATABASE|REDIS|SERVER'
```

## CI/CD 自动构建

每次你 push 代码到 `custom` 分支，GitHub Actions 会自动：
1. 构建新的 Docker 镜像
2. 推送到 `ghcr.io/lusya123/sub2api:custom`
3. 同时创建带 commit hash 的 tag（用于回滚）

你只需要在服务器上执行 `./upgrade.sh` 即可拉取最新版本。

## 自动同步官方更新

每天北京时间早上 9 点，GitHub Actions 会自动：
1. 同步官方 upstream/main 到你的 main 分支
2. 尝试 rebase custom 分支到最新的 main
3. 如果有冲突，会跳过并通知你手动处理

你可以在 GitHub Actions 页面查看同步状态。
