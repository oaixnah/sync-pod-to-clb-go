# 同步Pod到负载均衡后端 - Golang版本

这是原Python项目的Golang重写版本，提供了更好的性能、更小的内存占用和更快的启动时间。

## 项目概述

本项目监控Kubernetes集群中的Pod变化，并自动将Pod IP同步到腾讯云负载均衡器(CLB)的后端服务列表中。当Pod创建、删除或更新时，会自动更新对应的负载均衡器后端配置。

## 主要改进

与Python版本相比，Golang版本具有以下优势：

- **性能提升**: 更快的执行速度和更低的资源消耗
- **内存效率**: 显著降低内存使用量
- **启动速度**: 更快的应用启动时间
- **并发处理**: 更好的并发性能
- **部署简化**: 单一二进制文件，无需Python运行时
- **安全性**: 更严格的安全配置和非root用户运行

## 项目结构

```
.
├── main.go              # 主程序入口
├── tencent.go           # 腾讯云API客户端
├── config.go            # 配置管理
├── go.mod               # Go模块定义
├── Dockerfile           # Go版本的Dockerfile
├── deployment.yaml      # Go版本的K8s部署文件
├── Makefile             # 构建和部署脚本
├── rules.yaml           # 负载均衡规则配置
├── kube-config          # Kubernetes配置文件
└── README.md            # 本文档
```

## 环境要求

- Go 1.21+
- Docker
- Kubernetes集群访问权限
- 腾讯云CLB访问权限

## 快速开始

### 1. 环境变量配置

设置腾讯云访问密钥：

```bash
export CLOUD_TENCENT_SECRET_ID="your-secret-id"
export CLOUD_TENCENT_SECRET_KEY="your-secret-key"
export TENCENT_REGION="ap-beijing"  # 可选，默认为ap-beijing
```

### 2. 本地开发

```bash
# 安装依赖
go mod download

# 运行应用
go run .

# 或使用Makefile
make run
```

### 3. 构建和部署

```bash
# 构建二进制文件
make build

# 构建Docker镜像
make docker-build

# 推送到镜像仓库
make docker-push

# 部署到Kubernetes
make deploy

# 一键构建和部署
make all
```

## 配置说明

### rules.yaml

配置文件格式与Python版本保持一致：

```yaml
- load_balancer_id: lb-xxxxxxxx
  listeners:
    - port: 80
      protocol: http
      rules:
        - domain: example.com
          url: /api
          backend:
            namespace: default
            deployment: my-app
            port: 8080
```

### Kubernetes RBAC

Go版本包含了完整的RBAC配置，确保应用具有必要的权限：

- ServiceAccount: `pod-to-clb-controller`
- ClusterRole: 读取pods、deployments、replicasets
- ClusterRoleBinding: 绑定角色到服务账户

## 监控和日志

### 日志格式

使用结构化日志，包含时间戳和日志级别：

```
2024-01-15 10:30:45 INFO Start watching pod events...
2024-01-15 10:30:46 INFO default my-app ADDED pod-123 lb-xxx Adding new backend: [10.0.1.100]
```

### 健康检查

可以通过取消注释deployment-go.yaml中的健康检查配置来启用：

```yaml
livenessProbe:
  httpGet:
    path: /health
    port: 8080
  initialDelaySeconds: 30
  periodSeconds: 10
```

## 安全特性

- 非root用户运行 (UID: 1001)
- 只读根文件系统
- 禁用特权升级
- 移除所有Linux capabilities
- 使用Kubernetes Secrets管理敏感信息

## 性能优化

- 配置缓存机制（60秒过期）
- 高效的字符串操作
- 并发安全的配置管理
- 优化的内存使用

## 故障排除

### 常见问题

1. **权限错误**
   ```bash
   kubectl logs deployment/pod-to-clb-controller-go
   ```
   检查RBAC配置是否正确

2. **腾讯云API错误**
   - 验证Secret ID和Key是否正确
   - 检查区域设置
   - 确认CLB权限

3. **配置加载失败**
   - 检查rules.yaml格式
   - 验证负载均衡器ID是否存在

### 调试模式

设置日志级别为DEBUG：

```go
log.SetLevel(log.DebugLevel)
```

## 迁移指南

### 从Python版本迁移

1. **停止Python版本**
   ```bash
   kubectl delete deployment pod-to-clb-controller
   ```

2. **部署Go版本**
   ```bash
   make deploy
   ```

3. **验证功能**
   ```bash
   kubectl logs -f deployment/pod-to-clb-controller-go
   ```

### 配置兼容性

- rules.yaml格式完全兼容
- 环境变量名称保持一致
- 功能行为完全一致

## 开发指南

### 代码结构

- `main.go`: 主控制逻辑和Pod监控
- `tencent.go`: 腾讯云CLB API封装
- `config.go`: 配置文件加载和缓存管理

### 添加新功能

1. 遵循Go代码规范
2. 添加适当的错误处理
3. 更新测试用例
4. 更新文档

### 测试

```bash
# 运行测试
make test

# 代码格式化
make fmt

# 代码检查
make vet

# 代码质量检查
make lint
```

## 贡献

欢迎提交Issue和Pull Request来改进项目。

## 许可证

本项目采用与原Python版本相同的许可证。