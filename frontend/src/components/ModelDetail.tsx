import { useEffect, useState } from "react";
import { apiGet, type ModelStat, type Totals } from "@/lib/api";
import { fmtFull, type Range } from "@/lib/utils";
import { useApp } from "@/store";
import { useT } from "@/i18n";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { PctBar } from "@/components/PctBar";

interface ModelsResponse {
  totals: Totals;
  models: ModelStat[];
}

// 明细面板数据：默认跟随全局范围，点趋势图柱子后锁定到那一天
export function ModelDetail({ detailRange }: { detailRange: Range }) {
  const t = useT();
  const refreshTick = useApp((s) => s.refreshTick);
  const [data, setData] = useState<ModelsResponse | null>(null);

  useEffect(() => {
    apiGet<ModelsResponse>(`/api/models?start=${detailRange.start}&end=${detailRange.end}`)
      .then(setData)
      .catch(() => {});
  }, [detailRange, refreshTick]);

  const totals = data?.totals;
  const models = data?.models || [];
  const rangeLabel =
    detailRange.start === detailRange.end ? detailRange.start : `${detailRange.start} ~ ${detailRange.end}`;

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("detail.title", { range: rangeLabel })}</CardTitle>
      </CardHeader>
      <CardContent>
        {totals && (
          <div className="mb-3 flex flex-wrap gap-x-5 gap-y-1 text-xs text-muted-foreground">
            <span>{t("detail.requests")} <b className="text-foreground">{fmtFull(totals.requests)}</b></span>
            <span>{t("detail.input")} <b className="text-foreground">{fmtFull(totals.inputTokens)}</b></span>
            <span>{t("detail.output")} <b className="text-foreground">{fmtFull(totals.outputTokens)}</b></span>
            <span>{t("detail.cacheRead")} <b className="text-foreground">{fmtFull(totals.cacheRead)}</b></span>
            <span>{t("detail.cacheCreate")} <b className="text-foreground">{fmtFull(totals.cacheCreate)}</b></span>
          </div>
        )}
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t("table.model")}</TableHead>
              <TableHead>{t("table.requests")}</TableHead>
              <TableHead>{t("table.input")}</TableHead>
              <TableHead>{t("table.output")}</TableHead>
              <TableHead>{t("table.cacheCreate")}</TableHead>
              <TableHead>{t("table.cacheRead")}</TableHead>
              <TableHead className="w-[130px]">{t("detail.share")}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {models.map((m) => {
              const total = m.inputTokens + m.outputTokens + m.cacheCreate + m.cacheRead;
              const pct = totals && totals.totalTokens > 0 ? (total / totals.totalTokens) * 100 : 0;
              return (
                <TableRow key={m.model}>
                  <TableCell className="font-mono text-xs">{m.model}</TableCell>
                  <TableCell>{fmtFull(m.requests)}</TableCell>
                  <TableCell>{fmtFull(m.inputTokens)}</TableCell>
                  <TableCell>{fmtFull(m.outputTokens)}</TableCell>
                  <TableCell>{fmtFull(m.cacheCreate)}</TableCell>
                  <TableCell>{fmtFull(m.cacheRead)}</TableCell>
                  <TableCell>
                    <PctBar pct={pct} label={`${pct.toFixed(1)}%`} />
                  </TableCell>
                </TableRow>
              );
            })}
          </TableBody>
        </Table>
        {models.length === 0 && (
          <div className="py-8 text-center text-xs text-muted-foreground">{t("common.empty")}</div>
        )}
      </CardContent>
    </Card>
  );
}
