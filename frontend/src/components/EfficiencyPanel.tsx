import { useEffect, useState } from "react";
import { apiGet, type Efficiency } from "@/lib/api";
import { fmtFull, fmtNum } from "@/lib/utils";
import { useApp } from "@/store";
import { useT } from "@/i18n";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { PctBar } from "@/components/PctBar";

// 效率分析：缓存命中率越高，实际计费输入越少
export function EfficiencyPanel() {
  const t = useT();
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
        <CardTitle>{t("efficiency.title")}</CardTitle>
        <span className="text-xs text-muted-foreground">
          {t("efficiency.summary", { hit: pct(e.cacheHitRate), avg: fmtNum(e.avgPerRequest) })}
        </span>
      </CardHeader>
      <CardContent>
        <div className="mb-3">
          <div className="mb-1 flex justify-between text-xs text-muted-foreground">
            <span>{t("efficiency.overall")}</span>
            <span className="font-semibold text-foreground">{pct(e.cacheHitRate)}%</span>
          </div>
          <PctBar pct={e.cacheHitRate * 100} />
        </div>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t("table.model")}</TableHead>
              <TableHead>{t("table.requests")}</TableHead>
              <TableHead>{t("efficiency.avgOutput")}</TableHead>
              <TableHead>{t("efficiency.avgTotal")}</TableHead>
              <TableHead className="w-[160px]">{t("efficiency.hitRate")}</TableHead>
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
          <div className="py-6 text-center text-xs text-muted-foreground">{t("efficiency.empty")}</div>
        )}
      </CardContent>
    </Card>
  );
}
