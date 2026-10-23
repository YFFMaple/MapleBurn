import { useEffect, useState } from "react";
import { motion } from "motion/react";
import { apiGet, type Totals } from "@/lib/api";
import { daysBetween, fmtFull, fmtNum } from "@/lib/utils";
import { useApp } from "@/store";
import { Card, CardContent } from "@/components/ui/card";

export function StatCards() {
  const { range, refreshTick } = useApp();
  const [t, setT] = useState<Totals | null>(null);

  useEffect(() => {
    apiGet<{ totals: Totals }>(`/api/range?start=${range.start}&end=${range.end}`)
      .then((r) => setT(r.totals))
      .catch(() => {});
  }, [range, refreshTick]);

  const days = daysBetween(range.start, range.end);
  const cards = [
    {
      label: "总 Tokens",
      value: t ? fmtNum(t.totalTokens) : "-",
      tip: t
        ? `输入 ${fmtFull(t.inputTokens)} + 输出 ${fmtFull(t.outputTokens)} + 缓存写 ${fmtFull(t.cacheCreate)} + 缓存读 ${fmtFull(t.cacheRead)}`
        : "",
    },
    { label: "请求次数", value: t ? fmtFull(t.requests) : "-", tip: "" },
    {
      label: "日均 Tokens",
      value: t ? fmtNum(Math.round(t.totalTokens / days)) : "-",
      tip: t ? `${fmtFull(Math.round(t.totalTokens / days))} tokens/天` : "",
    },
  ];

  return (
    <div className="grid grid-cols-3 gap-4 max-md:grid-cols-1">
      {cards.map((c, i) => (
        <motion.div
          key={c.label}
          initial={{ opacity: 0, y: 8 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: i * 0.05, duration: 0.25 }}
        >
          <Card>
            <CardContent className="p-4">
              <div className="text-xs text-muted-foreground">{c.label}</div>
              <div className="mt-1 text-2xl font-semibold tabular-nums" title={c.tip}>
                {c.value}
              </div>
              <div className="mt-0.5 text-xs text-muted-foreground">
                {range.start} ~ {range.end}
              </div>
            </CardContent>
          </Card>
        </motion.div>
      ))}
    </div>
  );
}
