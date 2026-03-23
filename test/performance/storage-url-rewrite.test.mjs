import test from "node:test";
import assert from "node:assert/strict";

import { createStorageURLRewriter } from "./storage-url-rewrite.mjs";

test("rewrites host.docker.internal presigned URLs to localhost by default", () => {
  const rewrite = createStorageURLRewriter();

  const rewritten = rewrite(
    "http://host.docker.internal:19000/bucket/demo/file.bin?X-Amz-Signature=abc",
  );

  assert.equal(
    rewritten,
    "http://127.0.0.1:19000/bucket/demo/file.bin?X-Amz-Signature=abc",
  );
});

test("keeps legacy minio container address rewrite compatible", () => {
  const rewrite = createStorageURLRewriter();

  const rewritten = rewrite("http://zhi-file-perf-minio:9000/bucket/demo/file.bin");

  assert.equal(rewritten, "http://127.0.0.1:19000/bucket/demo/file.bin");
});

test("extends configured rewrite prefixes with built-in defaults", () => {
  const rewrite = createStorageURLRewriter({
    from: "http://custom-minio.internal:9000",
    to: "http://127.0.0.1:19000",
  });

  assert.equal(
    rewrite("http://custom-minio.internal:9000/bucket/demo/file.bin"),
    "http://127.0.0.1:19000/bucket/demo/file.bin",
  );
  assert.equal(
    rewrite("http://host.docker.internal:19000/bucket/demo/file.bin"),
    "http://127.0.0.1:19000/bucket/demo/file.bin",
  );
});

test("allows callers to disable rewrite explicitly via blank from", () => {
  const rewrite = createStorageURLRewriter({ from: "" });
  const original = "http://host.docker.internal:19000/bucket/demo/file.bin";
  assert.equal(rewrite(original), original);
});

test("allows callers to disable rewrite explicitly via blank to", () => {
  const rewrite = createStorageURLRewriter({ to: "" });
  const original = "http://host.docker.internal:19000/bucket/demo/file.bin";
  assert.equal(rewrite(original), original);
});
