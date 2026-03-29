.PHONY: build run test clean deps docker-build docker-run help

# 变量
BINARY_NAME := ai-code-review
MAIN_PATH := ./cmd/server
CONFIG := config.yaml

# 默认目标
all: build

# 编译
build:
	@echo "Building..."
	@CGO_ENABLED=1 go build -o $(BINARY_NAME) $(MAIN_PATH)
	@echo "Done: $(BINARY_NAME)"

# 运行
run: build
	@echo "Running..."
	@./$(BINARY_NAME) -config $(CONFIG)

# 直接运行（不重新编译）
run-only:
	@./$(BINARY_NAME) -config $(CONFIG)

# 运行测试
test:
	@echo "Running tests..."
	@go test -v ./...

# 运行测试（简短输出）
test-short:
	@go test ./...

# 下载依赖
deps:
	@echo "Downloading dependencies..."
	@go mod download
	@go mod tidy

# 清理编译产物
clean:
	@echo "Cleaning..."
	@rm -f $(BINARY_NAME)
	@echo "Done"

# Docker 构建
docker-build:
	@echo "Building Docker image..."
	@docker build -t ai-code-review:latest .
	@echo "Done"

# Docker 运行（默认配置 server.port 为 8078，与 config.example.yaml 一致）
docker-run:
	@docker run -p 8078:8078 \
		-v $(PWD)/data:/app/data \
		-v $(PWD)/$(CONFIG):/app/$(CONFIG) \
		ai-code-review:latest

# 初始化配置（从示例复制）
init-config:
	@if [ ! -f $(CONFIG) ]; then \
		cp config.example.yaml $(CONFIG); \
		echo "Created $(CONFIG) from config.example.yaml"; \
	else \
		echo "$(CONFIG) already exists"; \
	fi

# 帮助
help:
	@echo "AI Code Review - Makefile targets:"
	@echo ""
	@echo "  make build        - 编译项目"
	@echo "  make run          - 编译并运行"
	@echo "  make run-only     - 直接运行（不编译）"
	@echo "  make test         - 运行测试"
	@echo "  make deps         - 下载/整理依赖"
	@echo "  make clean        - 清理编译产物"
	@echo "  make docker-build - 构建 Docker 镜像"
	@echo "  make docker-run   - 运行 Docker 容器"
	@echo "  make init-config  - 从示例创建 config.yaml"
	@echo "  make help         - 显示此帮助"
