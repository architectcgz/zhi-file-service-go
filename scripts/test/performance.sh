#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/../.." && pwd)"
GO_BIN="${GO_BIN:-go}"
K6_BIN="${K6_BIN:-k6}"
PERF_MODE="${PERF_MODE:-bench}"
PERF_TARGET="${PERF_TARGET:-all}"

UPLOAD_BENCH='Benchmark(CreateUploadSessionInline|CompleteUploadSessionPresignedSingle)'
ACCESS_BENCH='Benchmark(GetFilePublic|ResolveDownloadPrivate|RedirectByAccessTicketPrivate)'

run_bench_upload() {
  "${GO_BIN}" test -run '^$' -bench "${UPLOAD_BENCH}" -benchmem ./internal/services/upload/app/commands
}

run_bench_access() {
  "${GO_BIN}" test -run '^$' -bench "${ACCESS_BENCH}" -benchmem ./internal/services/access/app/queries
}

run_k6_upload() {
  "${K6_BIN}" run test/performance/upload-session-hotpath.js
}

run_k6_access() {
  "${K6_BIN}" run test/performance/access-read-hotpath.js
}

run_target() {
  local upload_runner="$1"
  local access_runner="$2"

  case "${PERF_TARGET}" in
    upload)
      "${upload_runner}"
      ;;
    access)
      "${access_runner}"
      ;;
    all)
      "${upload_runner}"
      "${access_runner}"
      ;;
    *)
      echo "unsupported PERF_TARGET: ${PERF_TARGET}" >&2
      return 1
      ;;
  esac
}

cd "${ROOT_DIR}"

case "${PERF_MODE}" in
  bench)
    run_target run_bench_upload run_bench_access
    ;;
  k6)
    run_target run_k6_upload run_k6_access
    ;;
  *)
    echo "unsupported PERF_MODE: ${PERF_MODE}" >&2
    exit 1
    ;;
esac
