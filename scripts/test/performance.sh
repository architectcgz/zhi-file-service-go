#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/../.." && pwd)"
GO_BIN="${GO_BIN:-go}"
K6_BIN="${K6_BIN:-k6}"
PERF_MODE="${PERF_MODE:-bench}"
PERF_TARGET="${PERF_TARGET:-all}"
PERF_K6_SUITE="${PERF_K6_SUITE:-hotpath}"
PERF_K6_WARMUP="${PERF_K6_WARMUP:-}"
PERF_K6_WARMUP_DURATION="${PERF_K6_WARMUP_DURATION:-2s}"
PERF_K6_WARMUP_VUS="${PERF_K6_WARMUP_VUS:-1}"

UPLOAD_BENCH='Benchmark(CreateUploadSessionInline|CompleteUploadSessionPresignedSingle)'
ACCESS_BENCH='Benchmark(GetFilePublic|ResolveDownloadPrivate|RedirectByAccessTicketPrivate)'

run_bench_upload() {
  "${GO_BIN}" test -run '^$' -bench "${UPLOAD_BENCH}" -benchmem ./internal/services/upload/app/commands
}

run_bench_access() {
  "${GO_BIN}" test -run '^$' -bench "${ACCESS_BENCH}" -benchmem ./internal/services/access/app/queries
}

run_k6_upload() {
  run_k6_target "upload"
}

run_k6_access() {
  run_k6_target "access"
}

run_k6_target() {
  local target="$1"
  local script=""

  script="$(k6_script_for_target "${target}")"
  maybe_run_k6_warmup "${target}" "${script}"
  "${K6_BIN}" run "${script}"
}

k6_script_for_target() {
  local target="$1"

  case "${PERF_K6_SUITE}" in
    hotpath)
      case "${target}" in
        upload) echo "test/performance/upload-session-hotpath.js" ;;
        access) echo "test/performance/access-read-hotpath.js" ;;
        *)
          echo "unsupported hotpath target: ${target}" >&2
          return 1
          ;;
      esac
      ;;
    full-api)
      case "${target}" in
        upload) echo "test/performance/upload-all-apis.js" ;;
        access) echo "test/performance/access-all-apis.js" ;;
        *)
          echo "unsupported full-api target: ${target}" >&2
          return 1
          ;;
      esac
      ;;
    *)
      echo "unsupported PERF_K6_SUITE: ${PERF_K6_SUITE}" >&2
      return 1
      ;;
  esac
}

maybe_run_k6_warmup() {
  local target="$1"
  local script="$2"
  local should_run_rc=0
  local -a overrides=()

  if should_run_k6_warmup; then
    :
  else
    should_run_rc=$?
    if [[ ${should_run_rc} -eq 1 ]]; then
      return 0
    fi
    return "${should_run_rc}"
  fi

  mapfile -t overrides < <(k6_warmup_overrides_for_target "${target}")
  if [[ ${#overrides[@]} -eq 0 ]]; then
    return 0
  fi

  echo ">>> k6 warm-up: ${target} (${script})"
  env "${overrides[@]}" "${K6_BIN}" run "${script}"
}

should_run_k6_warmup() {
  if [[ -n "${PERF_K6_WARMUP}" ]]; then
    is_truthy "${PERF_K6_WARMUP}"
    return
  fi

  [[ "${PERF_K6_SUITE}" == "full-api" ]]
}

is_truthy() {
  case "${1,,}" in
    1|true|yes|on) return 0 ;;
    0|false|no|off|"") return 1 ;;
    *)
      echo "unsupported boolean value: $1" >&2
      return 2
      ;;
  esac
}

k6_warmup_overrides_for_target() {
  local target="$1"

  case "${target}" in
    upload)
      printf '%s\n' \
        "UPLOAD_INLINE_VUS=${PERF_K6_WARMUP_VUS}" \
        "UPLOAD_PRESIGNED_SINGLE_VUS=${PERF_K6_WARMUP_VUS}" \
        "UPLOAD_DIRECT_VUS=${PERF_K6_WARMUP_VUS}" \
        "UPLOAD_ABORT_VUS=${PERF_K6_WARMUP_VUS}" \
        "UPLOAD_INLINE_DURATION=${PERF_K6_WARMUP_DURATION}" \
        "UPLOAD_PRESIGNED_SINGLE_DURATION=${PERF_K6_WARMUP_DURATION}" \
        "UPLOAD_DIRECT_DURATION=${PERF_K6_WARMUP_DURATION}" \
        "UPLOAD_ABORT_DURATION=${PERF_K6_WARMUP_DURATION}"
      ;;
    access)
      printf '%s\n' \
        "ACCESS_GET_FILE_VUS=${PERF_K6_WARMUP_VUS}" \
        "ACCESS_CREATE_TICKET_VUS=${PERF_K6_WARMUP_VUS}" \
        "ACCESS_DOWNLOAD_VUS=${PERF_K6_WARMUP_VUS}" \
        "ACCESS_REDIRECT_VUS=${PERF_K6_WARMUP_VUS}" \
        "ACCESS_GET_FILE_DURATION=${PERF_K6_WARMUP_DURATION}" \
        "ACCESS_CREATE_TICKET_DURATION=${PERF_K6_WARMUP_DURATION}" \
        "ACCESS_DOWNLOAD_DURATION=${PERF_K6_WARMUP_DURATION}" \
        "ACCESS_REDIRECT_DURATION=${PERF_K6_WARMUP_DURATION}"
      ;;
  esac
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
