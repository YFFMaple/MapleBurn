import { useCallback, useEffect, useState } from "react";
import { motion } from "motion/react";
import { Pin, PinOff, ExternalLink, Minus, X, Droplets } from "lucide-react";
import { apiGet, type Totals } from "@/lib/api";
import { fmtFull, fmtNum } from "@/lib/utils";
import { useT, type Key } from "@/i18n";
import { MapleLeaf } from "@/components/MapleLeaf";

type Period = "today" | "week" | "month";

interface WidgetBar {
  label: string;
  value: number;
  future?: boolean;
}

interface WidgetData {
  period: Period;
  totals: Totals;
  cacheHit: number;
  bars: WidgetBar[];
  monthTokens: number;
  proxyOn: boolean;
  proxyRun: boolean;
}

// webview 宿主注入的绑定（浏览器里直接打开时没有，对应按钮自动隐藏）
declare global {
  interface Window {
    widgetSetTopmost?: (top: boolean) => Promise<void>;
    widgetGetTopmost?: () => Promise<boolean>;
    widgetSetGlass?: (on: boolean) => Promise<void>;
    widgetGetGlass?: () => Promise<boolean>;
    widgetDrag?: () => Promise<void>;
    widgetMinimize?: () => Promise<void>;
    widgetClose?: () => Promise<void>;
  }
}

const inWebview = typeof window.widgetDrag === "function";

const PERIODS: Period[] = ["today", "week", "month"];
const PERIOD_KEYS: Record<Period, Key> = {
  today: "widget.period.today",
  week: "widget.period.week",
  month: "widget.period.month",
};
const BARS_KEYS: Record<Period, Key> = {
  today: "widget.bars.today",
  week: "widget.bars.week",
  month: "widget.bars.month",
};
const DOW_KEYS: Key[] = ["dow.0", "dow.1", "dow.2", "dow.3", "dow.4", "dow.5", "dow.6"];

// 桌面小组件：无边框常驻小窗，5 秒轮询一次聚合数据
export function WidgetApp() {
  const t = useT();
  const [data, setData] = useState<WidgetData | null>(null);
  const [top, setTop] = useState(false);
  const [glass, setGlass] = useState(false);
  const [period, setPeriod] = useState<Period>(() => {
    const p = localStorage.getItem("widgetPeriod");
    return p === "week" || p === "month" ? p : "today";
  });

  const load = useCallback(
    () => apiGet<WidgetData>(`/api/widget?period=${period}`).then(setData).catch(() => {}),
    [period]
  );

  useEffect(() => {
    load();
    const timer = setInterval(() => {
      if (!document.hidden) load();
    }, 5000);
    return () => clearInterval(timer);
  }, [load]);

  useEffect(() => {
    window.widgetGetTopmost?.().then(setTop).catch(() => {});
    window.widgetGetGlass?.().then(setGlass).catch(() => {});
  }, []);

  const toggleTop = () => {
    const v = !top;
    setTop(v);
    window.widgetSetTopmost?.(v).catch(() => {});
  };

  const toggleGlass = () => {
    const v = !glass;
    setGlass(v);
    document.documentElement.classList.toggle("glass", v);
    window.widgetSetGlass?.(v).catch(() => {});
  };

  const pickPeriod = (p: Period) => {
    setPeriod(p);
    localStorage.setItem("widgetPeriod", p);
  };

  // 自绘标题栏拖拽：按住空白处让系统接管移动；点在按钮上不触发
  const onDragStart = (e: React.MouseEvent) => {
    if ((e.target as HTMLElement).closest("button, a")) return;
    window.widgetDrag?.().catch(() => {});
  };

  const bars = data?.bars ?? [];
  const max = Math.max(...bars.map((b) => b.value), 1);
  // 刻度太多会挤，最多放 5~6 个
  const tickEvery = Math.max(1, Math.ceil(bars.length / 5));
  const barLabel = (b: WidgetBar): string => {
    if (period === "today") return `${b.label}:00`;
    if (period === "week") {
      const d = new Date(`${new Date().getFullYear()}-${b.label}T00:00:00`);
      return `${b.label} ${t(DOW_KEYS[(d.getDay() + 6) % 7])}`;
    }
    return b.label;
  };

  return (
    <div className="glass-panel flex h-screen flex-col overflow-hidden rounded-xl border border-border/60 bg-background text-foreground select-none">
      {/* 自绘标题栏（拖拽区） */}
      <div
        className="flex shrink-0 cursor-move items-center gap-2 border-b border-border/50 bg-muted/40 px-3 py-2"
        onMouseDown={onDragStart}
      >
        <div className="flex size-5 items-center justify-center rounded-md bg-gradient-to-br from-[#ef6a4a] to-[#d43a2f] text-white">
          <MapleLeaf className="size-3" />
        </div>
        <span className="text-xs font-semibold">MapleBurn</span>
        <span className="text-[10px] text-muted-foreground">{new Date().toLocaleDateString()}</span>
        <div className="flex-1" />
        <TbBtn title={top ? t("widget.unpin") : t("widget.pin")} onClick={toggleTop}>
          {top ? <Pin className="size-3 text-primary" /> : <PinOff className="size-3" />}
        </TbBtn>
        <TbBtn title={t("widget.glass")} onClick={toggleGlass}>
          <Droplets className={`size-3 ${glass ? "text-primary" : ""}`} />
        </TbBtn>
        <a
          href="./index.html"
          target="_blank"
          title={t("widget.openFull")}
          className="rounded p-1 text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
        >
          <ExternalLink className="size-3" />
        </a>
        {inWebview && (
          <>
            <TbBtn title={t("widget.minimize")} onClick={() => window.widgetMinimize?.()}>
              <Minus className="size-3" />
            </TbBtn>
            <TbBtn
              title={t("widget.close")}
              onClick={() => window.widgetClose?.()}
              className="hover:bg-destructive/20 hover:text-destructive"
            >
              <X className="size-3" />
            </TbBtn>
          </>
        )}
      </div>

      <div className="flex min-h-0 flex-1 flex-col gap-2.5 p-3">
        {/* 周期切换 */}
        <div className="flex shrink-0 gap-1 rounded-lg border border-border/50 bg-muted/40 p-0.5">
          {PERIODS.map((p) => (
            <button
              key={p}
              onClick={() => pickPeriod(p)}
              className={`flex-1 cursor-pointer rounded-md py-1 text-[11px] transition-colors ${
                period === p
                  ? "bg-primary font-medium text-primary-foreground"
                  : "text-muted-foreground hover:text-foreground"
              }`}
            >
              {t(PERIOD_KEYS[p])}
            </button>
          ))}
        </div>

        {/* 总量 */}
        <motion.div
          className="shrink-0 rounded-lg border border-border/50 bg-card p-3"
          initial={{ opacity: 0, y: 6 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.25 }}
        >
          <div className="flex items-baseline justify-between">
            <span className="text-[11px] text-muted-foreground">{t(PERIOD_KEYS[period])} tokens</span>
            <span className="text-[11px] text-muted-foreground">
              {t("table.requests")} <b className="text-foreground">{data ? fmtFull(data.totals.requests) : "-"}</b>
            </span>
          </div>
          <div
            className="mt-0.5 bg-gradient-to-r from-[#ef6a4a] to-[#f2a154] bg-clip-text text-[34px] leading-tight font-bold tabular-nums text-transparent"
            title={data ? fmtFull(data.totals.totalTokens) : ""}
          >
            {data ? fmtNum(data.totals.totalTokens) : "-"}
          </div>
          <div className="mt-1 flex items-center gap-2 text-[11px] text-muted-foreground">
            <span>
              {t("detail.output")} <b className="text-foreground">{data ? fmtNum(data.totals.outputTokens) : "-"}</b>
            </span>
            <span className="text-border">|</span>
            <span>
              {t("widget.cacheHit")} <b className="text-foreground">{data ? (data.cacheHit * 100).toFixed(1) + "%" : "-"}</b>
            </span>
          </div>
          <div className="mt-1.5 h-1 overflow-hidden rounded-full bg-muted">
            <div
              className="h-full rounded-full bg-gradient-to-r from-[#d43a2f] to-[#f2a154] transition-all duration-500"
              style={{ width: `${(data?.cacheHit ?? 0) * 100}%` }}
            />
          </div>
        </motion.div>

        {/* 柱状图：吃满剩余高度 */}
        <motion.div
          className="flex min-h-0 flex-1 flex-col rounded-lg border border-border/50 bg-card p-3"
          initial={{ opacity: 0, y: 6 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.25, delay: 0.05 }}
        >
          <div className="mb-1.5 text-[11px] text-muted-foreground">{t(BARS_KEYS[period])}</div>
          <div className="flex min-h-0 flex-1 items-end gap-[3px]">
            {bars.map((b, i) => (
              <div
                key={i}
                className="flex-1 rounded-t-[3px] bg-gradient-to-t from-[#d43a2f]/80 to-[#f2a154] transition-opacity"
                style={{
                  height: `${Math.max((b.value / max) * 100, 2)}%`,
                  opacity: b.future ? 0.12 : b.value > 0 ? 0.4 + 0.6 * (b.value / max) : 0.1,
                }}
                title={`${barLabel(b)} · ${fmtFull(b.value)}`}
              />
            ))}
          </div>
          <div className="mt-1 flex justify-between text-[9px] text-muted-foreground">
            {bars.map((b, i) =>
              i % tickEvery === 0 || i === bars.length - 1 ? (
                <span key={i} className="flex-1 truncate text-center first:text-left last:text-right">
                  {period === "today" ? b.label : b.label.slice(3)}
                </span>
              ) : (
                <span key={i} className="flex-1" />
              )
            )}
          </div>
        </motion.div>

        {/* 本月 + 代理状态 */}
        <motion.div
          className="flex shrink-0 items-center justify-between rounded-lg border border-border/50 bg-card px-3 py-2 text-[11px]"
          initial={{ opacity: 0, y: 6 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.25, delay: 0.1 }}
        >
          <span className="text-muted-foreground">
            {t("widget.monthTotal")} <b className="text-foreground">{data ? fmtNum(data.monthTokens) : "-"}</b>
          </span>
          <span className="flex items-center gap-1.5 text-muted-foreground">
            <span className={`size-1.5 rounded-full ${data?.proxyRun ? "bg-success" : "bg-muted-foreground/40"}`} />
            {data?.proxyOn ? t("widget.proxyOn") : t("widget.proxyOff")}
          </span>
        </motion.div>
      </div>
    </div>
  );
}

function TbBtn({
  title,
  onClick,
  className = "",
  children,
}: {
  title: string;
  onClick: () => void;
  className?: string;
  children: React.ReactNode;
}) {
  return (
    <button
      title={title}
      onClick={onClick}
      className={`cursor-pointer rounded p-1 text-muted-foreground transition-colors hover:bg-muted hover:text-foreground ${className}`}
    >
      {children}
    </button>
  );
}
