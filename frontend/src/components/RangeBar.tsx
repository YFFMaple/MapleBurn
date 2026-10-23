import { useEffect, useState } from "react";
import { CalendarDays } from "lucide-react";
import { rangeFor, type Range } from "@/lib/utils";
import { useApp } from "@/store";
import { useT, type Key } from "@/i18n";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";

const PRESETS: [string, Key][] = [
  ["today", "range.today"],
  ["yesterday", "range.yesterday"],
  ["7d", "range.7d"],
  ["30d", "range.30d"],
  ["week", "range.week"],
  ["month", "range.month"],
  ["all", "range.all"],
];

export function RangeBar() {
  const t = useT();
  const { range, preset, setRange } = useApp();
  const [custom, setCustom] = useState<Range>(range);
  useEffect(() => setCustom(range), [range]);

  return (
    <div className="flex flex-wrap items-center gap-2">
      <div className="flex items-center gap-1 rounded-lg bg-muted p-1">
        {PRESETS.map(([key, label]) => (
          <button
            key={key}
            onClick={() => setRange(rangeFor(key), key)}
            className={cn(
              "rounded-md px-3 py-1.5 text-xs text-muted-foreground transition-colors cursor-pointer hover:text-foreground",
              preset === key && "bg-background text-foreground shadow-sm"
            )}
          >
            {t(label)}
          </button>
        ))}
      </div>
      <div className="flex items-center gap-1.5 text-xs text-muted-foreground">
        <CalendarDays className="size-3.5" />
        <Input
          type="date"
          className="h-8 w-35 text-xs"
          value={custom.start}
          onChange={(e) => setCustom({ ...custom, start: e.target.value })}
        />
        <span>~</span>
        <Input
          type="date"
          className="h-8 w-35 text-xs"
          value={custom.end}
          onChange={(e) => setCustom({ ...custom, end: e.target.value })}
        />
        <Button
          variant="secondary"
          size="sm"
          disabled={!custom.start || !custom.end || custom.start > custom.end}
          onClick={() => setRange(custom, null)}
        >
          {t("range.apply")}
        </Button>
      </div>
      <span className="text-xs text-muted-foreground">
        {t("range.current")}{range.start} ~ {range.end}
      </span>
    </div>
  );
}
