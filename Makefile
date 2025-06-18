# Makefile for sync-pod-to-clb Go version

# 变量定义
APP_NAME := sync-pod-to-clb
GO_VERSION := 1.21
DOCKER_REGISTRY := hub.docker.com/oaixnah
IMAGE_NAME := $(DOCKER_REGISTRY)/$(APP_NAME)
TAG := latest
GO_TAG := go-$(TAG)

# Go 相关变量
GOOS := linux
GOARCH := amd64
CGO_ENABLED := 0

.PHONY: help build test clean docker-build docker-push deploy fmt vet mod-tidy run

# 默认目标
help: ## 显示帮助信息
	@echo "Available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-15s\033[0m %s\n", $$1, $$2}'

# 格式化代码
fmt: ## 格式化 Go 代码
	@echo "Formatting Go code..."
	go fmt ./...

# 代码检查
vet: ## 运行 go vet
	@echo "Running go vet..."
	go vet ./...

# 整理依赖
mod-tidy: ## 整理 Go 模块依赖
	@echo "Tidying Go modules..."
	go mod tidy

# 测试
test: ## 运行测试
	@echo "Running tests..."
	go test -v ./...

# 构建二进制文件
build: fmt vet ## 构建 Go 二进制文件
	@echo "Building $(APP_NAME)..."
	CGO_ENABLED=$(CGO_ENABLED) GOOS=$(GOOS) GOARCH=$(GOARCH) go build -a -installsuffix cgo -o $(APP_NAME) .

# 本地运行
run: ## 本地运行应用
	@echo "Running $(APP_NAME) locally..."
	go run .

# 清理构建文件
clean: ## 清理构建文件
	@echo "Cleaning up..."
	rm -f $(APP_NAME)
	docker rmi $(IMAGE_NAME):$(GO_TAG) 2>/dev/null || true

# 构建 Docker 镜像
docker-build: ## 构建 Docker 镜像
	@echo "Building Docker image $(IMAGE_NAME):$(GO_TAG)..."
	docker build -f Dockerfile -t $(IMAGE_NAME):$(GO_TAG) .
	docker tag $(IMAGE_NAME):$(GO_TAG) $(IMAGE_NAME):go-latest

# 推送 Docker 镜像
docker-push: docker-build ## 推送 Docker 镜像到仓库
	@echo "Pushing Docker image $(IMAGE_NAME):$(GO_TAG)..."
	docker push $(IMAGE_NAME):$(GO_TAG)
	docker push $(IMAGE_NAME):go-latest

# 部署到 Kubernetes
deploy: ## 部署到 Kubernetes
	@echo "Deploying to Kubernetes..."
	@if [ -f deployment.yaml ]; then \
		kubectl apply -f deployment.yaml; \
	else \
		echo "deployment.yaml not found. Please create it first."; \
		exit 1; \
	fi

# 创建 Go 版本的部署文件
deployment: ## 创建 Go 版本的 Kubernetes 部署文件
	@echo "Creating deployment.yaml..."
	@sed 's|hub.docker.com/oaixnah/sync-pod-to-clb:latest|$(IMAGE_NAME):$(GO_TAG)|g' deployment.yaml > deployment.yaml
	@echo "deployment.yaml created successfully"

# 查看日志
logs: ## 查看应用日志
	kubectl logs -f deployment/pod-to-clb-controller -n default

# 重启部署
restart: ## 重启 Kubernetes 部署
	kubectl rollout restart deployment/pod-to-clb-controller -n default

# 检查部署状态
status: ## 检查部署状态
	kubectl get deployment pod-to-clb-controller -n default
	kubectl get pods -l app=pod-to-clb-controller -n default

# 完整的构建和部署流程
all: mod-tidy test build docker-build docker-push deployment deploy ## 完整的构建和部署流程

# 开发环境设置
dev-setup: ## 设置开发环境
	@echo "Setting up development environment..."
	go mod download
	go install golang.org/x/tools/cmd/goimports@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# 代码质量检查
lint: ## 运行 golangci-lint
	@echo "Running golangci-lint..."
	golangci-lint run

# 安全检查
sec: ## 运行安全检查
	@echo "Running security checks..."
	go list -json -m all | nancy sleuth