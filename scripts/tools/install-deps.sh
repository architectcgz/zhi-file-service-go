#!/usr/bin/env bash
# 安装开发工具依赖
set -euo pipefail

echo ">>> 安装 golang-migrate..."
go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest

echo ">>> 安装 sqlc..."
go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest

echo ">>> 安装 goimports..."
go install golang.org/x/tools/cmd/goimports@latest

echo ">>> 安装 golangci-lint..."
curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b "$(go env GOPATH)/bin" latest

echo ">>> 工具安装完成"
echo "  请确认以下命令可用：migrate, sqlc, goimports, golangci-lint"
