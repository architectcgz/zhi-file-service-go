import http from "k6/http";
import { check, sleep } from "k6";

import { createStorageURLRewriter } from "./storage-url-rewrite.mjs";
import {
  authHeaders,
  idempotencyKey,
  jsonHeaders,
  requestId,
  requireEnv,
  responseData,
  sha256Hex,
  uniqueToken,
} from "./common.js";

const BASE_URL = __ENV.BASE_URL || "http://127.0.0.1:8080";
const BEARER_TOKEN = requireEnv("BEARER_TOKEN");
const STORAGE_ENDPOINT_REWRITE_FROM =
  "STORAGE_ENDPOINT_REWRITE_FROM" in __ENV ? __ENV.STORAGE_ENDPOINT_REWRITE_FROM : undefined;
const STORAGE_ENDPOINT_REWRITE_TO =
  "STORAGE_ENDPOINT_REWRITE_TO" in __ENV ? __ENV.STORAGE_ENDPOINT_REWRITE_TO : undefined;
const DIRECT_PART_ONE_BYTES = Number(__ENV.UPLOAD_DIRECT_PART_ONE_BYTES || 5 * 1024 * 1024);
const DIRECT_PART_TWO_BYTES = Number(__ENV.UPLOAD_DIRECT_PART_TWO_BYTES || 512 * 1024);
const rewriteStorageURL = createStorageURLRewriter({
  from: STORAGE_ENDPOINT_REWRITE_FROM,
  to: STORAGE_ENDPOINT_REWRITE_TO,
});

const INLINE_PAYLOAD_BASE = `inline-${"A".repeat(16 * 1024)}`;
const PRESIGNED_SINGLE_PAYLOAD_BASE = `presigned-single-${"B".repeat(32 * 1024)}`;
const DIRECT_PART_ONE_BASE = buildPayload("direct-part-one-", "C", DIRECT_PART_ONE_BYTES);
const DIRECT_PART_TWO_BASE = buildPayload("direct-part-two-", "D", DIRECT_PART_TWO_BYTES);

function buildPayload(prefix, fill, totalBytes) {
  const targetBytes = Math.max(totalBytes, prefix.length);
  return prefix + fill.repeat(targetBytes - prefix.length);
}

function payloadSuffix(prefix, length = 12) {
  return sha256Hex(uniqueToken(prefix, 64)).slice(0, length);
}

function inlineFixture() {
  const suffix = payloadSuffix("inline");
  const payload = `${INLINE_PAYLOAD_BASE}-${suffix}`;
  return {
    fileName: `perf-inline-${suffix}.png`,
    payload,
    hash: sha256Hex(payload),
  };
}

function presignedSingleFixture() {
  const suffix = payloadSuffix("presigned");
  const payload = `${PRESIGNED_SINGLE_PAYLOAD_BASE}-${suffix}`;
  return {
    fileName: `perf-presigned-${suffix}.bin`,
    payload,
    hash: sha256Hex(payload),
  };
}

function directFixture() {
  const suffix = payloadSuffix("direct");
  const partOne = DIRECT_PART_ONE_BASE;
  const partTwo = `${DIRECT_PART_TWO_BASE}-${suffix}`;
  return {
    fileName: `perf-direct-${suffix}.bin`,
    partOne,
    partTwo,
    hash: sha256Hex(partOne + partTwo),
  };
}

export const options = {
  scenarios: {
    inline_complete_flow: {
      executor: "constant-vus",
      vus: Number(__ENV.UPLOAD_INLINE_VUS || 6),
      duration: __ENV.UPLOAD_INLINE_DURATION || "1m",
      exec: "inlineCompleteFlow",
    },
    presigned_single_flow: {
      executor: "constant-vus",
      vus: Number(__ENV.UPLOAD_PRESIGNED_SINGLE_VUS || 5),
      duration: __ENV.UPLOAD_PRESIGNED_SINGLE_DURATION || "1m",
      startTime: "5s",
      exec: "presignedSingleFlow",
    },
    direct_multipart_flow: {
      executor: "constant-vus",
      vus: Number(__ENV.UPLOAD_DIRECT_VUS || 4),
      duration: __ENV.UPLOAD_DIRECT_DURATION || "1m",
      startTime: "10s",
      exec: "directMultipartFlow",
    },
    abort_flow: {
      executor: "constant-vus",
      vus: Number(__ENV.UPLOAD_ABORT_VUS || 2),
      duration: __ENV.UPLOAD_ABORT_DURATION || "1m",
      startTime: "15s",
      exec: "abortFlow",
    },
  },
  thresholds: {
    http_req_failed: ["rate<0.01"],
    "http_req_duration{scenario:inline_complete_flow}": ["p(95)<1500"],
    "http_req_duration{scenario:presigned_single_flow}": ["p(95)<1800"],
    "http_req_duration{scenario:direct_multipart_flow}": ["p(95)<2500"],
    "http_req_duration{scenario:abort_flow}": ["p(95)<1200"],
  },
};

function createUploadSession(body, operation) {
  return http.post(`${BASE_URL}/api/v1/upload-sessions`, JSON.stringify(body), {
    headers: jsonHeaders(BEARER_TOKEN, {
      "X-Request-Id": requestId(`${operation}-create`),
      "Idempotency-Key": idempotencyKey(`${operation}-create`),
    }),
    tags: {
      name: "POST /api/v1/upload-sessions",
      operation: "create_upload_session",
      flow: operation,
    },
  });
}

function getUploadSession(uploadSessionId, operation) {
  return http.get(`${BASE_URL}/api/v1/upload-sessions/${uploadSessionId}`, {
    headers: authHeaders(BEARER_TOKEN, {
      "X-Request-Id": requestId(`${operation}-get`),
    }),
    tags: {
      name: "GET /api/v1/upload-sessions/:uploadSessionId",
      operation: "get_upload_session",
      flow: operation,
    },
  });
}

function completeUploadSession(uploadSessionId, body, operation) {
  return completeUploadSessionWithKey(uploadSessionId, body, operation, idempotencyKey(`${operation}-complete`));
}

function completeUploadSessionWithKey(uploadSessionId, body, operation, completionKey) {
  return http.post(
    `${BASE_URL}/api/v1/upload-sessions/${uploadSessionId}/complete`,
    JSON.stringify(body),
    {
      headers: jsonHeaders(BEARER_TOKEN, {
        "X-Request-Id": requestId(`${operation}-complete`),
        "Idempotency-Key": completionKey,
      }),
      tags: {
        name: "POST /api/v1/upload-sessions/:uploadSessionId/complete",
        operation: "complete_upload_session",
        flow: operation,
      },
    },
  );
}

function completeWithRetry(uploadSessionId, body, operation, attempts = 5) {
  const completionKey = idempotencyKey(`${operation}-complete`);
  let lastRes = null;
  for (let attempt = 0; attempt < attempts; attempt += 1) {
    lastRes = completeUploadSessionWithKey(uploadSessionId, body, operation, completionKey);
    if (lastRes.status === 200) {
      return lastRes;
    }
    if (lastRes.status !== 409 && lastRes.status !== 503) {
      return lastRes;
    }
    sleep(0.2);
  }
  return lastRes;
}

export function inlineCompleteFlow() {
  const fixture = inlineFixture();
  const createRes = createUploadSession(
    {
      fileName: fixture.fileName,
      contentType: "image/png",
      sizeBytes: fixture.payload.length,
      accessLevel: "PRIVATE",
      uploadMode: "INLINE",
      contentHash: {
        algorithm: "SHA256",
        value: fixture.hash,
      },
    },
    "inline",
  );
  const created = responseData(createRes);
  const uploadSessionId = created && created.uploadSessionId;

  check(createRes, {
    "inline create session status is 201": (res) => res.status === 201,
    "inline create session has id": () => Boolean(uploadSessionId),
  });
  if (!uploadSessionId) {
    return;
  }

  const getRes = getUploadSession(uploadSessionId, "inline");
  check(getRes, {
    "inline get session status is 200": (res) => res.status === 200,
  });

  const uploadRes = http.put(`${BASE_URL}/api/v1/upload-sessions/${uploadSessionId}/content`, fixture.payload, {
    headers: authHeaders(BEARER_TOKEN, {
      "Content-Type": "image/png",
      "X-Request-Id": requestId("inline-content"),
    }),
    tags: {
      name: "PUT /api/v1/upload-sessions/:uploadSessionId/content",
      operation: "upload_session_content",
      flow: "inline",
    },
  });
  check(uploadRes, {
    "inline upload content status is 200": (res) => res.status === 200,
  });

  const completeRes = completeUploadSession(
    uploadSessionId,
    {
      contentHash: {
        algorithm: "SHA256",
        value: fixture.hash,
      },
    },
    "inline",
  );
  check(completeRes, {
    "inline complete status is 200": (res) => res.status === 200,
    "inline complete returns file id": (res) => Boolean(responseData(res)?.fileId),
  });

  sleep(1);
}

export function presignedSingleFlow() {
  const fixture = presignedSingleFixture();
  const createRes = createUploadSession(
    {
      fileName: fixture.fileName,
      contentType: "image/png",
      sizeBytes: fixture.payload.length,
      accessLevel: "PRIVATE",
      uploadMode: "PRESIGNED_SINGLE",
      contentHash: {
        algorithm: "SHA256",
        value: fixture.hash,
      },
    },
    "presigned-single",
  );
  const created = responseData(createRes);
  const uploadSessionId = created && created.uploadSessionId;
  const putUrl = created && created.putUrl;

  check(createRes, {
    "presigned single create session status is 201": (res) => res.status === 201,
    "presigned single create session has id": () => Boolean(uploadSessionId),
    "presigned single create session has putUrl": () => Boolean(putUrl),
  });
  if (!uploadSessionId || !putUrl) {
    return;
  }

  const getRes = getUploadSession(uploadSessionId, "presigned-single");
  check(getRes, {
    "presigned single get session status is 200": (res) => res.status === 200,
  });

  const storageRes = http.put(rewriteStorageURL(putUrl), fixture.payload, {
    headers: created.putHeaders || {},
    tags: {
      name: "PUT storage presigned single",
      operation: "storage_put_presigned_single",
      flow: "presigned-single",
    },
  });
  check(storageRes, {
    "presigned single storage put status is 200 or 204": (res) => res.status === 200 || res.status === 204,
  });

  const completeRes = completeUploadSession(
    uploadSessionId,
    {
      contentHash: {
        algorithm: "SHA256",
        value: fixture.hash,
      },
    },
    "presigned-single",
  );
  check(completeRes, {
    "presigned single complete status is 200": (res) => res.status === 200,
    "presigned single complete returns file id": (res) => Boolean(responseData(res)?.fileId),
  });

  sleep(1);
}

export function directMultipartFlow() {
  const fixture = directFixture();
  const createRes = createUploadSession(
    {
      fileName: fixture.fileName,
      contentType: "image/png",
      sizeBytes: fixture.partOne.length + fixture.partTwo.length,
      accessLevel: "PRIVATE",
      uploadMode: "DIRECT",
      totalParts: 2,
      contentHash: {
        algorithm: "SHA256",
        value: fixture.hash,
      },
    },
    "direct",
  );
  const created = responseData(createRes);
  const uploadSessionId = created && created.uploadSessionId;

  check(createRes, {
    "direct create session status is 201": (res) => res.status === 201,
    "direct create session has id": () => Boolean(uploadSessionId),
  });
  if (!uploadSessionId) {
    return;
  }

  const getRes = getUploadSession(uploadSessionId, "direct");
  check(getRes, {
    "direct get session status is 200": (res) => res.status === 200,
  });

  const presignRes = http.post(
    `${BASE_URL}/api/v1/upload-sessions/${uploadSessionId}/parts/presign`,
    JSON.stringify({
      expiresInSeconds: 600,
      parts: [{ partNumber: 1 }, { partNumber: 2 }],
    }),
    {
      headers: jsonHeaders(BEARER_TOKEN, {
        "X-Request-Id": requestId("direct-presign"),
      }),
      tags: {
        name: "POST /api/v1/upload-sessions/:uploadSessionId/parts/presign",
        operation: "presign_upload_session_parts",
        flow: "direct",
      },
    },
  );
  const presignedParts = responseData(presignRes)?.parts || [];
  check(presignRes, {
    "direct presign status is 200": (res) => res.status === 200,
    "direct presign returns two parts": () => presignedParts.length === 2,
  });
  if (presignedParts.length !== 2) {
    return;
  }

  const partBodies = {
    1: fixture.partOne,
    2: fixture.partTwo,
  };
  for (const part of presignedParts) {
    const storageRes = http.put(rewriteStorageURL(part.url), partBodies[part.partNumber], {
      headers: part.headers || {},
      tags: {
        name: "PUT storage multipart part",
        operation: "storage_put_multipart_part",
        flow: "direct",
        part_number: String(part.partNumber),
      },
    });
    check(storageRes, {
      [`direct part ${part.partNumber} upload status is 200 or 204`]: (res) =>
        res.status === 200 || res.status === 204,
    });
  }

  const listPartsRes = http.get(`${BASE_URL}/api/v1/upload-sessions/${uploadSessionId}/parts`, {
    headers: authHeaders(BEARER_TOKEN, {
      "X-Request-Id": requestId("direct-list-parts"),
    }),
    tags: {
      name: "GET /api/v1/upload-sessions/:uploadSessionId/parts",
      operation: "list_upload_session_parts",
      flow: "direct",
    },
  });
  const uploadedParts = responseData(listPartsRes)?.parts || [];
  check(listPartsRes, {
    "direct list parts status is 200": (res) => res.status === 200,
    "direct list parts returns two parts": () => uploadedParts.length === 2,
  });
  if (uploadedParts.length !== 2) {
    return;
  }

  const completeRes = completeWithRetry(
    uploadSessionId,
    {
      uploadedParts: uploadedParts.map((part) => ({
        partNumber: part.partNumber,
        etag: part.etag,
        sizeBytes: part.sizeBytes,
      })),
      contentHash: {
        algorithm: "SHA256",
        value: fixture.hash,
      },
    },
    "direct",
  );
  check(completeRes, {
    "direct complete status is 200": (res) => res.status === 200,
    "direct complete returns file id": (res) => Boolean(responseData(res)?.fileId),
  });

  sleep(1);
}

export function abortFlow() {
  const fixture = inlineFixture();
  const createRes = createUploadSession(
    {
      fileName: `perf-abort-${payloadSuffix("abort")}.txt`,
      contentType: "image/png",
      sizeBytes: fixture.payload.length,
      accessLevel: "PRIVATE",
      uploadMode: "INLINE",
      contentHash: {
        algorithm: "SHA256",
        value: fixture.hash,
      },
    },
    "abort",
  );
  const uploadSessionId = responseData(createRes)?.uploadSessionId;

  check(createRes, {
    "abort create session status is 201": (res) => res.status === 201,
    "abort create session has id": () => Boolean(uploadSessionId),
  });
  if (!uploadSessionId) {
    return;
  }

  const abortRes = http.post(
    `${BASE_URL}/api/v1/upload-sessions/${uploadSessionId}/abort`,
    JSON.stringify({
      reason: "performance-test abort flow",
    }),
    {
      headers: jsonHeaders(BEARER_TOKEN, {
        "X-Request-Id": requestId("abort-session"),
        "Idempotency-Key": idempotencyKey("abort-session"),
      }),
      tags: {
        name: "POST /api/v1/upload-sessions/:uploadSessionId/abort",
        operation: "abort_upload_session",
        flow: "abort",
      },
    },
  );
  check(abortRes, {
    "abort session status is 200": (res) => res.status === 200,
  });

  sleep(1);
}
