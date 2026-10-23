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
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow, Badge } from "@/components/ui/table";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Switch } from "@/components/ui/switch";
import { PctBar } from "@/components/PctBar";

const LIMIT = 50;

export function ProxyPage() {
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
      setMsg(
        enabled
          ? "已开启：Claude Code 的 base URL 已自动指向代理（原值已备份），重启 Claude Code 后生效"
          : "已关闭：Claude Code 的 base URL 已自动还原，重启 Claude Code 后生效"
      );
    } catch (e) {
      setMsg("操作失败：" + (e as Error).message);
      loadStatus(); // 失败后重新同步开关状态
    }
  };

  const copy = (text: string) => {
    navigator.clipboard.writeText(text);
    setMsg("已复制：" + text);
  };

  const fetchProviders = async () => {
    if (!fetchURL.trim()) return;
    setMsg("");
    try {
      const list = await apiPost<Provider[]>("/api/proxy/providers/fetch", { url: fetchURL.trim() });
      setProviders(list);
      setFetchURL("");
      setMsg(`注册表已更新，共 ${list.length} 个服务商`);
    } catch (e) {
      setMsg("更新失败：" + (e as Error).message);
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
          <CardTitle>观测代理</CardTitle>
          <div className="flex items-center gap-2">
            <span className="text-xs text-muted-foreground">{status?.enabled ? "已开启" : "已关闭"}</span>
            <Switch checked={status?.enabled ?? false} onCheckedChange={apply} />
          </div>
        </CardHeader>
        <CardContent className="flex flex-col gap-3">
          <p className="text-xs text-muted-foreground">
            开启后自动把 Claude Code 的 <code className="rounded bg-muted px-1">ANTHROPIC_BASE_URL</code> 指向代理地址（原值自动备份），
            透明转发到真实上游，只记录用量和延迟，不修改任何内容；关闭开关或退出程序时自动还原，绝不影响正常使用。
            改动需重启 Claude Code 生效。
          </p>
          <div className="grid grid-cols-[auto_1fr_auto] items-center gap-x-3 gap-y-2 text-sm">
            <span className="text-xs text-muted-foreground">代理地址</span>
            <code className="rounded bg-muted px-2 py-1 font-mono text-xs">{status?.addr ?? "-"}</code>
            <Button variant="ghost" size="sm" onClick={() => status && copy(status.addr)}>
              <Copy /> 复制
            </Button>
            <span className="text-xs text-muted-foreground">真实上游</span>
            <Input
              className="h-8 font-mono text-xs"
              value={upstream}
              onChange={(e) => setUpstream(e.target.value)}
              placeholder={status?.detectedUpstream}
            />
            <Button variant="secondary" size="sm" onClick={() => apply(status?.enabled ?? false)}>
              保存
            </Button>
            <span className="text-xs text-muted-foreground">当前服务商</span>
            <span className="text-xs">
              {status ? `${status.providerDisplay}（${status.provider}）` : "-"}
              <span className="ml-2 text-muted-foreground">按上游域名自动匹配</span>
            </span>
            <Badge variant={status?.running ? "success" : "secondary"} className="justify-self-start">
              {status?.running ? "运行中" : "未运行"}
            </Badge>
          </div>
          {status?.upstream && status.upstream !== status.detectedUpstream && (
            <p className="text-xs text-muted-foreground">
              检测到 Claude Code 当前配置的上游是 <code className="rounded bg-muted px-1">{status.detectedUpstream}</code>
            </p>
          )}
          {msg && <p className="text-xs text-primary">{msg}</p>}
        </CardContent>
      </Card>

      {/* 近 30 天汇总 */}
      <Card>
        <CardHeader>
          <CardTitle>近 30 天代理流量</CardTitle>
          <span className="text-xs text-muted-foreground">共 {fmtFull(summary.requests)} 次请求</span>
        </CardHeader>
        <CardContent>
          {summary.models.length === 0 ? (
            <div className="py-6 text-center text-xs text-muted-foreground">
              还没有代理流量。开启开关并把客户端指向代理地址后，这里会实时出现数据。
            </div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>服务商 / 模型</TableHead>
                  <TableHead className="w-20">请求数</TableHead>
                  <TableHead className="w-24">总 tokens</TableHead>
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
          <CardTitle>代理请求流水</CardTitle>
          <Button variant="ghost" size="sm" onClick={loadRequests}>
            <RefreshCw />
          </Button>
        </CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>时间</TableHead>
                <TableHead>服务商</TableHead>
                <TableHead>模型</TableHead>
                <TableHead>路径</TableHead>
                <TableHead>状态</TableHead>
                <TableHead>耗时</TableHead>
                <TableHead>输入</TableHead>
                <TableHead>输出</TableHead>
                <TableHead>缓存读</TableHead>
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
                      <Badge variant="destructive">失败</Badge>
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
            <div className="py-8 text-center text-xs text-muted-foreground">暂无记录</div>
          )}
          <div className="mt-4 flex items-center justify-center gap-4">
            <Button variant="secondary" size="sm" disabled={offset <= 0} onClick={() => setOffset(Math.max(0, offset - LIMIT))}>
              <ChevronLeft /> 上一页
            </Button>
            <span className="text-xs text-muted-foreground">
              第 {page} / {pages} 页 · 共 {fmtFull(total)} 条
            </span>
            <Button variant="secondary" size="sm" disabled={offset + LIMIT >= total} onClick={() => setOffset(offset + LIMIT)}>
              下一页 <ChevronRight />
            </Button>
          </div>
        </CardContent>
      </Card>

      {/* 服务商注册表 */}
      <Card>
        <CardHeader>
          <CardTitle>服务商注册表</CardTitle>
          <span className="text-xs text-muted-foreground">{providers.length} 个</span>
        </CardHeader>
        <CardContent className="flex flex-col gap-3">
          <div className="flex flex-wrap gap-1.5">
            {providers.map((p) => (
              <Badge key={p.name} variant="secondary" title={`adapter: ${p.adapter} · 匹配: ${p.hostContains.join(", ")}`}>
                {p.displayName}
              </Badge>
            ))}
          </div>
          <div className="flex items-center gap-2">
            <Input
              className="h-8 flex-1 font-mono text-xs"
              placeholder="注册表 URL（JSON，含 providers 数组），下载后合并进本地"
              value={fetchURL}
              onChange={(e) => setFetchURL(e.target.value)}
              onKeyDown={(e) => e.key === "Enter" && fetchProviders()}
            />
            <Button variant="secondary" size="sm" onClick={fetchProviders}>
              <Download /> 下载更新
            </Button>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
