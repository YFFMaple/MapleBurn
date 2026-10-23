import { useEffect, useState } from "react";
import { Copy, TrendingDown, TrendingUp } from "lucide-react";
import { apiGet, type Report } from "@/lib/api";
import { fmtFull, fmtNum } from "@/lib/utils";
import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { cn } from "@/lib/utils";

export function ReportDialog({ open, onOpenChange }: { open: boolean; onOpenChange: (o: boolean) => void }) {
  const [period, setPeriod] = useState<"week" | "month">("week");
  const [rep, setRep] = useState<Report | null>(null);
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    if (!open) return;
    apiGet<Report>(`/api/report?period=${period}`).then(setRep).catch(() => {});
  }, [open, period]);

  const copyText = async () => {
    if (!rep) return;
    const t = rep.totals;
    const lines = [
      `AI Token ${rep.period === "week" ? "周报" : "月报"}（${rep.start} ~ ${rep.end}）`,
      `总 Tokens：${fmtFull(t.totalTokens)}（输入 ${fmtFull(t.inputTokens)} / 输出 ${fmtFull(t.outputTokens)} / 缓存读 ${fmtFull(t.cacheRead)} / 缓存写 ${fmtFull(t.cacheCreate)}）`,
      `请求次数：${fmtFull(t.requests)} · 活跃天数：${rep.activeDays}`,
      "Top 模型：",
      ...rep.models.map(
        (m) =>
          `  ${m.model}：${fmtFull(m.inputTokens + m.outputTokens + m.cacheCreate + m.cacheRead)} tokens / ${fmtFull(m.requests)} 次`
      ),
    ];
    await navigator.clipboard.writeText(lines.join("\n"));
    setCopied(true);
    setTimeout(() => setCopied(false), 1500);
  };

  const delta = rep ? rep.totals.totalTokens - rep.prevTotals.totalTokens : 0;
  const pct = rep && rep.prevTotals.totalTokens > 0 ? ((delta / rep.prevTotals.totalTokens) * 100).toFixed(1) : null;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent open={open} className="max-w-2xl">
        <DialogHeader>
          <DialogTitle>
            {rep ? `${rep.period === "week" ? "本周" : "本月"}报告（${rep.start} ~ ${rep.end}）` : "周期报告"}
          </DialogTitle>
        </DialogHeader>
        <div className="flex gap-2">
          <Button variant={period === "week" ? "default" : "secondary"} size="sm" onClick={() => setPeriod("week")}>
            本周
          </Button>
          <Button variant={period === "month" ? "default" : "secondary"} size="sm" onClick={() => setPeriod("month")}>
            本月
          </Button>
          <div className="flex-1" />
          <Button variant="ghost" size="sm" onClick={copyText}>
            <Copy /> {copied ? "✓ 已复制" : "复制"}
          </Button>
        </div>
        {rep && (
          <>
            <div className="grid grid-cols-4 gap-3 max-md:grid-cols-2">
              <Cell
                label="总 Tokens"
                value={fmtNum(rep.totals.totalTokens)}
                sub={
                  pct !== null ? (
                    <span className={cn("inline-flex items-center gap-0.5", delta >= 0 ? "text-success" : "text-destructive")}>
                      {delta >= 0 ? <TrendingUp className="size-3" /> : <TrendingDown className="size-3" />}
                      {delta >= 0 ? "+" : ""}
                      {pct}%
                    </span>
                  ) : (
                    "上期为 0"
                  )
                }
              />
              <Cell label="请求次数" value={fmtFull(rep.totals.requests)} sub={`上期 ${fmtFull(rep.prevTotals.requests)}`} />
              <Cell
                label="活跃天数"
                value={String(rep.activeDays)}
                sub={`日均 ${fmtNum(rep.activeDays > 0 ? Math.round(rep.totals.totalTokens / rep.activeDays) : 0)}`}
              />
              <Cell label="输出 Tokens" value={fmtNum(rep.totals.outputTokens)} sub={`上期 ${fmtNum(rep.prevTotals.outputTokens)}`} />
            </div>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Top 模型</TableHead>
                  <TableHead>请求数</TableHead>
                  <TableHead>总 tokens</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {rep.models.map((m) => (
                  <TableRow key={m.model}>
                    <TableCell className="font-mono text-xs">{m.model}</TableCell>
                    <TableCell>{fmtFull(m.requests)}</TableCell>
                    <TableCell>{fmtFull(m.inputTokens + m.outputTokens + m.cacheCreate + m.cacheRead)}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </>
        )}
      </DialogContent>
    </Dialog>
  );
}

function Cell({ label, value, sub }: { label: string; value: string; sub: React.ReactNode }) {
  return (
    <div className="rounded-lg border bg-card p-3">
      <div className="text-xs text-muted-foreground">{label}</div>
      <div className="mt-0.5 text-lg font-semibold tabular-nums">{value}</div>
      <div className="mt-0.5 text-xs text-muted-foreground">{sub}</div>
    </div>
  );
}
