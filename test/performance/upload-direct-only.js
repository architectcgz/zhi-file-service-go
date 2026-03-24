import { directMultipartFlow } from "./upload-all-apis.js";

export const options = {
  scenarios: {
    direct_multipart_flow: {
      executor: "constant-vus",
      vus: Number(__ENV.UPLOAD_DIRECT_VUS || 10),
      duration: __ENV.UPLOAD_DIRECT_DURATION || "1m",
      exec: "directMultipartFlow",
    },
  },
  thresholds: {
    http_req_failed: ["rate<0.01"],
    "http_req_duration{scenario:direct_multipart_flow}": ["p(95)<2500"],
  },
};

export { directMultipartFlow };
