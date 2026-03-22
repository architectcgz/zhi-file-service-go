import http from "k6/http";
import { check, sleep } from "k6";

const BASE_URL = __ENV.BASE_URL || "http://127.0.0.1:8080";
const BEARER_TOKEN = __ENV.BEARER_TOKEN || "";
const UPLOAD_SESSION_ID = __ENV.UPLOAD_SESSION_ID || "";
const TENANT_FILE_HASH =
  __ENV.TENANT_FILE_HASH ||
  "4f6f0d53c1efb6bb7c9f6b4e5b7d7e2b7b5b2f4b33f3ef0d4ec2ef9f74de4f75";

export const options = {
  scenarios: {
    create_session: {
      executor: "constant-vus",
      vus: 10,
      duration: "1m",
      exec: "createSession",
    },
    complete_session: {
      executor: "constant-vus",
      vus: 5,
      duration: "1m",
      startTime: "5s",
      exec: "completeSession",
      env: {
        ENABLE_COMPLETE: UPLOAD_SESSION_ID ? "true" : "false",
      },
    },
  },
  thresholds: {
    http_req_failed: ["rate<0.01"],
    http_req_duration: ["p(95)<2000"],
  },
};

function authHeaders() {
  const headers = {
    "Content-Type": "application/json",
  };
  if (BEARER_TOKEN) {
    headers.Authorization = `Bearer ${BEARER_TOKEN}`;
  }
  return headers;
}

export function createSession() {
  const payload = JSON.stringify({
    fileName: "load-test-avatar.png",
    contentType: "image/png",
    sizeBytes: 182044,
    accessLevel: "PUBLIC",
    uploadMode: "PRESIGNED_SINGLE",
    contentHash: {
      algorithm: "SHA256",
      value: TENANT_FILE_HASH,
    },
  });

  const res = http.post(`${BASE_URL}/api/v1/upload-sessions`, payload, {
    headers: authHeaders(),
    tags: {
      operation: "create_upload_session",
    },
  });

  check(res, {
    "create session status is 201 or 200": (r) => r.status === 201 || r.status === 200,
  });
  sleep(1);
}

export function completeSession() {
  if (!UPLOAD_SESSION_ID || __ENV.ENABLE_COMPLETE !== "true") {
    return;
  }

  const payload = JSON.stringify({
    contentHash: {
      algorithm: "SHA256",
      value: TENANT_FILE_HASH,
    },
  });

  const res = http.post(
    `${BASE_URL}/api/v1/upload-sessions/${UPLOAD_SESSION_ID}/complete`,
    payload,
    {
      headers: authHeaders(),
      tags: {
        operation: "complete_upload_session",
      },
    },
  );

  check(res, {
    "complete session status is 200 or 409": (r) => r.status === 200 || r.status === 409,
  });
  sleep(1);
}
