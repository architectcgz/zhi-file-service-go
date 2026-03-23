import http from "k6/http";
import exec from "k6/execution";
import { check, sleep } from "k6";

const ADMIN_BASE_URL = stripTrailingSlash(__ENV.ADMIN_BASE_URL || "http://127.0.0.1:8082");
const ADMIN_BEARER_TOKEN = (__ENV.ADMIN_BEARER_TOKEN || "").trim();
const ADMIN_TENANT_PREFIX = sanitizeTenantPrefix(__ENV.ADMIN_TENANT_PREFIX || "k6admin");
const ADMIN_TENANT_EMAIL_DOMAIN = (__ENV.ADMIN_TENANT_EMAIL_DOMAIN || "perf.local").trim();
const ADMIN_LIST_LIMIT = parsePositiveInt(__ENV.ADMIN_LIST_LIMIT, 20);
const ADMIN_FILE_MODE = ((__ENV.ADMIN_FILE_MODE || "missing").trim().toLowerCase() || "missing");
const ADMIN_FILE_ID = (__ENV.ADMIN_FILE_ID || "").trim();
const ADMIN_AUDIT_ACTION = (__ENV.ADMIN_AUDIT_ACTION || "PATCH_TENANT_POLICY").trim();
const ADMIN_ACTOR_ID = (__ENV.ADMIN_ACTOR_ID || "").trim();
const ADMIN_DELETE_REASON = (__ENV.ADMIN_DELETE_REASON || "k6 governance cleanup").trim();
const ADMIN_RETIRE_REASON = (__ENV.ADMIN_RETIRE_REASON || "k6 tenant teardown").trim();
const ADMIN_SLEEP_SECONDS = parseFloatOrDefault(__ENV.ADMIN_SLEEP_SECONDS, 1);

const policyBaseline = {
  maxStorageBytes: parsePositiveInt(__ENV.ADMIN_POLICY_MAX_STORAGE_BYTES, 21474836480),
  maxFileCount: parsePositiveInt(__ENV.ADMIN_POLICY_MAX_FILE_COUNT, 200000),
  maxSingleFileSize: parsePositiveInt(__ENV.ADMIN_POLICY_MAX_SINGLE_FILE_SIZE, 4294967296),
  allowedMimeTypes: parseCSV(
    __ENV.ADMIN_POLICY_ALLOWED_MIME_TYPES || "application/pdf,image/png,video/mp4",
  ),
  allowedExtensions: parseCSV(__ENV.ADMIN_POLICY_ALLOWED_EXTENSIONS || "pdf,png,mp4"),
  defaultAccessLevel: (__ENV.ADMIN_POLICY_DEFAULT_ACCESS_LEVEL || "PRIVATE").trim().toUpperCase(),
  autoCreateEnabled: parseBoolean(__ENV.ADMIN_POLICY_AUTO_CREATE_ENABLED, false),
};

const fileModeIsExisting = ADMIN_FILE_MODE === "existing";

export const options = {
  scenarios: {
    create_shadow_tenant: {
      executor: "shared-iterations",
      vus: 1,
      iterations: 1,
      exec: "createShadowTenant",
    },
    list_tenants: {
      executor: "constant-vus",
      vus: 4,
      duration: "1m",
      startTime: "1s",
      exec: "listTenants",
    },
    get_tenant: {
      executor: "constant-vus",
      vus: 4,
      duration: "1m",
      startTime: "3s",
      exec: "getTenant",
    },
    patch_tenant: {
      executor: "constant-vus",
      vus: 1,
      duration: "40s",
      startTime: "5s",
      exec: "patchTenant",
    },
    get_tenant_policy: {
      executor: "constant-vus",
      vus: 3,
      duration: "1m",
      startTime: "7s",
      exec: "getTenantPolicy",
    },
    patch_tenant_policy: {
      executor: "constant-vus",
      vus: 1,
      duration: "40s",
      startTime: "9s",
      exec: "patchTenantPolicy",
    },
    get_tenant_usage: {
      executor: "constant-vus",
      vus: 3,
      duration: "1m",
      startTime: "11s",
      exec: "getTenantUsage",
    },
    list_admin_files: {
      executor: "constant-vus",
      vus: 3,
      duration: "1m",
      startTime: "13s",
      exec: "listAdminFiles",
    },
    get_admin_file: {
      executor: "constant-vus",
      vus: 1,
      duration: "30s",
      startTime: "15s",
      exec: "getAdminFile",
    },
    delete_admin_file: {
      executor: "per-vu-iterations",
      vus: 1,
      iterations: 1,
      startTime: "18s",
      exec: "deleteAdminFile",
    },
    list_audit_logs: {
      executor: "constant-vus",
      vus: 3,
      duration: "1m",
      startTime: "20s",
      exec: "listAuditLogs",
    },
  },
  thresholds: {
    "http_req_failed{scenario:create_shadow_tenant}": ["rate<0.01"],
    "http_req_failed{scenario:list_tenants}": ["rate<0.01"],
    "http_req_failed{scenario:get_tenant}": ["rate<0.01"],
    "http_req_failed{scenario:patch_tenant}": ["rate<0.01"],
    "http_req_failed{scenario:get_tenant_policy}": ["rate<0.01"],
    "http_req_failed{scenario:patch_tenant_policy}": ["rate<0.01"],
    "http_req_failed{scenario:get_tenant_usage}": ["rate<0.01"],
    "http_req_failed{scenario:list_admin_files}": ["rate<0.01"],
    "http_req_failed{scenario:list_audit_logs}": ["rate<0.01"],
    "http_req_duration{operation:list_tenants}": ["p(95)<300"],
    "http_req_duration{operation:get_tenant}": ["p(95)<300"],
    "http_req_duration{operation:patch_tenant}": ["p(95)<500"],
    "http_req_duration{operation:get_tenant_policy}": ["p(95)<300"],
    "http_req_duration{operation:patch_tenant_policy}": ["p(95)<500"],
    "http_req_duration{operation:get_tenant_usage}": ["p(95)<300"],
    "http_req_duration{operation:list_admin_files}": ["p(95)<300"],
    "http_req_duration{operation:list_audit_logs}": ["p(95)<300"],
  },
};

export function setup() {
  ensureAdminToken();

  const runId = buildRunID();
  const tenantId = buildTenantID(runId);
  const tenantName = buildTenantName(runId);

  const createResponse = requestJSON("POST", "/api/admin/v1/tenants", {
    expectedStatuses: [201],
    operation: "create_tenant_setup",
    idempotencyKey: buildKey("create-base", runId),
    body: {
      tenantId,
      tenantName,
      contactEmail: buildContactEmail(tenantId),
      description: `k6 admin governance baseline ${runId}`,
    },
  });
  const createPayload = parseJSON(createResponse);
  assertBodyField(createPayload, ["data", "tenantId"], tenantId, "setup create tenant");

  const patchTenantResponse = requestJSON("PATCH", `/api/admin/v1/tenants/${tenantId}`, {
    expectedStatuses: [200],
    operation: "patch_tenant_setup",
    idempotencyKey: buildKey("patch-base", runId),
    body: buildTenantPatch(tenantId, runId),
  });
  assertBodyField(parseJSON(patchTenantResponse), ["data", "tenantId"], tenantId, "setup patch tenant");

  const patchPolicyResponse = requestJSON(
    "PATCH",
    `/api/admin/v1/tenants/${tenantId}/policy`,
    {
      expectedStatuses: [200],
      operation: "patch_tenant_policy_setup",
      idempotencyKey: buildKey("policy-base", runId),
      body: buildPolicyPatch(),
    },
  );
  assertBodyField(parseJSON(patchPolicyResponse), ["data", "tenantId"], tenantId, "setup patch tenant policy");

  const fileId = resolveFileID(runId);

  return {
    runId,
    tenantId,
    tenantName,
    fileId,
    fileMode: fileModeIsExisting ? "existing" : "missing",
  };
}

export function teardown(data) {
  if (!data || !data.tenantId) {
    return;
  }

  requestJSON("PATCH", `/api/admin/v1/tenants/${data.tenantId}`, {
    expectedStatuses: [200, 404, 409],
    operation: "retire_tenant_teardown",
    idempotencyKey: buildKey("retire-base", data.runId || "teardown"),
    body: {
      status: "DELETED",
      reason: ADMIN_RETIRE_REASON,
      description: "k6 admin governance teardown",
    },
  });
}

export function createShadowTenant(data) {
  const shadowId = buildTenantID(`${data.runId || "run"}${padNumber(__ITER)}`);
  const createResponse = requestJSON("POST", "/api/admin/v1/tenants", {
    expectedStatuses: [201],
    operation: "create_tenant",
    idempotencyKey: buildKey("create-shadow", shadowId),
    body: {
      tenantId: shadowId,
      tenantName: buildTenantName(shadowId),
      contactEmail: buildContactEmail(shadowId),
      description: `k6 shadow tenant ${shadowId}`,
    },
  });
  const createPayload = parseJSON(createResponse);
  check(createPayload, {
    "shadow tenant created": (payload) => nestedValue(payload, ["data", "tenantId"]) === shadowId,
  });

  const retireResponse = requestJSON("PATCH", `/api/admin/v1/tenants/${shadowId}`, {
    expectedStatuses: [200],
    operation: "patch_tenant_shadow_retire",
    idempotencyKey: buildKey("retire-shadow", shadowId),
    body: {
      status: "DELETED",
      reason: ADMIN_RETIRE_REASON,
      description: "k6 shadow tenant cleanup",
    },
  });
  check(parseJSON(retireResponse), {
    "shadow tenant retired": (payload) => nestedValue(payload, ["data", "status"]) === "DELETED",
  });
}

export function listTenants() {
  const response = requestJSON(
    "GET",
    `/api/admin/v1/tenants?status=ACTIVE&limit=${ADMIN_LIST_LIMIT}`,
    {
      expectedStatuses: [200],
      operation: "list_tenants",
    },
  );
  const payload = parseJSON(response);
  check(payload, {
    "list tenants returns array": (body) => Array.isArray(body.data),
  });
  pause();
}

export function getTenant(data) {
  const response = requestJSON("GET", `/api/admin/v1/tenants/${data.tenantId}`, {
    expectedStatuses: [200],
    operation: "get_tenant",
  });
  const payload = parseJSON(response);
  check(payload, {
    "get tenant returns base tenant": (body) => nestedValue(body, ["data", "tenantId"]) === data.tenantId,
  });
  pause();
}

export function patchTenant(data) {
  const response = requestJSON("PATCH", `/api/admin/v1/tenants/${data.tenantId}`, {
    expectedStatuses: [200],
    operation: "patch_tenant",
    idempotencyKey: buildKey("patch-tenant", `${data.runId}-${__VU}-${__ITER}`),
    body: buildTenantPatch(data.tenantId, `${data.runId}-${__VU}-${__ITER}`),
  });
  const payload = parseJSON(response);
  check(payload, {
    "patch tenant keeps target tenant": (body) => nestedValue(body, ["data", "tenantId"]) === data.tenantId,
  });
  pause();
}

export function getTenantPolicy(data) {
  const response = requestJSON("GET", `/api/admin/v1/tenants/${data.tenantId}/policy`, {
    expectedStatuses: [200],
    operation: "get_tenant_policy",
  });
  const payload = parseJSON(response);
  check(payload, {
    "get tenant policy returns tenantId": (body) => nestedValue(body, ["data", "tenantId"]) === data.tenantId,
  });
  pause();
}

export function patchTenantPolicy(data) {
  const response = requestJSON(
    "PATCH",
    `/api/admin/v1/tenants/${data.tenantId}/policy`,
    {
      expectedStatuses: [200],
      operation: "patch_tenant_policy",
      idempotencyKey: buildKey("patch-policy", `${data.runId}-${__VU}-${__ITER}`),
      body: buildPolicyPatch(),
    },
  );
  const payload = parseJSON(response);
  check(payload, {
    "patch tenant policy returns tenantId": (body) => nestedValue(body, ["data", "tenantId"]) === data.tenantId,
  });
  pause();
}

export function getTenantUsage(data) {
  const response = requestJSON("GET", `/api/admin/v1/tenants/${data.tenantId}/usage`, {
    expectedStatuses: [200],
    operation: "get_tenant_usage",
  });
  const payload = parseJSON(response);
  check(payload, {
    "get tenant usage returns tenantId": (body) => nestedValue(body, ["data", "tenantId"]) === data.tenantId,
  });
  pause();
}

export function listAdminFiles(data) {
  const response = requestJSON(
    "GET",
    `/api/admin/v1/files?tenantId=${encodeURIComponent(data.tenantId)}&status=ACTIVE&limit=${ADMIN_LIST_LIMIT}`,
    {
      expectedStatuses: [200],
      operation: "list_admin_files",
    },
  );
  const payload = parseJSON(response);
  check(payload, {
    "list admin files returns array": (body) => Array.isArray(body.data),
  });
  pause();
}

export function getAdminFile(data) {
  const expectedStatuses = fileModeIsExisting ? [200, 404] : [404];
  const response = requestJSON("GET", `/api/admin/v1/files/${data.fileId}`, {
    expectedStatuses,
    operation: "get_admin_file",
  });
  const payload = parseJSON(response);
  check(payload, {
    "get admin file returns expected status": (body) =>
      fileModeIsExisting
        ? nestedValue(body, ["data", "fileId"]) === data.fileId ||
          nestedValue(body, ["error", "code"]) === "FILE_NOT_FOUND"
        : nestedValue(body, ["error", "code"]) === "FILE_NOT_FOUND",
  });
  pause();
}

export function deleteAdminFile(data) {
  const expectedStatuses = fileModeIsExisting ? [200, 404] : [404];
  const response = requestJSON("DELETE", `/api/admin/v1/files/${data.fileId}`, {
    expectedStatuses,
    operation: "delete_admin_file",
    idempotencyKey: buildKey("delete-file", `${data.runId}-${data.fileId}`),
    body: {
      reason: ADMIN_DELETE_REASON,
    },
  });
  const payload = parseJSON(response);
  check(payload, {
    "delete admin file is executable": (body) =>
      nestedValue(body, ["data", "status"]) === "DELETED" ||
      nestedValue(body, ["error", "code"]) === "FILE_NOT_FOUND",
  });
}

export function listAuditLogs(data) {
  const params = [
    ["tenantId", data.tenantId],
    ["limit", String(ADMIN_LIST_LIMIT)],
  ];
  if (ADMIN_AUDIT_ACTION) {
    params.push(["action", ADMIN_AUDIT_ACTION]);
  }
  if (ADMIN_ACTOR_ID) {
    params.push(["actorId", ADMIN_ACTOR_ID]);
  }

  const response = requestJSON("GET", `/api/admin/v1/audit-logs?${buildQueryString(params)}`, {
    expectedStatuses: [200],
    operation: "list_audit_logs",
  });
  const payload = parseJSON(response);
  check(payload, {
    "list audit logs returns array": (body) => Array.isArray(body.data),
  });
  pause();
}

function requestJSON(method, path, options) {
  const expectedStatuses = (options && options.expectedStatuses) || [200];
  const operation = (options && options.operation) || "admin_request";
  const body = options && Object.prototype.hasOwnProperty.call(options, "body") ? options.body : null;

  const response = http.request(method, `${ADMIN_BASE_URL}${path}`, body ? JSON.stringify(body) : null, {
    headers: buildHeaders(operation, options && options.idempotencyKey),
    tags: {
      operation,
    },
    responseCallback: http.expectedStatuses(...expectedStatuses),
  });

  check(response, {
    [`${operation} status accepted`]: (res) => expectedStatuses.indexOf(res.status) !== -1,
  });
  return response;
}

function buildHeaders(operation, idempotencyKey) {
  const context = executionContext();
  const headers = {
    "Content-Type": "application/json",
    "X-Request-Id": buildKey(
      `req-${operation}`,
      `${context.vu}-${context.iteration}-${Math.random().toString(36).slice(2, 8)}`,
    ),
  };
  if (idempotencyKey) {
    headers["Idempotency-Key"] = idempotencyKey;
  }
  if (ADMIN_BEARER_TOKEN) {
    headers.Authorization = `Bearer ${ADMIN_BEARER_TOKEN}`;
  }
  return headers;
}

function buildTenantPatch(tenantId, suffix) {
  return {
    tenantName: buildTenantName(`${tenantId}-${suffix}`),
    contactEmail: buildContactEmail(`${tenantId}-${suffix}`),
    description: `k6 tenant patch ${suffix}`,
  };
}

function buildPolicyPatch() {
  return {
    maxStorageBytes: policyBaseline.maxStorageBytes,
    maxFileCount: policyBaseline.maxFileCount,
    maxSingleFileSize: policyBaseline.maxSingleFileSize,
    allowedMimeTypes: policyBaseline.allowedMimeTypes,
    allowedExtensions: policyBaseline.allowedExtensions,
    defaultAccessLevel: policyBaseline.defaultAccessLevel,
    autoCreateEnabled: policyBaseline.autoCreateEnabled,
  };
}

function buildRunID() {
  return `${Date.now().toString(36)}${Math.random().toString(36).slice(2, 8)}`.slice(0, 12);
}

function buildTenantID(seed) {
  const suffix = compactAlphaNum(seed).slice(0, 16);
  return `${ADMIN_TENANT_PREFIX}${suffix}`.slice(0, 32);
}

function buildTenantName(seed) {
  return `K6 Admin ${compactAlphaNum(seed).slice(0, 12)}`;
}

function buildContactEmail(seed) {
  const local = compactAlphaNum(seed).slice(0, 24) || "k6admin";
  return `${local}@${ADMIN_TENANT_EMAIL_DOMAIN}`;
}

function resolveFileID(runId) {
  if (ADMIN_FILE_ID) {
    return ADMIN_FILE_ID;
  }
  const suffix = compactUpperAlphaNum(runId).padEnd(24, "A").slice(0, 24);
  return `01${suffix}`;
}

function buildKey(prefix, suffix) {
  return `${prefix}-${compactAlphaNum(suffix)}`.slice(0, 120);
}

function buildQueryString(entries) {
  return entries
    .map(([key, value]) => `${encodeURIComponent(key)}=${encodeURIComponent(value)}`)
    .join("&");
}

function executionContext() {
  const context = {
    scenarioName: "setup",
    vu: 0,
    iteration: 0,
  };

  try {
    context.scenarioName = exec.scenario.name || context.scenarioName;
    context.vu = exec.vu.idInTest || 0;
    context.iteration = exec.scenario.iterationInTest || 0;
  } catch (_) {
    return context;
  }

  return context;
}

function compactAlphaNum(value) {
  return String(value || "")
    .toLowerCase()
    .replace(/[^a-z0-9]/g, "");
}

function compactUpperAlphaNum(value) {
  return String(value || "")
    .toUpperCase()
    .replace(/[^A-Z0-9]/g, "");
}

function sanitizeTenantPrefix(value) {
  const normalized = compactAlphaNum(value);
  if (!normalized) {
    return "k6admin";
  }
  return normalized.slice(0, 12);
}

function stripTrailingSlash(value) {
  return String(value || "").replace(/\/+$/, "");
}

function parseJSON(response) {
  if (!response || !response.body) {
    return {};
  }
  try {
    return JSON.parse(response.body);
  } catch (_error) {
    return {};
  }
}

function nestedValue(target, path) {
  let current = target;
  for (const key of path) {
    if (!current || !Object.prototype.hasOwnProperty.call(current, key)) {
      return undefined;
    }
    current = current[key];
  }
  return current;
}

function assertBodyField(payload, path, expectedValue, label) {
  const actualValue = nestedValue(payload, path);
  if (actualValue !== expectedValue) {
    throw new Error(`${label} expected ${path.join(".")}=${expectedValue}, got ${actualValue}`);
  }
}

function parsePositiveInt(value, fallbackValue) {
  const parsed = Number.parseInt(value || "", 10);
  if (!Number.isFinite(parsed) || parsed <= 0) {
    return fallbackValue;
  }
  return parsed;
}

function parseFloatOrDefault(value, fallbackValue) {
  const parsed = Number.parseFloat(value || "");
  if (!Number.isFinite(parsed) || parsed < 0) {
    return fallbackValue;
  }
  return parsed;
}

function parseCSV(value) {
  return String(value || "")
    .split(",")
    .map((item) => item.trim())
    .filter(Boolean);
}

function parseBoolean(value, fallbackValue) {
  const normalized = String(value || "").trim().toLowerCase();
  if (!normalized) {
    return fallbackValue;
  }
  return normalized === "1" || normalized === "true" || normalized === "yes" || normalized === "on";
}

function ensureAdminToken() {
  if (!ADMIN_BEARER_TOKEN) {
    throw new Error("ADMIN_BEARER_TOKEN is required for admin-service k6 workload");
  }
  if (ADMIN_FILE_MODE !== "missing" && ADMIN_FILE_MODE !== "existing") {
    throw new Error(`unsupported ADMIN_FILE_MODE: ${ADMIN_FILE_MODE}`);
  }
  if (fileModeIsExisting && !ADMIN_FILE_ID) {
    throw new Error("ADMIN_FILE_ID is required when ADMIN_FILE_MODE=existing");
  }
}

function padNumber(value) {
  return String(value).padStart(4, "0");
}

function pause() {
  if (ADMIN_SLEEP_SECONDS > 0) {
    sleep(ADMIN_SLEEP_SECONDS);
  }
}
