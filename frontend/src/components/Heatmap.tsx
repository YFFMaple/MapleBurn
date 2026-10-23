import { useEffect, useState } from "react";
import { apiGet } from "@/lib/api";
import { fmtFull } from "@/lib/utils";
import { useApp } from "@/store";
import { useT, type Key } from "@/i18n";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";

const DOWS: Key[] = ["dow.0", "dow.1", "dow.2", "dow.3", "dow.4", "dow.5", "dow.6"];

export function Heatmap() {
  const t = useT();
  const { range, refreshTick } = useApp();
  const [grid, setGrid] = useState<number[][]>([]);

  useEffect(() => {
    apiGet<number[][]>(`/api/heatmap?start=${range.start}&end=${range.end}`)
      .then((g) => setGrid(g || []))
      .catch(() => {});
  }, [range, refreshTick]);

  const max = Math.max(...grid.flat(), 1);

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("heatmap.title")}</CardTitle>
        <span className="text-xs text-muted-foreground">{t("heatmap.hint")}</span>
      </CardHeader>
      <CardContent>
        <div className="grid gap-[3px]" style={{ gridTemplateColumns: "30px repeat(24, 1fr)" }}>
          <div />
          {Array.from({ length: 24 }, (_, h) => (
            <div key={h} className="text-center text-[10px] text-muted-foreground">
              {h % 3 === 0 ? h : ""}
            </div>
          ))}
          {grid.map((row, d) => (
            <div key={d} className="contents">
              <div className="flex items-center justify-center text-xs text-muted-foreground">
                {t(DOWS[d])}
              </div>
              {row.map((v, h) => (
                <div
                  key={h}
                  className="h-5 rounded-[3px] bg-primary outline outline-1 outline-border transition-transform hover:scale-110"
                  style={{ opacity: v > 0 ? 0.12 + 0.88 * Math.pow(v / max, 0.5) : 0.04 }}
                  title={t("heatmap.cellTip", { dow: t(DOWS[d]), hour: h, n: fmtFull(v) })}
                />
              ))}
            </div>
          ))}
        </div>
      </CardContent>
    </Card>
  );
}
