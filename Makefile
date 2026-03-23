# zhi-file-service-go Makefile
# 命令分为五类：依赖环境 / 数据库与存储 bootstrap / 服务运行 / 测试验证 / 工具检查
# 所有配置通过环境变量注入，启动前先执行 make doctor 校验必填项

.PHONY: help deps infra-up infra-down \
        migrate-up migrate-down migrate-status \
        seed-dev seed-test bucket-init \
        run-upload run-access run-admin run-job \
        fmt lint openapi-validate doctor \
        test-unit test-integration test-contract test-e2e test-performance \
        build-upload build-access build-admin build-job build-all

# ============================================================
# 默认目标
# ============================================================

help:
	@echo ""
	@echo "zhi-file-service-go 开发命令"
	@echo ""
	@echo "  依赖环境"
	@echo "    deps               安装工具依赖（golang-migrate, sqlc, golangci-lint 等）"
	@echo ""
	@echo "  基础设施 bootstrap"
	@echo "    infra-up            启动 PostgreSQL / Redis / MinIO 容器"
	@echo "    infra-down          停止并清理容器"
	@echo "    bucket-init         初始化对象存储 bucket"
	@echo "    migrate-up          执行全部待执行 migration"
	@echo "    migrate-down        回滚最近一次 migration"
	@echo "    migrate-status      查看 migration 状态"
	@echo "    seed-dev            写入开发用 seed 数据"
	@echo "    seed-test           写入测试用 seed 数据（CI 使用）"
	@echo ""
	@echo "  服务运行"
	@echo "    run-upload          启动 upload-service"
	@echo "    run-access          启动 access-service"
	@echo "    run-admin           启动 admin-service"
	@echo "    run-job             启动 job-service"
	@echo ""
	@echo "  测试验证"
	@echo "    test-unit           单元测试"
	@echo "    test-integration    集成测试（需要真实 PG/Redis/MinIO）"
	@echo "    test-contract       OpenAPI 契约测试"
	@echo "    test-e2e            端到端测试"
	@echo "    test-performance    性能测试（bench 或 k6）"
	@echo ""
	@echo "  工具检查"
	@echo "    fmt                 格式化代码（gofmt + goimports）"
	@echo "    lint                静态分析（golangci-lint）"
	@echo "    openapi-validate    校验 OpenAPI YAML 合法性"
	@echo "    doctor              检查必填环境变量是否齐全"
	@echo ""

# ============================================================
# 1. 依赖环境
# ============================================================

deps:
	@echo ">>> 安装工具依赖..."
	scripts/tools/install-deps.sh

# ============================================================
# 2. 基础设施 bootstrap
# ============================================================

infra-up:
	@echo ">>> 启动本地依赖容器..."
	docker compose -f bootstrap/docker-compose.yml up -d

infra-down:
	@echo ">>> 停止本地依赖容器..."
	docker compose -f bootstrap/docker-compose.yml down

bucket-init:
	@echo ">>> 初始化对象存储 bucket..."
	scripts/bootstrap/bucket-init.sh

migrate-up:
	@echo ">>> 执行 migration..."
	scripts/bootstrap/migrate.sh up

migrate-down:
	@echo ">>> 回滚最近一次 migration..."
	scripts/bootstrap/migrate.sh down

migrate-status:
	@echo ">>> 查看 migration 状态..."
	scripts/bootstrap/migrate.sh status

seed-dev:
	@echo ">>> 写入开发 seed 数据..."
	scripts/bootstrap/seed.sh dev

seed-test:
	@echo ">>> 写入测试 seed 数据..."
	scripts/bootstrap/seed.sh test

# ============================================================
# 3. 服务运行
# ============================================================

run-upload:
	@echo ">>> 启动 upload-service..."
	APP_SERVICE_NAME=upload-service go run ./cmd/upload-service/...

run-access:
	@echo ">>> 启动 access-service..."
	APP_SERVICE_NAME=access-service go run ./cmd/access-service/...

run-admin:
	@echo ">>> 启动 admin-service..."
	APP_SERVICE_NAME=admin-service go run ./cmd/admin-service/...

run-job:
	@echo ">>> 启动 job-service..."
	APP_SERVICE_NAME=job-service go run ./cmd/job-service/...

# ============================================================
# 4. 测试验证
# ============================================================

test-unit:
	@echo ">>> 运行单元测试..."
	go test -count=1 -race ./internal/... ./pkg/...

test-integration:
	@echo ">>> 运行集成测试（需要真实依赖）..."
	go test -count=1 -race -tags=integration ./test/integration/...

test-contract:
	@echo ">>> 运行 OpenAPI 契约测试..."
	scripts/test/contract.sh

test-e2e:
	@echo ">>> 运行端到端测试..."
	scripts/test/e2e.sh

test-performance:
	@echo ">>> 运行性能测试..."
	scripts/test/performance.sh

# ============================================================
# 5. 工具检查
# ============================================================

fmt:
	@echo ">>> 格式化代码..."
	gofmt -w .
	goimports -w .

lint:
	@echo ">>> 静态分析..."
	golangci-lint run ./...

openapi-validate:
	@echo ">>> 校验 OpenAPI 契约..."
	scripts/tools/validate-openapi.sh

doctor:
	@echo ">>> 检查必填环境变量..."
	scripts/tools/doctor.sh

# ============================================================
# 6. 构建
# ============================================================

build-upload:
	go build -o bin/upload-service ./cmd/upload-service/...

build-access:
	go build -o bin/access-service ./cmd/access-service/...

build-admin:
	go build -o bin/admin-service ./cmd/admin-service/...

build-job:
	go build -o bin/job-service ./cmd/job-service/...

build-all: build-upload build-access build-admin build-job
