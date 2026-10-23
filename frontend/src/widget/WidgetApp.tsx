import { useEffect, useState } from "react";
import { motion } from "motion/react";
import { Pin, PinOff, ExternalLink, Minus, X } from "lucide-react";
import { apiGet, type Totals } from "@/lib/api";
import { fmtFull, fmtNum } from "@/lib/utils";
import { useT } from "@/i18n";
import { MapleLeaf } from "@/components/MapleLeaf";

interface WidgetSummary {
  today: Totals;
  week: Totals;
  cacheHit: number;
  hourly: number[];
  proxyOn: boolean;
  proxyRun: boolean;
}

// webview 宿主注入的绑定（浏览器里直接打开时没有，对应按钮自动隐藏）
declare global {
  interface Window {
    widgetSetTopmost?: (top: boolean) => Promise<void>;
    widgetGetTopmost?: () => Promise<boolean>;
    widgetDrag?: () => Promise<void>;
    widgetMinimize?: () => Promise<void>;
    widgetClose?: () => Promise<void>;
  }
}

const inWebview = typeof window.widgetDrag === "function";

// 桌面小组件：无边框常驻小窗，5 秒轮询一次聚合数据
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

  // 自绘标题栏拖拽：按住空白处让系统接管移动；点在按钮上不触发
  const onDragStart = (e: React.MouseEvent) => {
    if ((e.target as HTMLElement).closest("button, a")) return;
    window.widgetDrag?.().catch(() => {});
  };

  const today = sum?.today;
  const max = Math.max(...(sum?.hourly ?? [1]), 1);
  const nowHour = new Date().getHours();

  return (
    <div className="flex h-screen flex-col overflow-hidden rounded-xl border border-border/60 bg-background text-foreground select-none">
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
        {/* 今日总量 */}
        <motion.div
          className="rounded-lg border border-border/50 bg-card p-3"
          initial={{ opacity: 0, y: 6 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.25 }}
        >
          <div className="flex items-baseline justify-between">
            <span className="text-[11px] text-muted-foreground">{t("widget.today")}</span>
            <span className="text-[11px] text-muted-foreground">
              {t("table.requests")} <b className="text-foreground">{today ? fmtFull(today.requests) : "-"}</b>
            </span>
          </div>
          <div
            className="mt-0.5 bg-gradient-to-r from-[#ef6a4a] to-[#f2a154] bg-clip-text text-[34px] leading-tight font-bold tabular-nums text-transparent"
            title={today ? fmtFull(today.totalTokens) : ""}
          >
            {today ? fmtNum(today.totalTokens) : "-"}
          </div>
          <div className="mt-1 flex items-center gap-2 text-[11px] text-muted-foreground">
            <span>
              {t("detail.output")} <b className="text-foreground">{today ? fmtNum(today.outputTokens) : "-"}</b>
            </span>
            <span className="text-border">|</span>
            <span>
              {t("widget.cacheHit")} <b className="text-foreground">{sum ? (sum.cacheHit * 100).toFixed(1) + "%" : "-"}</b>
            </span>
          </div>
          <div className="mt-1.5 h-1 overflow-hidden rounded-full bg-muted">
            <div
              className="h-full rounded-full bg-gradient-to-r from-[#d43a2f] to-[#f2a154] transition-all duration-500"
              style={{ width: `${(sum?.cacheHit ?? 0) * 100}%` }}
            />
          </div>
        </motion.div>

        {/* 今日逐小时：吃满剩余高度 */}
        <motion.div
          className="flex min-h-0 flex-1 flex-col rounded-lg border border-border/50 bg-card p-3"
          initial={{ opacity: 0, y: 6 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.25, delay: 0.05 }}
        >
          <div className="mb-1.5 text-[11px] text-muted-foreground">{t("widget.hourly")}</div>
          <div className="flex min-h-0 flex-1 items-end gap-[3px]">
            {(sum?.hourly ?? Array(24).fill(0)).map((v, h) => (
              <div
                key={h}
                className="flex-1 rounded-t-[3px] bg-gradient-to-t from-[#d43a2f]/80 to-[#f2a154] transition-opacity"
                style={{
                  height: `${Math.max((v / max) * 100, 2)}%`,
                  opacity: h > nowHour ? 0.12 : v > 0 ? 0.4 + 0.6 * (v / max) : 0.1,
                }}
                title={`${h}:00 · ${fmtFull(v)}`}
              />
            ))}
          </div>
          <div className="mt-1 flex justify-between text-[9px] text-muted-foreground">
            <span>0</span>
            <span>6</span>
            <span>12</span>
            <span>18</span>
            <span>24</span>
          </div>
        </motion.div>

        {/* 本周 + 代理状态 */}
        <motion.div
          className="flex shrink-0 items-center justify-between rounded-lg border border-border/50 bg-card px-3 py-2 text-[11px]"
          initial={{ opacity: 0, y: 6 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.25, delay: 0.1 }}
        >
          <span className="text-muted-foreground">
            {t("widget.week")} <b className="text-foreground">{sum ? fmtNum(sum.week.totalTokens) : "-"}</b>
          </span>
          <span className="flex items-center gap-1.5 text-muted-foreground">
            <span className={`size-1.5 rounded-full ${sum?.proxyRun ? "bg-success" : "bg-muted-foreground/40"}`} />
            {sum?.proxyOn ? t("widget.proxyOn") : t("widget.proxyOff")}
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
