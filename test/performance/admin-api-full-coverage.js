import http from "k6/http";
import exec from "k6/execution";
import { check, fail, sleep } from "k6";

const ADMIN_BASE_URL = __ENV.BASE_URL || "http://127.0.0.1:18082";
const UPLOAD_BASE_URL = __ENV.UPLOAD_BASE_URL || "http://127.0.0.1:18080";
const ADMIN_BEARER_TOKEN = __ENV.ADMIN_BEARER_TOKEN || __ENV.BEARER_TOKEN || "";
const DATA_BEARER_TOKEN = __ENV.DATA_BEARER_TOKEN || __ENV.DATA_PLANE_TOKEN || "";
const TENANT_ID = __ENV.TENANT_ID || "demo";
const FILE_ID = __ENV.FILE_ID || "";
const RUN_SEED = (__ENV.RUN_SEED || `${Date.now().toString(36)}`).toLowerCase();

const INLINE_FILE_BODY = "perf-admin-delete-body-0001";
const INLINE_FILE_SIZE = INLINE_FILE_BODY.length;
const INLINE_FILE_HASH = "8bb05f85d3f74d70968c62234cf75ccb443ac36857b9f92c7df7bb90fe4f9a08";
const INLINE_FILE_NAME = `perf-admin-${RUN_SEED}.png`;
const INLINE_FILE_CONTENT_TYPE = "image/png";

export const options = {
  scenarios: {
    list_tenants: {
      executor: "constant-vus",
      vus: 4,
      duration: "45s",
      exec: "listTenants",
    },
    create_tenant: {
      executor: "constant-vus",
      vus: 2,
      duration: "30s",
      startTime: "2s",
      exec: "createTenant",
    },
    get_tenant: {
      executor: "constant-vus",
      vus: 4,
      duration: "45s",
      startTime: "4s",
      exec: "getTenant",
    },
    patch_tenant: {
      executor: "constant-vus",
      vus: 2,
      duration: "30s",
      startTime: "6s",
      exec: "patchTenant",
    },
    get_tenant_policy: {
      executor: "constant-vus",
      vus: 3,
      duration: "40s",
      startTime: "8s",
      exec: "getTenantPolicy",
    },
    patch_tenant_policy: {
      executor: "constant-vus",
      vus: 2,
      duration: "30s",
      startTime: "10s",
      exec: "patchTenantPolicy",
    },
    get_tenant_usage: {
      executor: "constant-vus",
      vus: 3,
      duration: "40s",
      startTime: "12s",
      exec: "getTenantUsage",
    },
    list_files: {
      executor: "constant-vus",
      vus: 4,
      duration: "45s",
      startTime: "14s",
      exec: "listFiles",
    },
    get_file: {
      executor: "constant-vus",
      vus: 4,
      duration: "45s",
      startTime: "16s",
      exec: "getFile",
    },
    delete_file: {
      executor: "constant-vus",
      vus: 1,
      duration: "20s",
      startTime: "18s",
      exec: "deleteFile",
    },
    list_audit_logs: {
      executor: "constant-vus",
      vus: 3,
      duration: "40s",
      startTime: "20s",
      exec: "listAuditLogs",
    },
  },
  thresholds: {
    "http_req_failed{service:admin}": ["rate<0.01"],
    "http_req_duration{service:admin,operation:list_tenants}": ["p(95)<300"],
    "http_req_duration{service:admin,operation:create_tenant}": ["p(95)<500"],
    "http_req_duration{service:admin,operation:get_tenant}": ["p(95)<300"],
    "http_req_duration{service:admin,operation:patch_tenant}": ["p(95)<500"],
    "http_req_duration{service:admin,operation:get_tenant_policy}": ["p(95)<300"],
    "http_req_duration{service:admin,operation:patch_tenant_policy}": ["p(95)<500"],
    "http_req_duration{service:admin,operation:get_tenant_usage}": ["p(95)<300"],
    "http_req_duration{service:admin,operation:list_files}": ["p(95)<300"],
    "http_req_duration{service:admin,operation:get_file}": ["p(95)<300"],
    "http_req_duration{service:admin,operation:delete_file}": ["p(95)<600"],
    "http_req_duration{service:admin,operation:list_audit_logs}": ["p(95)<400"],
  },
};

function jsonHeaders(token, requestID, extraHeaders = {}) {
  const headers = {
    "Content-Type": "application/json",
    "X-Request-Id": requestID,
    ...extraHeaders,
  };
  if (token) {
    headers.Authorization = `Bearer ${token}`;
  }
  return headers;
}

function binaryHeaders(token, requestID, contentType) {
  const headers = {
    "Content-Type": contentType,
    "X-Request-Id": requestID,
  };
  if (token) {
    headers.Authorization = `Bearer ${token}`;
  }
  return headers;
}

function adminRequestID(prefix) {
  return requestID(prefix, "admin");
}

function helperRequestID(prefix) {
  return requestID(prefix, "helper");
}

function requestID(prefix, namespace) {
  const context = executionContext();
  const scenarioName = sanitize(context.scenarioName);
  const vuID = context.vuID;
  const iteration = context.iteration;
  return `${prefix}-${namespace}-${RUN_SEED}-${scenarioName}-${vuID}-${iteration}`.slice(0, 96);
}

function executionContext() {
  const context = {
    scenarioName: "setup",
    vuID: 0,
    iteration: 0,
  };

  try {
    context.scenarioName = exec.scenario.name || context.scenarioName;
    context.iteration = exec.scenario.iterationInTest || 0;
    context.vuID = exec.vu.idInTest || 0;
  } catch (_) {
    return context;
  }

  return context;
}

function sanitize(value) {
  return String(value || "")
    .toLowerCase()
    .replace(/[^a-z0-9-]+/g, "-")
    .replace(/^-+|-+$/g, "");
}

function uniqueTenantID(prefix) {
  const context = executionContext();
  const scenarioName = sanitize(context.scenarioName);
  const vuID = context.vuID;
  const iteration = context.iteration;
  return `${prefix}-${RUN_SEED}-${scenarioName}-${vuID}-${iteration}`.toLowerCase().slice(0, 32);
}

function requireValue(name, value) {
  if (!value) {
    fail(`${name} is required`);
  }
  return value;
}

function adminRequest(method, path, body, operation, extra = {}) {
  const token = requireValue("ADMIN_BEARER_TOKEN", ADMIN_BEARER_TOKEN);
  const params = {
    headers: jsonHeaders(token, adminRequestID(operation), extra.headers || {}),
    tags: {
      service: "admin",
      operation,
    },
    redirects: extra.redirects,
  };
  if (extra.params) {
    Object.assign(params, extra.params);
  }

  if (method === "GET") {
    return http.get(`${ADMIN_BASE_URL}${path}`, params);
  }
  if (method === "POST") {
    return http.post(`${ADMIN_BASE_URL}${path}`, body, params);
  }
  if (method === "PATCH") {
    return http.patch(`${ADMIN_BASE_URL}${path}`, body, params);
  }
  if (method === "DELETE") {
    return http.del(`${ADMIN_BASE_URL}${path}`, body, params);
  }
  fail(`unsupported admin method: ${method}`);
}

function createManagedFile(dataToken) {
  const token = requireValue("DATA_BEARER_TOKEN", dataToken);
  const createRes = http.post(
    `${UPLOAD_BASE_URL}/api/v1/upload-sessions`,
    JSON.stringify({
      fileName: INLINE_FILE_NAME,
      contentType: INLINE_FILE_CONTENT_TYPE,
      sizeBytes: INLINE_FILE_SIZE,
      accessLevel: "PRIVATE",
      uploadMode: "INLINE",
      contentHash: {
        algorithm: "SHA256",
        value: INLINE_FILE_HASH,
      },
    }),
    {
      headers: jsonHeaders(token, helperRequestID("create-upload-session")),
      tags: {
        service: "upload-helper",
        operation: "create_upload_session",
      },
    },
  );
  if (createRes.status !== 201 && createRes.status !== 200) {
    fail(`create upload session failed: status=${createRes.status} body=${createRes.body}`);
  }

  const uploadSessionID = createRes.json("data.uploadSessionId");
  if (!uploadSessionID) {
    fail(`create upload session missing uploadSessionId: ${createRes.body}`);
  }

  const uploadRes = http.put(
    `${UPLOAD_BASE_URL}/api/v1/upload-sessions/${uploadSessionID}/content`,
    INLINE_FILE_BODY,
    {
      headers: binaryHeaders(token, helperRequestID("upload-inline-content"), INLINE_FILE_CONTENT_TYPE),
      tags: {
        service: "upload-helper",
        operation: "upload_session_content",
      },
    },
  );
  if (uploadRes.status !== 200) {
    fail(`upload inline content failed: status=${uploadRes.status} body=${uploadRes.body}`);
  }

  const completeRes = http.post(
    `${UPLOAD_BASE_URL}/api/v1/upload-sessions/${uploadSessionID}/complete`,
    JSON.stringify({
      contentHash: {
        algorithm: "SHA256",
        value: INLINE_FILE_HASH,
      },
    }),
    {
      headers: jsonHeaders(token, helperRequestID("complete-upload-session"), {
        "Idempotency-Key": helperRequestID("complete-upload-idem"),
      }),
      tags: {
        service: "upload-helper",
        operation: "complete_upload_session",
      },
    },
  );
  if (completeRes.status !== 200) {
    fail(`complete upload session failed: status=${completeRes.status} body=${completeRes.body}`);
  }

  const fileID = completeRes.json("data.fileId");
  if (!fileID) {
    fail(`complete upload session missing fileId: ${completeRes.body}`);
  }
  return fileID;
}

function ensureTenantExists(tenantID) {
  const res = adminRequest(
    "POST",
    "/api/admin/v1/tenants",
    JSON.stringify({
      tenantId: tenantID,
      tenantName: `Perf ${tenantID}`,
      contactEmail: `${tenantID}@example.com`,
      description: "performance test tenant",
    }),
    "setup_create_tenant",
    {
      headers: {
        "Idempotency-Key": `${tenantID}-setup-idem`,
      },
    },
  );
  if (res.status !== 201 && res.status !== 409) {
    fail(`ensure tenant failed: status=${res.status} body=${res.body}`);
  }
}

export function setup() {
  requireValue("ADMIN_BEARER_TOKEN", ADMIN_BEARER_TOKEN);
  if (!FILE_ID) {
    requireValue("DATA_BEARER_TOKEN", DATA_BEARER_TOKEN);
  }

  const patchTenantID = `pt-${RUN_SEED}`.slice(0, 32);
  ensureTenantExists(patchTenantID);

  const fileID = FILE_ID || createManagedFile(DATA_BEARER_TOKEN);
  return {
    tenantId: TENANT_ID,
    patchTenantId: patchTenantID,
    fileId: fileID,
  };
}

export function listTenants() {
  const res = adminRequest("GET", "/api/admin/v1/tenants?limit=20", null, "list_tenants");
  check(res, {
    "list tenants status is 200": (r) => r.status === 200,
    "list tenants has data": (r) => Array.isArray(r.json("data")),
  });
  sleep(1);
}

export function createTenant() {
  const tenantID = uniqueTenantID("ct");
  const res = adminRequest(
    "POST",
    "/api/admin/v1/tenants",
    JSON.stringify({
      tenantId: tenantID,
      tenantName: `Perf ${tenantID}`,
      contactEmail: `${tenantID}@example.com`,
      description: `created by ${tenantID}`,
    }),
    "create_tenant",
    {
      headers: {
        "Idempotency-Key": `${tenantID}-idem`,
      },
    },
  );
  check(res, {
    "create tenant status is 201": (r) => r.status === 201,
    "create tenant echoes tenant id": (r) => r.json("data.tenantId") === tenantID,
  });
  sleep(1);
}

export function getTenant(data) {
  const res = adminRequest("GET", `/api/admin/v1/tenants/${data.patchTenantId}`, null, "get_tenant");
  check(res, {
    "get tenant status is 200": (r) => r.status === 200,
    "get tenant returns target": (r) => r.json("data.tenantId") === data.patchTenantId,
  });
  sleep(1);
}

export function patchTenant(data) {
  const description = `patched-${adminRequestID("patch-tenant")}`;
  const res = adminRequest(
    "PATCH",
    `/api/admin/v1/tenants/${data.patchTenantId}`,
    JSON.stringify({
      description,
      contactEmail: `${data.patchTenantId}@perf.example.com`,
    }),
    "patch_tenant",
    {
      headers: {
        "Idempotency-Key": `${adminRequestID("patch-tenant-idem")}`,
      },
    },
  );
  check(res, {
    "patch tenant status is 200": (r) => r.status === 200,
    "patch tenant keeps target": (r) => r.json("data.tenantId") === data.patchTenantId,
  });
  sleep(1);
}

export function getTenantPolicy(data) {
  const res = adminRequest(
    "GET",
    `/api/admin/v1/tenants/${data.patchTenantId}/policy`,
    null,
    "get_tenant_policy",
  );
  check(res, {
    "get tenant policy status is 200": (r) => r.status === 200,
    "get tenant policy returns target": (r) => r.json("data.tenantId") === data.patchTenantId,
  });
  sleep(1);
}

export function patchTenantPolicy(data) {
  const iteration = exec.scenario.iterationInTest;
  const res = adminRequest(
    "PATCH",
    `/api/admin/v1/tenants/${data.patchTenantId}/policy`,
    JSON.stringify({
      maxStorageBytes: 1073741824 + iteration,
      maxFileCount: 1000 + iteration,
      maxSingleFileSize: 1048576,
      defaultAccessLevel: "PRIVATE",
      autoCreateEnabled: false,
      reason: "performance policy update",
    }),
    "patch_tenant_policy",
    {
      headers: {
        "Idempotency-Key": `${adminRequestID("patch-policy-idem")}`,
      },
    },
  );
  check(res, {
    "patch tenant policy status is 200": (r) => r.status === 200,
    "patch tenant policy returns target": (r) => r.json("data.tenantId") === data.patchTenantId,
  });
  sleep(1);
}

export function getTenantUsage(data) {
  const res = adminRequest("GET", `/api/admin/v1/tenants/${data.tenantId}/usage`, null, "get_tenant_usage");
  check(res, {
    "get tenant usage status is 200": (r) => r.status === 200,
    "get tenant usage returns tenant": (r) => r.json("data.tenantId") === data.tenantId,
  });
  sleep(1);
}

export function listFiles(data) {
  const res = adminRequest(
    "GET",
    `/api/admin/v1/files?tenantId=${data.tenantId}&status=ACTIVE&limit=20`,
    null,
    "list_files",
  );
  check(res, {
    "list files status is 200": (r) => r.status === 200,
    "list files has data": (r) => Array.isArray(r.json("data")),
  });
  sleep(1);
}

export function getFile(data) {
  const res = adminRequest("GET", `/api/admin/v1/files/${data.fileId}`, null, "get_file");
  check(res, {
    "get file status is 200": (r) => r.status === 200,
    "get file returns target": (r) => r.json("data.fileId") === data.fileId,
  });
  sleep(1);
}

export function deleteFile() {
  const fileID = createManagedFile(DATA_BEARER_TOKEN);
  const res = adminRequest(
    "DELETE",
    `/api/admin/v1/files/${fileID}`,
    JSON.stringify({
      reason: "performance delete verification",
    }),
    "delete_file",
    {
      headers: {
        "Idempotency-Key": `${adminRequestID("delete-file-idem")}`,
      },
    },
  );
  check(res, {
    "delete file status is 200": (r) => r.status === 200,
    "delete file returns deleted status": (r) => r.json("data.status") === "DELETED",
    "delete file returns target": (r) => r.json("data.fileId") === fileID,
  });
  sleep(1);
}

export function listAuditLogs(data) {
  const res = adminRequest(
    "GET",
    `/api/admin/v1/audit-logs?tenantId=${data.tenantId}&limit=20`,
    null,
    "list_audit_logs",
  );
  check(res, {
    "list audit logs status is 200": (r) => r.status === 200,
    "list audit logs has data": (r) => Array.isArray(r.json("data")),
  });
  sleep(1);
}
