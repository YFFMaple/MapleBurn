import { useEffect, useState } from "react";
import { apiGet, type ModelStat, type Totals } from "@/lib/api";
import { fmtFull, type Range } from "@/lib/utils";
import { useApp } from "@/store";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { PctBar } from "@/components/PctBar";

interface ModelsResponse {
  totals: Totals;
  models: ModelStat[];
}

// 明细面板数据：默认跟随全局范围，点趋势图柱子后锁定到那一天
export function ModelDetail({ detailRange }: { detailRange: Range }) {
  const refreshTick = useApp((s) => s.refreshTick);
  const [data, setData] = useState<ModelsResponse | null>(null);

  useEffect(() => {
    apiGet<ModelsResponse>(`/api/models?start=${detailRange.start}&end=${detailRange.end}`)
      .then(setData)
      .catch(() => {});
  }, [detailRange, refreshTick]);

  const t = data?.totals;
  const models = data?.models || [];

  return (
    <Card>
      <CardHeader>
        <CardTitle>模型明细（{detailRange.start === detailRange.end ? detailRange.start : `${detailRange.start} ~ ${detailRange.end}`}）</CardTitle>
      </CardHeader>
      <CardContent>
        {t && (
          <div className="mb-3 flex flex-wrap gap-x-5 gap-y-1 text-xs text-muted-foreground">
            <span>请求 <b className="text-foreground">{fmtFull(t.requests)}</b></span>
            <span>输入 <b className="text-foreground">{fmtFull(t.inputTokens)}</b></span>
            <span>输出 <b className="text-foreground">{fmtFull(t.outputTokens)}</b></span>
            <span>缓存读取 <b className="text-foreground">{fmtFull(t.cacheRead)}</b></span>
            <span>缓存写入 <b className="text-foreground">{fmtFull(t.cacheCreate)}</b></span>
          </div>
        )}
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>模型</TableHead>
              <TableHead>请求数</TableHead>
              <TableHead>输入</TableHead>
              <TableHead>输出</TableHead>
              <TableHead>缓存写入</TableHead>
              <TableHead>缓存读取</TableHead>
              <TableHead className="w-[130px]">占比</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {models.map((m) => {
              const total = m.inputTokens + m.outputTokens + m.cacheCreate + m.cacheRead;
              const pct = t && t.totalTokens > 0 ? (total / t.totalTokens) * 100 : 0;
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
          <div className="py-8 text-center text-xs text-muted-foreground">该范围没有记录</div>
        )}
      </CardContent>
    </Card>
  );
}
