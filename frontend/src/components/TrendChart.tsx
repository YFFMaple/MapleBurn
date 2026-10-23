import { useEffect, useState } from "react";
import { apiGet, type DayStat } from "@/lib/api";
import { addDays, daysBetween, fmtFull } from "@/lib/utils";
import { useApp } from "@/store";
import { useT } from "@/i18n";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { cn } from "@/lib/utils";

export function TrendChart({ onPickDate }: { onPickDate: (date: string) => void }) {
  const t = useT();
  const { range, refreshTick } = useApp();
  const [series, setSeries] = useState<DayStat[]>([]);
  const [selected, setSelected] = useState<string | null>(null);

  useEffect(() => {
    let { start, end } = range;
    // 范围太长时最多显示最近 92 天，避免柱子挤成一片
    if (daysBetween(start, end) > 92) start = addDays(end, -91);
    apiGet<DayStat[]>(`/api/series?start=${start}&end=${end}`)
      .then((s) => setSeries(s || []))
      .catch(() => {});
  }, [range, refreshTick]);

  const max = Math.max(...series.map((s) => s.totalTokens), 1);

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("trend.title", { range: `${range.start} ~ ${range.end}` })}</CardTitle>
        <span className="text-xs text-muted-foreground">{t("trend.hint")}</span>
      </CardHeader>
      <CardContent>
        <div className="flex h-[130px] items-end gap-[3px]">
          {series.map((s) => (
            <div
              key={s.date}
              className="group relative flex h-full flex-1 cursor-pointer flex-col items-center justify-end"
              onClick={() => {
                setSelected(s.date);
                onPickDate(s.date);
              }}
            >
              <div className="pointer-events-none absolute -top-1 z-10 hidden -translate-y-full rounded-md border bg-popover px-2.5 py-1.5 text-xs whitespace-nowrap shadow-lg group-hover:block">
                {s.date}
                <br />
                {fmtFull(s.totalTokens)} {t("trend.tokens")}
              </div>
              <div
                className={cn(
                  "w-[70%] min-h-[2px] rounded-t-sm bg-chart-bar opacity-70 transition-opacity group-hover:opacity-100",
                  selected === s.date && "opacity-100 shadow-[0_0_10px_var(--color-chart-bar)]"
                )}
                style={{ height: `${Math.max((s.totalTokens / max) * 100, 1.5)}%` }}
              />
              <div className="mt-1 text-[10px] text-muted-foreground">{s.date.slice(5)}</div>
            </div>
          ))}
        </div>
      </CardContent>
    </Card>
  );
}
