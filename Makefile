# CampusHelper-Backend Makefile
# 简化的项目构建与开发命令

# =====================================================
# 变量定义
# =====================================================
PROJECT_NAME := campus-helper-backend
MODULE := go_projects/praProject1
PROTO_DIR := PB
PROTO_OUT := .
PROTO_FILES := $(wildcard $(PROTO_DIR)/*.proto)

# protoc 插件（必须已在 PATH 中）
PROTOC := protoc
PROTOC_GEN_GO := protoc-gen-go
PROTOC_GEN_GO_GRPC := protoc-gen-go-grpc

# =====================================================
# 默认目标
# =====================================================
.PHONY: help
help: ## 显示帮助信息
	@echo "CampusHelper-Backend 项目构建命令:"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'
	@echo ""

# =====================================================
# Protobuf 代码生成
# =====================================================
.PHONY: proto
proto: ## 重新生成所有 .proto 文件的 Go 代码
	@echo "🔧 正在生成 Protobuf Go 代码..."
	@for f in $(PROTO_FILES); do \
		echo "  → $$f"; \
		$(PROTOC) -I . \
			--go_out=$(PROTO_OUT) --go_opt=module=$(MODULE) \
			--go-grpc_out=$(PROTO_OUT) --go-grpc_opt=module=$(MODULE) \
			$$f || exit 1; \
	done
	@# 清理可能生成的多余目录（go_package 解析边界情况）
	@find $(PROTO_DIR)/pb -type d -name "PB" -exec rm -rf {} + 2>/dev/null || true
	@echo "✅ Protobuf 代码生成完成"

.PHONY: proto-content
proto-content: ## 重新生成 content.proto 的 Go 代码
	@echo "🔧 正在生成 content.proto Go 代码..."
	$(PROTOC) -I . \
		--go_out=$(PROTO_OUT) --go_opt=module=$(MODULE) \
		--go-grpc_out=$(PROTO_OUT) --go-grpc_opt=module=$(MODULE) \
		$(PROTO_DIR)/content.proto
	@find $(PROTO_DIR)/pb -type d -name "PB" -exec rm -rf {} + 2>/dev/null || true
	@echo "✅ content.proto 生成完成"

# =====================================================
# 编译与测试
# =====================================================
.PHONY: build
build: ## 编译整个项目
	@echo "🔨 正在编译..."
	go build ./...
	@echo "✅ 编译完成"

.PHONY: test
test: ## 运行所有测试
	@echo "🧪 正在运行测试..."
	go test ./... -v
	@echo "✅ 测试完成"

.PHONY: vet
vet: ## 运行 go vet 静态分析
	@echo "🔍 正在运行 go vet..."
	go vet ./...
	@echo "✅ 静态分析完成"

.PHONY: tidy
tidy: ## 整理 go.mod 依赖
	@echo "🧹 正在整理依赖..."
	go mod tidy
	@echo "✅ 依赖整理完成"

# =====================================================
# 组合命令
# =====================================================
.PHONY: all
all: proto build vet test ## 完整流程：生成 proto + 编译 + vet + 测试

.PHONY: clean
clean: ## 清理生成的多余文件
	@find $(PROTO_DIR)/pb -type d -name "PB" -exec rm -rf {} + 2>/dev/null || true
	@echo "✅ 清理完成"