import { useCallback, useEffect, useState } from "react";
import { Copy, Download, RefreshCw, ChevronLeft, ChevronRight } from "lucide-react";
import {
  apiGet,
  apiPost,
  type ModelStat,
  type Provider,
  type ProxyRequest,
  type ProxyStatus,
} from "@/lib/api";
import { fmtFull, fmtNum, fmtTs, todayStr, addDays } from "@/lib/utils";
import { useApp } from "@/store";
import { useT } from "@/i18n";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow, Badge } from "@/components/ui/table";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Switch } from "@/components/ui/switch";
import { PctBar } from "@/components/PctBar";

const LIMIT = 50;

export function ProxyPage() {
  const t = useT();
  const refreshTick = useApp((s) => s.refreshTick);
  const [status, setStatus] = useState<ProxyStatus | null>(null);
  const [upstream, setUpstream] = useState("");
  const [requests, setRequests] = useState<ProxyRequest[]>([]);
  const [total, setTotal] = useState(0);
  const [offset, setOffset] = useState(0);
  const [summary, setSummary] = useState<{ models: ModelStat[]; requests: number }>({ models: [], requests: 0 });
  const [providers, setProviders] = useState<Provider[]>([]);
  const [fetchURL, setFetchURL] = useState("");
  const [msg, setMsg] = useState("");

  const loadStatus = useCallback(async () => {
    try {
      const st = await apiGet<ProxyStatus>("/api/proxy");
      setStatus(st);
      setUpstream(st.upstream || st.detectedUpstream);
    } catch {
      /* ignore */
    }
  }, []);

  const loadRequests = useCallback(async () => {
    try {
      const res = await apiGet<{ requests: ProxyRequest[]; total: number }>(
        `/api/proxy/requests?limit=${LIMIT}&offset=${offset}`
      );
      setRequests(res.requests || []);
      setTotal(res.total || 0);
    } catch {
      /* ignore */
    }
  }, [offset]);

  useEffect(() => {
    loadStatus();
    apiGet<Provider[]>("/api/proxy/providers").then(setProviders).catch(() => {});
    apiGet<{ models: ModelStat[]; requests: number }>(
      `/api/proxy/summary?start=${addDays(todayStr(), -29)}&end=${todayStr()}`
    )
      .then(setSummary)
      .catch(() => {});
  }, [refreshTick, loadStatus]);

  useEffect(() => {
    loadRequests();
    const t = setInterval(loadRequests, 5000); // 流水 5 秒轮询，接近实时
    return () => clearInterval(t);
  }, [loadRequests]);

  const apply = async (enabled: boolean) => {
    setMsg("");
    try {
      const st = await apiPost<ProxyStatus>("/api/proxy", {
        enabled,
        port: status?.port ?? 17841,
        upstream: upstream.trim(),
        provider: "",
      });
      setStatus(st);
      setMsg(enabled ? t("proxy.msgOn") : t("proxy.msgOff"));
    } catch (e) {
      setMsg(t("proxy.msgFail", { msg: (e as Error).message }));
      loadStatus(); // 失败后重新同步开关状态
    }
  };

  const copy = (text: string) => {
    navigator.clipboard.writeText(text);
    setMsg(t("proxy.copied", { text }));
  };

  const fetchProviders = async () => {
    if (!fetchURL.trim()) return;
    setMsg("");
    try {
      const list = await apiPost<Provider[]>("/api/proxy/providers/fetch", { url: fetchURL.trim() });
      setProviders(list);
      setFetchURL("");
      setMsg(t("proxy.fetchDone", { n: list.length }));
    } catch (e) {
      setMsg(t("proxy.fetchFail", { msg: (e as Error).message }));
    }
  };

  const page = Math.floor(offset / LIMIT) + 1;
  const pages = Math.max(1, Math.ceil(total / LIMIT));
  const sumMax = Math.max(
    ...summary.models.map((m) => m.inputTokens + m.outputTokens + m.cacheCreate + m.cacheRead),
    1
  );

  return (
    <div className="flex flex-col gap-4">
      {/* 开关与配置 */}
      <Card>
        <CardHeader>
          <CardTitle>{t("proxy.title")}</CardTitle>
          <div className="flex items-center gap-2">
            <span className="text-xs text-muted-foreground">{status?.enabled ? t("proxy.on") : t("proxy.off")}</span>
            <Switch checked={status?.enabled ?? false} onCheckedChange={apply} />
          </div>
        </CardHeader>
        <CardContent className="flex flex-col gap-3">
          <p className="text-xs text-muted-foreground">
            {t("proxy.desc")}
          </p>
          <div className="grid grid-cols-[auto_1fr_auto] items-center gap-x-3 gap-y-2 text-sm">
            <span className="text-xs text-muted-foreground">{t("proxy.addr")}</span>
            <code className="rounded bg-muted px-2 py-1 font-mono text-xs">{status?.addr ?? "-"}</code>
            <Button variant="ghost" size="sm" onClick={() => status && copy(status.addr)}>
              <Copy /> {t("settings.copy")}
            </Button>
            <span className="text-xs text-muted-foreground">{t("proxy.upstream")}</span>
            <Input
              className="h-8 font-mono text-xs"
              value={upstream}
              onChange={(e) => setUpstream(e.target.value)}
              placeholder={status?.detectedUpstream}
            />
            <Button variant="secondary" size="sm" onClick={() => apply(status?.enabled ?? false)}>
              {t("proxy.save")}
            </Button>
            <span className="text-xs text-muted-foreground">{t("proxy.provider")}</span>
            <span className="text-xs">
              {status ? `${status.providerDisplay}（${status.provider}）` : "-"}
              <span className="ml-2 text-muted-foreground">{t("proxy.autoMatch")}</span>
            </span>
            <Badge variant={status?.running ? "success" : "secondary"} className="justify-self-start">
              {status?.running ? t("proxy.running") : t("proxy.stopped")}
            </Badge>
          </div>
          {status?.upstream && status.upstream !== status.detectedUpstream && (
            <p className="text-xs text-muted-foreground">
              {t("proxy.detected", { url: status.detectedUpstream })}
            </p>
          )}
          {msg && <p className="text-xs text-primary">{msg}</p>}
        </CardContent>
      </Card>

      {/* 近 30 天汇总 */}
      <Card>
        <CardHeader>
          <CardTitle>{t("proxy.summaryTitle")}</CardTitle>
          <span className="text-xs text-muted-foreground">{t("proxy.summaryTotal", { n: fmtFull(summary.requests) })}</span>
        </CardHeader>
        <CardContent>
          {summary.models.length === 0 ? (
            <div className="py-6 text-center text-xs text-muted-foreground">
              {t("proxy.summaryEmpty")}
            </div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t("proxy.providerModel")}</TableHead>
                  <TableHead className="w-20">{t("table.requests")}</TableHead>
                  <TableHead className="w-24">{t("sessions.totalTokens")}</TableHead>
                  <TableHead className="w-[160px]"></TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {summary.models.map((m) => {
                  const t = m.inputTokens + m.outputTokens + m.cacheCreate + m.cacheRead;
                  return (
                    <TableRow key={m.model}>
                      <TableCell className="font-mono text-xs">{m.model}</TableCell>
                      <TableCell>{fmtFull(m.requests)}</TableCell>
                      <TableCell>{fmtNum(t)}</TableCell>
                      <TableCell><PctBar pct={(t / sumMax) * 100} /></TableCell>
                    </TableRow>
                  );
                })}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>

      {/* 请求流水 */}
      <Card>
        <CardHeader>
          <CardTitle>{t("proxy.requestsTitle")}</CardTitle>
          <Button variant="ghost" size="sm" onClick={loadRequests}>
            <RefreshCw />
          </Button>
        </CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t("events.time")}</TableHead>
                <TableHead>{t("proxy.providerCol")}</TableHead>
                <TableHead>{t("table.model")}</TableHead>
                <TableHead>{t("proxy.path")}</TableHead>
                <TableHead>{t("proxy.statusCol")}</TableHead>
                <TableHead>{t("proxy.latency")}</TableHead>
                <TableHead>{t("table.input")}</TableHead>
                <TableHead>{t("table.output")}</TableHead>
                <TableHead>{t("table.cacheRead")}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {requests.map((r) => (
                <TableRow key={r.id} title={r.error || undefined}>
                  <TableCell className="font-mono text-xs whitespace-nowrap">{fmtTs(r.ts)}</TableCell>
                  <TableCell className="font-mono text-xs">{r.provider}</TableCell>
                  <TableCell className="font-mono text-xs">{r.model || "-"}</TableCell>
                  <TableCell className="max-w-[160px] truncate font-mono text-xs" title={r.path}>{r.path}</TableCell>
                  <TableCell>
                    {r.error ? (
                      <Badge variant="destructive">{t("proxy.failed")}</Badge>
                    ) : (
                      <Badge variant={r.status < 400 ? "success" : "destructive"}>{r.status}</Badge>
                    )}
                  </TableCell>
                  <TableCell className="text-xs">{r.latencyMs >= 1000 ? (r.latencyMs / 1000).toFixed(1) + "s" : r.latencyMs + "ms"}</TableCell>
                  <TableCell className="text-xs">{r.inputTokens ? fmtFull(r.inputTokens) : "-"}</TableCell>
                  <TableCell className="text-xs">{r.outputTokens ? fmtFull(r.outputTokens) : "-"}</TableCell>
                  <TableCell className="text-xs">{r.cacheRead ? fmtFull(r.cacheRead) : "-"}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
          {requests.length === 0 && (
            <div className="py-8 text-center text-xs text-muted-foreground">{t("proxy.empty")}</div>
          )}
          <div className="mt-4 flex items-center justify-center gap-4">
            <Button variant="secondary" size="sm" disabled={offset <= 0} onClick={() => setOffset(Math.max(0, offset - LIMIT))}>
              <ChevronLeft /> {t("events.prev")}
            </Button>
            <span className="text-xs text-muted-foreground">
              {t("events.pager", { page, pages, total: fmtFull(total) })}
            </span>
            <Button variant="secondary" size="sm" disabled={offset + LIMIT >= total} onClick={() => setOffset(offset + LIMIT)}>
              {t("events.next")} <ChevronRight />
            </Button>
          </div>
        </CardContent>
      </Card>

      {/* 服务商注册表 */}
      <Card>
        <CardHeader>
          <CardTitle>{t("proxy.registryTitle")}</CardTitle>
          <span className="text-xs text-muted-foreground">{t("proxy.registryCount", { n: providers.length })}</span>
        </CardHeader>
        <CardContent className="flex flex-col gap-3">
          <div className="flex flex-wrap gap-1.5">
            {providers.map((p) => (
              <Badge key={p.name} variant="secondary" title={t("proxy.adapterTip", { adapter: p.adapter, hosts: p.hostContains.join(", ") })}>
                {p.displayName}
              </Badge>
            ))}
          </div>
          <div className="flex items-center gap-2">
            <Input
              className="h-8 flex-1 font-mono text-xs"
              placeholder={t("proxy.fetchPh")}
              value={fetchURL}
              onChange={(e) => setFetchURL(e.target.value)}
              onKeyDown={(e) => e.key === "Enter" && fetchProviders()}
            />
            <Button variant="secondary" size="sm" onClick={fetchProviders}>
              <Download /> {t("proxy.fetch")}
            </Button>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
