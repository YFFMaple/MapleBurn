import { useEffect, useState } from "react";
import { motion } from "motion/react";
import { apiGet, type Totals } from "@/lib/api";
import { daysBetween, fmtFull, fmtNum } from "@/lib/utils";
import { useApp } from "@/store";
import { useT } from "@/i18n";
import { Card, CardContent } from "@/components/ui/card";

export function StatCards() {
  const t = useT();
  const { range, refreshTick } = useApp();
  const [totals, setTotals] = useState<Totals | null>(null);

  useEffect(() => {
    apiGet<{ totals: Totals }>(`/api/range?start=${range.start}&end=${range.end}`)
      .then((r) => setTotals(r.totals))
      .catch(() => {});
  }, [range, refreshTick]);

  const days = daysBetween(range.start, range.end);
  const cards = [
    {
      label: t("cards.totalTokens"),
      value: totals ? fmtNum(totals.totalTokens) : "-",
      tip: totals
        ? `${t("detail.input")} ${fmtFull(totals.inputTokens)} + ${t("detail.output")} ${fmtFull(totals.outputTokens)} + ${t("detail.cacheCreate")} ${fmtFull(totals.cacheCreate)} + ${t("detail.cacheRead")} ${fmtFull(totals.cacheRead)}`
        : "",
    },
    { label: t("cards.requests"), value: totals ? fmtFull(totals.requests) : "-", tip: "" },
    {
      label: t("cards.dailyAvg"),
      value: totals ? fmtNum(Math.round(totals.totalTokens / days)) : "-",
      tip: totals ? `${fmtFull(Math.round(totals.totalTokens / days))} ${t("cards.tokensPerDay")}` : "",
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
