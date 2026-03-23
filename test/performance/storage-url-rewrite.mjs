const DEFAULT_STORAGE_ENDPOINT_REWRITE_FROM = [
  "http://host.docker.internal:19000",
  "http://zhi-file-perf-minio:9000",
];

const DEFAULT_STORAGE_ENDPOINT_REWRITE_TO = "http://127.0.0.1:19000";

function normalizePrefixes(value) {
  if (Array.isArray(value)) {
    return value.flatMap((entry) => normalizePrefixes(entry));
  }
  if (value === undefined || value === null) {
    return [];
  }
  return String(value)
    .split(/[,\n]/)
    .map((entry) => entry.trim())
    .filter(Boolean);
}

function mergePrefixes(explicitPrefixes) {
  const seen = new Set();
  const merged = [];
  for (const prefix of [...explicitPrefixes, ...DEFAULT_STORAGE_ENDPOINT_REWRITE_FROM]) {
    if (seen.has(prefix)) {
      continue;
    }
    seen.add(prefix);
    merged.push(prefix);
  }
  return merged;
}

export function createStorageURLRewriter({ from, to } = {}) {
  if (from === "" || to === "") {
    return (value) => value;
  }

  const target = to === undefined ? DEFAULT_STORAGE_ENDPOINT_REWRITE_TO : to;
  const prefixes =
    from === undefined
      ? DEFAULT_STORAGE_ENDPOINT_REWRITE_FROM.slice()
      : mergePrefixes(normalizePrefixes(from));

  if (!target || prefixes.length === 0) {
    return (value) => value;
  }

  return (value) => {
    if (!value) {
      return value;
    }
    for (const prefix of prefixes) {
      if (value.startsWith(prefix)) {
        return target + value.slice(prefix.length);
      }
    }
    return value;
  };
}
