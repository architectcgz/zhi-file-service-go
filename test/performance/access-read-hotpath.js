import http from "k6/http";
import { check, sleep } from "k6";

const BASE_URL = __ENV.BASE_URL || "http://127.0.0.1:8080";
const BEARER_TOKEN = __ENV.BEARER_TOKEN || "";
const FILE_ID = __ENV.FILE_ID || "file-1";
const ACCESS_TICKET = __ENV.ACCESS_TICKET || "";
const DISPOSITION = __ENV.DISPOSITION || "attachment";

export const options = {
  scenarios: {
    get_file: {
      executor: "constant-vus",
      vus: 15,
      duration: "1m",
      exec: "getFile",
    },
    resolve_download: {
      executor: "constant-vus",
      vus: 10,
      duration: "1m",
      startTime: "5s",
      exec: "resolveDownload",
    },
    redirect_by_ticket: {
      executor: "constant-vus",
      vus: 10,
      duration: "1m",
      startTime: "10s",
      exec: "redirectByTicket",
      env: {
        ENABLE_REDIRECT: ACCESS_TICKET ? "true" : "false",
      },
    },
  },
  thresholds: {
    "http_req_failed{scenario:get_file}": ["rate<0.01"],
    "http_req_failed{scenario:resolve_download}": ["rate<0.01"],
    "http_req_duration{scenario:get_file}": ["p(95)<80"],
    "http_req_duration{scenario:resolve_download}": ["p(95)<120"],
    "http_req_duration{scenario:redirect_by_ticket}": ["p(95)<120"],
  },
};

function authHeaders() {
  const headers = {};
  if (BEARER_TOKEN) {
    headers.Authorization = `Bearer ${BEARER_TOKEN}`;
  }
  return headers;
}

export function getFile() {
  const res = http.get(`${BASE_URL}/api/v1/files/${FILE_ID}`, {
    headers: authHeaders(),
    tags: {
      operation: "get_file",
    },
  });

  check(res, {
    "get file status is 200": (r) => r.status === 200,
  });
  sleep(1);
}

export function resolveDownload() {
  const res = http.get(`${BASE_URL}/api/v1/files/${FILE_ID}/download?disposition=${DISPOSITION}`, {
    headers: authHeaders(),
    redirects: 0,
    tags: {
      operation: "resolve_download",
    },
  });

  check(res, {
    "resolve download status is 302": (r) => r.status === 302,
    "resolve download has location": (r) => Boolean(r.headers.Location),
  });
  sleep(1);
}

export function redirectByTicket() {
  if (!ACCESS_TICKET || __ENV.ENABLE_REDIRECT !== "true") {
    return;
  }

  const res = http.get(`${BASE_URL}/api/v1/access-tickets/${ACCESS_TICKET}/redirect`, {
    redirects: 0,
    tags: {
      operation: "redirect_by_access_ticket",
    },
  });

  check(res, {
    "redirect by ticket status is 302": (r) => r.status === 302,
    "redirect by ticket has location": (r) => Boolean(r.headers.Location),
  });
  sleep(1);
}
