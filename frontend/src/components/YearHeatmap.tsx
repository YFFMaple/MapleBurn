import { useEffect, useMemo, useState } from "react";
import { apiGet, type DayStat } from "@/lib/api";
import { addDays, fmtFull, fmtNum, todayStr } from "@/lib/utils";
import { useApp } from "@/store";
import { useT } from "@/i18n";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { cn } from "@/lib/utils";

// 年度热力图：GitHub 贡献图风格，365 天逐日用量
export function YearHeatmap() {
  const t = useT();
  const refreshTick = useApp((s) => s.refreshTick);
  const [series, setSeries] = useState<DayStat[]>([]);

  useEffect(() => {
    const end = todayStr();
    const start = addDays(end, -364);
    apiGet<DayStat[]>(`/api/series?start=${start}&end=${end}`)
      .then((s) => setSeries(s || []))
      .catch(() => {});
  }, [refreshTick]);

  const { weeks, max, total } = useMemo(() => {
    const byDate = new Map(series.map((s) => [s.date, s.totalTokens]));
    const max = Math.max(...series.map((s) => s.totalTokens), 1);
    const total = series.reduce((a, s) => a + s.totalTokens, 0);
    // 按周分列：每列 7 天（周一开头），从今天往前推满整周
    const end = new Date(todayStr() + "T00:00:00");
    const start = new Date(end);
    start.setDate(start.getDate() - 364 - ((start.getDay() + 6) % 7));
    const weeks: { date: string; value: number }[][] = [];
    let cur: { date: string; value: number }[] = [];
    for (let d = new Date(start); d <= end; d.setDate(d.getDate() + 1)) {
      const ds = `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;
      cur.push({ date: ds, value: byDate.get(ds) ?? 0 });
      if (cur.length === 7) {
        weeks.push(cur);
        cur = [];
      }
    }
    if (cur.length) weeks.push(cur);
    return { weeks, max, total };
  }, [series]);

  const level = (v: number) => {
    if (v <= 0) return 0;
    const r = v / max;
    if (r > 0.75) return 4;
    if (r > 0.5) return 3;
    if (r > 0.25) return 2;
    return 1;
  };
  const colors = [
    "bg-muted",
    "bg-primary/25",
    "bg-primary/45",
    "bg-primary/70",
    "bg-primary",
  ];

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("yearmap.title")}</CardTitle>
        <span className="text-xs text-muted-foreground">
          {t("yearmap.hint", { total: fmtNum(total) })}
        </span>
      </CardHeader>
      <CardContent className="overflow-x-auto">
        <div className="flex gap-[3px]">
          {weeks.map((w, i) => (
            <div key={i} className="flex flex-col gap-[3px]">
              {w.map((d) => (
                <div
                  key={d.date}
                  className={cn("size-[11px] rounded-[2px]", colors[level(d.value)])}
                  title={`${d.date} · ${fmtFull(d.value)} tokens`}
                />
              ))}
            </div>
          ))}
        </div>
        <div className="mt-2 flex items-center justify-end gap-1 text-[10px] text-muted-foreground">
          {t("yearmap.less")}
          {colors.map((c, i) => (
            <div key={i} className={cn("size-[11px] rounded-[2px]", c)} />
          ))}
          {t("yearmap.more")}
        </div>
      </CardContent>
    </Card>
  );
}
