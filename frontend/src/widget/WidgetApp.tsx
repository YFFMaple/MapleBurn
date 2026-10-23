import { useEffect, useState } from "react";
import { Pin, PinOff, ExternalLink } from "lucide-react";
import { apiGet, type Totals } from "@/lib/api";
import { fmtFull, fmtNum } from "@/lib/utils";
import { useT } from "@/i18n";
import { MapleLeaf } from "@/components/MapleLeaf";
import { PctBar } from "@/components/PctBar";

interface WidgetSummary {
  today: Totals;
  week: Totals;
  cacheHit: number;
  hourly: number[];
  proxyOn: boolean;
  proxyRun: boolean;
}

// webview 宿主注入的绑定（浏览器里直接打开时没有，按钮自动隐藏）
declare global {
  interface Window {
    widgetSetTopmost?: (top: boolean) => Promise<void>;
    widgetGetTopmost?: () => Promise<boolean>;
  }
}

// 桌面小组件：320px 宽的常驻小窗，5 秒轮询一次聚合数据
export function WidgetApp() {
  const t = useT();
  const [sum, setSum] = useState<WidgetSummary | null>(null);
  const [top, setTop] = useState(false);

  useEffect(() => {
    const load = () => apiGet<WidgetSummary>("/api/widget").then(setSum).catch(() => {});
    load();
    const timer = setInterval(() => {
      if (!document.hidden) load();
    }, 5000);
    return () => clearInterval(timer);
  }, []);

  useEffect(() => {
    window.widgetGetTopmost?.().then(setTop).catch(() => {});
  }, []);

  const toggleTop = () => {
    const v = !top;
    setTop(v);
    window.widgetSetTopmost?.(v).catch(() => {});
  };

  const today = sum?.today;
  const max = Math.max(...(sum?.hourly ?? [1]), 1);
  const nowHour = new Date().getHours();

  return (
    <div className="flex h-screen flex-col gap-3 overflow-hidden bg-background p-4 text-foreground select-none">
      {/* 标题行 */}
      <div className="flex items-center gap-2">
        <div className="flex size-6 items-center justify-center rounded-md bg-gradient-to-br from-[#ef6a4a] to-[#d43a2f] text-white">
          <MapleLeaf className="size-3.5" />
        </div>
        <span className="text-sm font-semibold">MapleBurn</span>
        <span className="text-xs text-muted-foreground">{new Date().toLocaleDateString()}</span>
        <div className="flex-1" />
        {window.widgetSetTopmost && (
          <button
            className="rounded p-1 text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
            title={top ? t("widget.unpin") : t("widget.pin")}
            onClick={toggleTop}
          >
            {top ? <Pin className="size-3.5 text-primary" /> : <PinOff className="size-3.5" />}
          </button>
        )}
        <a
          href="./index.html"
          target="_blank"
          className="rounded p-1 text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
          title={t("widget.openFull")}
        >
          <ExternalLink className="size-3.5" />
        </a>
      </div>

      {/* 今日总量 */}
      <div className="rounded-xl border bg-card p-3">
        <div className="text-xs text-muted-foreground">{t("widget.today")}</div>
        <div className="mt-0.5 text-3xl font-semibold tabular-nums" title={today ? fmtFull(today.totalTokens) : ""}>
          {today ? fmtNum(today.totalTokens) : "-"}
        </div>
        <div className="mt-1 flex gap-3 text-xs text-muted-foreground">
          <span>
            {t("table.requests")} <b className="text-foreground">{today ? fmtFull(today.requests) : "-"}</b>
          </span>
          <span>
            {t("detail.output")} <b className="text-foreground">{today ? fmtNum(today.outputTokens) : "-"}</b>
          </span>
        </div>
      </div>

      {/* 缓存命中 */}
      <div className="rounded-xl border bg-card p-3">
        <div className="mb-1.5 flex justify-between text-xs">
          <span className="text-muted-foreground">{t("widget.cacheHit")}</span>
          <span className="font-semibold">{sum ? (sum.cacheHit * 100).toFixed(1) + "%" : "-"}</span>
        </div>
        <PctBar pct={(sum?.cacheHit ?? 0) * 100} />
      </div>

      {/* 今日逐小时 */}
      <div className="flex-1 rounded-xl border bg-card p-3">
        <div className="mb-2 text-xs text-muted-foreground">{t("widget.hourly")}</div>
        <div className="flex h-24 items-end gap-[2px]">
          {(sum?.hourly ?? Array(24).fill(0)).map((v, h) => (
            <div
              key={h}
              className="flex-1 rounded-t-[2px] bg-primary transition-opacity"
              style={{
                height: `${Math.max((v / max) * 100, 2)}%`,
                opacity: h > nowHour ? 0.15 : v > 0 ? 0.35 + 0.65 * (v / max) : 0.12,
              }}
              title={`${h}:00 · ${fmtFull(v)}`}
            />
          ))}
        </div>
        <div className="mt-1 flex justify-between text-[10px] text-muted-foreground">
          <span>0</span>
          <span>6</span>
          <span>12</span>
          <span>18</span>
          <span>24</span>
        </div>
      </div>

      {/* 本周 + 代理状态 */}
      <div className="flex items-center justify-between rounded-xl border bg-card px-3 py-2.5 text-xs">
        <span className="text-muted-foreground">
          {t("widget.week")} <b className="text-foreground">{sum ? fmtNum(sum.week.totalTokens) : "-"}</b>
        </span>
        <span className="flex items-center gap-1.5">
          <span
            className={`size-1.5 rounded-full ${sum?.proxyRun ? "bg-success" : "bg-muted-foreground/40"}`}
          />
          {sum?.proxyOn ? t("widget.proxyOn") : t("widget.proxyOff")}
        </span>
      </div>
    </div>
  );
}
