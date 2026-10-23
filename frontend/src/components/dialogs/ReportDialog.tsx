import { useEffect, useState } from "react";
import { Copy, ImageDown, TrendingDown, TrendingUp } from "lucide-react";
import { apiGet, type Report } from "@/lib/api";
import { exportReportImage } from "@/lib/reportCard";
import { fmtFull, fmtNum } from "@/lib/utils";
import { useT } from "@/i18n";
import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { cn } from "@/lib/utils";

export function ReportDialog({ open, onOpenChange }: { open: boolean; onOpenChange: (o: boolean) => void }) {
  const t = useT();
  const [period, setPeriod] = useState<"week" | "month">("week");
  const [rep, setRep] = useState<Report | null>(null);
  const [copied, setCopied] = useState(false);
  const [imgState, setImgState] = useState<"" | "clipboard" | "download">("");

  useEffect(() => {
    if (!open) return;
    apiGet<Report>(`/api/report?period=${period}`).then(setRep).catch(() => {});
  }, [open, period]);

  const copyText = async () => {
    if (!rep) return;
    const x = rep.totals;
    const lines = [
      `MapleBurn ${rep.period === "week" ? t("report.weekName") : t("report.monthName")}（${rep.start} ~ ${rep.end}）`,
      `${t("report.totalTokens")}：${fmtFull(x.totalTokens)}（${t("table.input")} ${fmtFull(x.inputTokens)} / ${t("table.output")} ${fmtFull(x.outputTokens)} / ${t("table.cacheRead")} ${fmtFull(x.cacheRead)} / ${t("table.cacheCreate")} ${fmtFull(x.cacheCreate)}）`,
      `${t("report.requests")}：${fmtFull(x.requests)} · ${t("report.activeDays")}：${rep.activeDays}`,
      `${t("report.topModels")}：`,
      ...rep.models.map(
        (m) =>
          `  ${m.model}：${fmtFull(m.inputTokens + m.outputTokens + m.cacheCreate + m.cacheRead)} tokens / ${fmtFull(m.requests)}`
      ),
    ];
    await navigator.clipboard.writeText(lines.join("\n"));
    setCopied(true);
    setTimeout(() => setCopied(false), 1500);
  };

  // 导出图片：渲染成卡片 PNG，优先进剪贴板，不行就下载
  const exportImage = async () => {
    if (!rep) return;
    try {
      const how = await exportReportImage(rep, t);
      setImgState(how);
      setTimeout(() => setImgState(""), 1500);
    } catch {
      /* 渲染失败就算了，不动 UI */
    }
  };

  const delta = rep ? rep.totals.totalTokens - rep.prevTotals.totalTokens : 0;
  const pct = rep && rep.prevTotals.totalTokens > 0 ? ((delta / rep.prevTotals.totalTokens) * 100).toFixed(1) : null;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent open={open} className="max-w-2xl">
        <DialogHeader>
          <DialogTitle>
            {rep
              ? t("report.title", {
                  period: rep.period === "week" ? t("report.week") : t("report.month"),
                  range: `${rep.start} ~ ${rep.end}`,
                })
              : t("report.titleDefault")}
          </DialogTitle>
        </DialogHeader>
        <div className="flex gap-2">
          <Button variant={period === "week" ? "default" : "secondary"} size="sm" onClick={() => setPeriod("week")}>
            {t("report.week")}
          </Button>
          <Button variant={period === "month" ? "default" : "secondary"} size="sm" onClick={() => setPeriod("month")}>
            {t("report.month")}
          </Button>
          <div className="flex-1" />
          <Button size="sm" onClick={exportImage}>
            <ImageDown /> {imgState === "clipboard" ? t("report.imageCopied") : imgState === "download" ? t("report.imageSaved") : t("report.copyImage")}
          </Button>
          <Button variant="ghost" size="sm" onClick={copyText}>
            <Copy /> {copied ? t("report.copied") : t("report.copyText")}
          </Button>
        </div>
        {rep && (
          <>
            <div className="grid grid-cols-4 gap-3 max-md:grid-cols-2">
              <Cell
                label={t("report.totalTokens")}
                value={fmtNum(rep.totals.totalTokens)}
                sub={
                  pct !== null ? (
                    <span className={cn("inline-flex items-center gap-0.5", delta >= 0 ? "text-success" : "text-destructive")}>
                      {delta >= 0 ? <TrendingUp className="size-3" /> : <TrendingDown className="size-3" />}
                      {delta >= 0 ? "+" : ""}
                      {pct}%
                    </span>
                  ) : (
                    t("report.prevZero")
                  )
                }
              />
              <Cell label={t("report.requests")} value={fmtFull(rep.totals.requests)} sub={t("report.prev", { n: fmtFull(rep.prevTotals.requests) })} />
              <Cell
                label={t("report.activeDays")}
                value={String(rep.activeDays)}
                sub={t("report.dailyAvg", { n: fmtNum(rep.activeDays > 0 ? Math.round(rep.totals.totalTokens / rep.activeDays) : 0) })}
              />
              <Cell label={t("report.outputTokens")} value={fmtNum(rep.totals.outputTokens)} sub={t("report.prev", { n: fmtNum(rep.prevTotals.outputTokens) })} />
            </div>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t("report.topModels")}</TableHead>
                  <TableHead>{t("table.requests")}</TableHead>
                  <TableHead>{t("sessions.totalTokens")}</TableHead>
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
