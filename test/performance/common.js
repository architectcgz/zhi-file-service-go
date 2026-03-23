import http from "k6/http";
import crypto from "k6/crypto";
import exec from "k6/execution";

export function requireEnv(name) {
  const value = (__ENV[name] || "").trim();
  if (!value) {
    throw new Error(`${name} is required`);
  }
  return value;
}

export function authHeaders(token, extra = {}) {
  const headers = { ...extra };
  if (token) {
    headers.Authorization = `Bearer ${token}`;
  }
  return headers;
}

export function jsonHeaders(token, extra = {}) {
  return authHeaders(token, {
    "Content-Type": "application/json",
    ...extra,
  });
}

export function parseJSON(res) {
  if (!res || !res.body) {
    return null;
  }
  return JSON.parse(res.body);
}

export function responseData(res) {
  const payload = parseJSON(res);
  return payload ? payload.data : null;
}

export function headerValue(res, name) {
  if (!res || !res.headers) {
    return "";
  }
  const expected = name.toLowerCase();
  for (const [key, value] of Object.entries(res.headers)) {
    if (key.toLowerCase() === expected) {
      return Array.isArray(value) ? value[0] : value;
    }
  }
  return "";
}

export function sha256Hex(value) {
  return crypto.sha256(value, "hex");
}

export function uniqueToken(prefix, maxLength = 64) {
  const context = executionContext();
  const value = [
    prefix,
    context.scenarioName,
    context.vu.toString(36),
    context.iteration.toString(36),
    Date.now().toString(36),
    Math.random().toString(36).slice(2, 8),
  ]
    .join("-")
    .toLowerCase()
    .replace(/[^a-z0-9-]/g, "");
  return value.slice(0, maxLength);
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

export function requestId(prefix) {
  return uniqueToken(prefix, 96);
}

export function idempotencyKey(prefix) {
  return uniqueToken(prefix, 96);
}

export function tenantId(prefix = "perf") {
  return uniqueToken(prefix, 32);
}

export function createInlineFile(uploadBaseUrl, bearerToken, payload, options = {}) {
  const contentType = options.contentType || "image/png";
  const accessLevel = options.accessLevel || "PRIVATE";
  const fileName = options.fileName || `${uniqueToken("perf-file", 24)}.png`;
  const contentHash = sha256Hex(payload);

  const createRes = http.post(
    `${uploadBaseUrl}/api/v1/upload-sessions`,
    JSON.stringify({
      fileName,
      contentType,
      sizeBytes: payload.length,
      accessLevel,
      uploadMode: "INLINE",
      contentHash: {
        algorithm: "SHA256",
        value: contentHash,
      },
    }),
    {
      headers: jsonHeaders(bearerToken, {
        "X-Request-Id": requestId("bootstrap-upload-create"),
        "Idempotency-Key": idempotencyKey("bootstrap-upload-create"),
      }),
      tags: {
        operation: "bootstrap_create_upload_session",
      },
    },
  );
  if (createRes.status !== 201) {
    throw new Error(`create inline upload session failed: ${createRes.status} ${createRes.body}`);
  }

  const session = responseData(createRes);
  const sessionId = session && session.uploadSessionId;
  if (!sessionId) {
    throw new Error(`create inline upload session missing uploadSessionId: ${createRes.body}`);
  }

  const uploadRes = http.put(`${uploadBaseUrl}/api/v1/upload-sessions/${sessionId}/content`, payload, {
    headers: authHeaders(bearerToken, {
      "Content-Type": contentType,
      "X-Request-Id": requestId("bootstrap-upload-content"),
    }),
    tags: {
      operation: "bootstrap_upload_session_content",
    },
  });
  if (uploadRes.status !== 200) {
    throw new Error(`upload inline content failed: ${uploadRes.status} ${uploadRes.body}`);
  }

  const completeRes = http.post(
    `${uploadBaseUrl}/api/v1/upload-sessions/${sessionId}/complete`,
    JSON.stringify({
      contentHash: {
        algorithm: "SHA256",
        value: contentHash,
      },
    }),
    {
      headers: jsonHeaders(bearerToken, {
        "X-Request-Id": requestId("bootstrap-upload-complete"),
        "Idempotency-Key": idempotencyKey("bootstrap-upload-complete"),
      }),
      tags: {
        operation: "bootstrap_complete_upload_session",
      },
    },
  );
  if (completeRes.status !== 200) {
    throw new Error(`complete inline upload session failed: ${completeRes.status} ${completeRes.body}`);
  }

  const completed = responseData(completeRes);
  const fileId = completed && completed.fileId;
  if (!fileId) {
    throw new Error(`complete inline upload session missing fileId: ${completeRes.body}`);
  }

  return {
    fileId,
    uploadSessionId: sessionId,
    contentHash,
  };
}

export function createAccessTicket(accessBaseUrl, bearerToken, fileId, options = {}) {
  const res = http.post(
    `${accessBaseUrl}/api/v1/files/${fileId}/access-tickets`,
    JSON.stringify({
      expiresInSeconds: options.expiresInSeconds || 300,
      responseDisposition: options.responseDisposition || "attachment",
      responseFileName: options.responseFileName || "perf-download.bin",
    }),
    {
      headers: jsonHeaders(bearerToken, {
        "X-Request-Id": requestId("bootstrap-access-ticket"),
        "Idempotency-Key": idempotencyKey("bootstrap-access-ticket"),
      }),
      tags: {
        operation: "bootstrap_create_access_ticket",
      },
    },
  );
  if (res.status !== 201) {
    throw new Error(`create access ticket failed: ${res.status} ${res.body}`);
  }

  const data = responseData(res);
  if (!data || !data.ticket) {
    throw new Error(`create access ticket missing ticket: ${res.body}`);
  }
  return data;
}
