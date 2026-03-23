import http from "k6/http";
import { check, sleep } from "k6";

import {
  authHeaders,
  createAccessTicket,
  createInlineFile,
  idempotencyKey,
  jsonHeaders,
  requestId,
  requireEnv,
  responseData,
} from "./common.js";

const BASE_URL = __ENV.BASE_URL || "http://127.0.0.1:8081";
const UPLOAD_BASE_URL = __ENV.UPLOAD_BASE_URL || "http://127.0.0.1:8080";
const BEARER_TOKEN = requireEnv("BEARER_TOKEN");

const ACCESS_FIXTURE = `access-fixture-${"E".repeat(24 * 1024)}`;

export const options = {
  scenarios: {
    get_file: {
      executor: "constant-vus",
      vus: Number(__ENV.ACCESS_GET_FILE_VUS || 12),
      duration: __ENV.ACCESS_GET_FILE_DURATION || "1m",
      exec: "getFile",
    },
    create_access_ticket: {
      executor: "constant-vus",
      vus: Number(__ENV.ACCESS_CREATE_TICKET_VUS || 8),
      duration: __ENV.ACCESS_CREATE_TICKET_DURATION || "1m",
      startTime: "5s",
      exec: "createAccessTicketFlow",
    },
    resolve_download: {
      executor: "constant-vus",
      vus: Number(__ENV.ACCESS_DOWNLOAD_VUS || 10),
      duration: __ENV.ACCESS_DOWNLOAD_DURATION || "1m",
      startTime: "10s",
      exec: "downloadFile",
    },
    redirect_by_ticket: {
      executor: "constant-vus",
      vus: Number(__ENV.ACCESS_REDIRECT_VUS || 8),
      duration: __ENV.ACCESS_REDIRECT_DURATION || "1m",
      startTime: "15s",
      exec: "redirectByTicket",
    },
  },
  thresholds: {
    http_req_failed: ["rate<0.01"],
    "http_req_duration{scenario:get_file}": ["p(95)<200"],
    "http_req_duration{scenario:create_access_ticket}": ["p(95)<250"],
    "http_req_duration{scenario:resolve_download}": ["p(95)<300"],
    "http_req_duration{scenario:redirect_by_ticket}": ["p(95)<350"],
  },
};

export function setup() {
  const bootstrap = createInlineFile(UPLOAD_BASE_URL, BEARER_TOKEN, ACCESS_FIXTURE, {
    contentType: "image/png",
    fileName: "perf-access-fixture.png",
    accessLevel: "PRIVATE",
  });

  return {
    fileId: bootstrap.fileId,
  };
}

export function getFile(data) {
  const res = http.get(`${BASE_URL}/api/v1/files/${data.fileId}`, {
    headers: authHeaders(BEARER_TOKEN, {
      "X-Request-Id": requestId("access-get-file"),
    }),
    tags: {
      operation: "get_file",
    },
  });

  check(res, {
    "get file status is 200": (response) => response.status === 200,
    "get file returns requested file": (response) => responseData(response)?.fileId === data.fileId,
  });

  sleep(1);
}

export function createAccessTicketFlow(data) {
  const res = http.post(
    `${BASE_URL}/api/v1/files/${data.fileId}/access-tickets`,
    JSON.stringify({
      expiresInSeconds: 300,
      responseDisposition: "attachment",
      responseFileName: "perf-download.png",
    }),
    {
      headers: jsonHeaders(BEARER_TOKEN, {
        "X-Request-Id": requestId("access-create-ticket"),
        "Idempotency-Key": idempotencyKey("access-create-ticket"),
      }),
      tags: {
        operation: "create_access_ticket",
      },
    },
  );

  check(res, {
    "create access ticket status is 201": (response) => response.status === 201,
    "create access ticket returns ticket": (response) => Boolean(responseData(response)?.ticket),
  });

  sleep(1);
}

export function downloadFile(data) {
  const res = http.get(`${BASE_URL}/api/v1/files/${data.fileId}/download?disposition=attachment`, {
    headers: authHeaders(BEARER_TOKEN, {
      "X-Request-Id": requestId("access-download"),
    }),
    redirects: 0,
    tags: {
      operation: "download_file",
    },
  });

  check(res, {
    "download file status is 302": (response) => response.status === 302,
    "download file returns location": (response) => Boolean(response.headers.Location),
  });

  sleep(1);
}

export function redirectByTicket(data) {
  const ticket = createAccessTicket(BASE_URL, BEARER_TOKEN, data.fileId, {
    expiresInSeconds: 300,
    responseDisposition: "attachment",
    responseFileName: "perf-download.png",
  });

  const res = http.get(`${BASE_URL}/api/v1/access-tickets/${ticket.ticket}/redirect`, {
    redirects: 0,
    tags: {
      operation: "redirect_by_access_ticket",
    },
  });

  check(res, {
    "redirect by ticket status is 302": (response) => response.status === 302,
    "redirect by ticket returns location": (response) => Boolean(response.headers.Location),
  });

  sleep(1);
}
