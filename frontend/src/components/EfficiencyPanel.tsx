import { useEffect, useState } from "react";
import { apiGet, type Efficiency } from "@/lib/api";
import { fmtFull, fmtNum } from "@/lib/utils";
import { useApp } from "@/store";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { PctBar } from "@/components/PctBar";

// 效率分析：缓存命中率越高，实际计费输入越少
export function EfficiencyPanel() {
  const { range, refreshTick } = useApp();
  const [e, setE] = useState<Efficiency | null>(null);

  useEffect(() => {
    apiGet<Efficiency>(`/api/efficiency?start=${range.start}&end=${range.end}`)
      .then(setE)
      .catch(() => {});
  }, [range, refreshTick]);

  if (!e) return null;
  const pct = (v: number) => (v * 100).toFixed(1);

  return (
    <Card>
      <CardHeader>
        <CardTitle>效率分析</CardTitle>
        <span className="text-xs text-muted-foreground">
          缓存命中率 {pct(e.cacheHitRate)}% · 平均每请求 {fmtNum(e.avgPerRequest)} tokens
        </span>
      </CardHeader>
      <CardContent>
        <div className="mb-3">
          <div className="mb-1 flex justify-between text-xs text-muted-foreground">
            <span>整体缓存命中率（缓存读 ÷ 全部输入侧 token）</span>
            <span className="font-semibold text-foreground">{pct(e.cacheHitRate)}%</span>
          </div>
          <PctBar pct={e.cacheHitRate * 100} />
        </div>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>模型</TableHead>
              <TableHead>请求数</TableHead>
              <TableHead>平均输出</TableHead>
              <TableHead>平均每请求总量</TableHead>
              <TableHead className="w-[160px]">缓存命中率</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {e.models.map((m) => (
              <TableRow key={m.model}>
                <TableCell className="font-mono text-xs">{m.model}</TableCell>
                <TableCell>{fmtFull(m.requests)}</TableCell>
                <TableCell>{fmtNum(m.avgOutput)}</TableCell>
                <TableCell>{fmtNum(m.avgTotal)}</TableCell>
                <TableCell>
                  <PctBar pct={m.cacheHitRate * 100} label={`${pct(m.cacheHitRate)}%`} />
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
        {e.models.length === 0 && (
          <div className="py-6 text-center text-xs text-muted-foreground">该范围没有记录</div>
        )}
      </CardContent>
    </Card>
  );
}
