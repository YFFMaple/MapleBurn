// 与 Go 后端的 API 契约
export interface Totals {
  requests: number;
  inputTokens: number;
  outputTokens: number;
  cacheCreate: number;
  cacheRead: number;
  totalTokens: number;
}

export interface ModelStat {
  model: string;
  requests: number;
  inputTokens: number;
  outputTokens: number;
  cacheCreate: number;
  cacheRead: number;
}

export interface DayStat extends Totals {
  date: string;
}

export interface ProjectStat extends Totals {
  project: string;
}

export interface SessionStat {
  sessionId: string;
  project: string;
  models: string;
  requests: number;
  totalTokens: number;
  firstTs: string;
  lastTs: string;
}

export interface UsageRow {
  ts: number;
  model: string;
  project: string;
  sessionId: string;
  inputTokens: number;
  outputTokens: number;
  cacheCreate: number;
  cacheRead: number;
}

export interface ToolStat {
  tool: string;
  count: number;
  errors: number;
}

export interface SearchHit {
  sessionId: string;
  project: string;
  role: string;
  ts: string;
  text: string;
}

export interface ModelEfficiency {
  model: string;
  requests: number;
  cacheHitRate: number;
  avgOutput: number;
  avgTotal: number;
}

export interface Efficiency {
  cacheHitRate: number;
  avgPerRequest: number;
  models: ModelEfficiency[];
}

export interface FileStat {
  path: string;
  ops: string;
  count: number;
}

export interface Report {
  period: string;
  start: string;
  end: string;
  totals: Totals;
  prevTotals: Totals;
  activeDays: number;
  models: ModelStat[];
  daily: DayStat[];
}

export interface SessionMessage {
  role: string;
  text: string;
  ts: string;
  model?: string;
  inputTokens: number;
  outputTokens: number;
}

export interface Overview {
  today: Totals;
  week: Totals;
  month: Totals;
  allTime: Totals;
  dataDir: string;
  logDir: string;
}

export interface Adjustment {
  active: boolean;
  target: number;
  realTotal: number;
  factor: number;
}

export interface LogDirInfo {
  path: string;
  exists: boolean;
}

export interface ScanResult {
  filesScanned: number;
  newRecords: number;
  linesParsed: number;
  skippedDirs: string[];
}

// ---- 观测代理 ----
export interface ProxyStatus {
  enabled: boolean;
  running: boolean;
  addr: string;
  port: number;
  upstream: string;
  provider: string;
  providerDisplay: string;
  detectedUpstream: string;
}

export interface ProxyRequest {
  id: number;
  ts: number;
  provider: string;
  model: string;
  path: string;
  status: number;
  latencyMs: number;
  inputTokens: number;
  outputTokens: number;
  cacheCreate: number;
  cacheRead: number;
  error: string;
}

export interface Provider {
  name: string;
  displayName: string;
  adapter: string;
  hostContains: string[];
}

async function api<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path, init);
  if (!res.ok) throw new Error(await res.text());
  return res.json();
}

export const apiGet = <T>(path: string) => api<T>(path);
export const apiPost = <T>(path: string, body?: unknown) =>
  api<T>(path, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
export const apiDelete = <T>(path: string) => api<T>(path, { method: "DELETE" });
