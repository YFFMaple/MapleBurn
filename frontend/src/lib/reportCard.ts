import type { Report } from "@/lib/api";
import { fmtFull, fmtNum } from "@/lib/utils";
import type { TFunc } from "@/i18n";

// 枫叶轮廓（与托盘图标/Logo 同一多边形，0~100 坐标）
const LEAF_POINTS =
  "50,2 56,18 68,11 64,28 82,21 75,38 94,33 84,50 97,57 72,64 78,79 60,76 57,88 53,88 53,98 47,98 47,88 43,88 40,76 22,79 28,64 3,57 16,50 6,33 25,38 18,21 36,28 32,11 44,18"
    .split(" ")
    .map((p) => p.split(",").map(Number) as [number, number]);

// 卡片配色：暖色暗底 + 枫叶渐变，和应用主题同一套气质
const C = {
  bg0: "#1a1510",
  bg1: "#12100c",
  panel: "#221c15",
  border: "#3a3028",
  text: "#f0e9df",
  muted: "#a89c8c",
  red: "#d43a2f",
  orange: "#ef6a4a",
  amber: "#f2a154",
  up: "#6fbf73",
  down: "#e2543f",
};

const FONT = "'Segoe UI', 'PingFang SC', 'Microsoft YaHei', system-ui, sans-serif";
const MONO = "'Cascadia Code', Consolas, monospace";

function roundRect(ctx: CanvasRenderingContext2D, x: number, y: number, w: number, h: number, r: number) {
  ctx.beginPath();
  ctx.moveTo(x + r, y);
  ctx.arcTo(x + w, y, x + w, y + h, r);
  ctx.arcTo(x + w, y + h, x, y + h, r);
  ctx.arcTo(x, y + h, x, y, r);
  ctx.arcTo(x, y, x + w, y, r);
  ctx.closePath();
}

function drawLeaf(ctx: CanvasRenderingContext2D, cx: number, cy: number, size: number) {
  const s = size / 100;
  ctx.beginPath();
  LEAF_POINTS.forEach(([x, y], i) => {
    const px = cx + (x - 50) * s;
    const py = cy + (y - 50) * s;
    if (i === 0) ctx.moveTo(px, py);
    else ctx.lineTo(px, py);
  });
  ctx.closePath();
  ctx.fill();
}

function grad(ctx: CanvasRenderingContext2D, x0: number, y0: number, x1: number, y1: number) {
  const g = ctx.createLinearGradient(x0, y0, x1, y1);
  g.addColorStop(0, C.orange);
  g.addColorStop(1, C.amber);
  return g;
}

function barGrad(ctx: CanvasRenderingContext2D, y0: number, y1: number) {
  const g = ctx.createLinearGradient(0, y1, 0, y0);
  g.addColorStop(0, C.red);
  g.addColorStop(1, C.amber);
  return g;
}

// renderReportCard 把周期报告画成一张可分享的 PNG 卡片
export function renderReportCard(rep: Report, t: TFunc): HTMLCanvasElement {
  const S = 2; // 2 倍像素，导出不糊
  const W = 800;
  const PAD = 40;
  const models = rep.models.slice(0, 5);
  const daily = rep.daily ?? [];

  // 先算高度：顶部留白 + 头部 96 + 大数字区 120 + 统计行 108 + 图表 210 + 模型区 + 底部 56
  const modelH = models.length > 0 ? 44 + models.length * 40 : 0;
  const H = PAD + 96 + 120 + 108 + 210 + modelH + 56;

  const canvas = document.createElement("canvas");
  canvas.width = W * S;
  canvas.height = H * S;
  const ctx = canvas.getContext("2d")!;
  ctx.scale(S, S);

  // 背景
  const bg = ctx.createLinearGradient(0, 0, 0, H);
  bg.addColorStop(0, C.bg0);
  bg.addColorStop(1, C.bg1);
  ctx.fillStyle = bg;
  roundRect(ctx, 0, 0, W, H, 20);
  ctx.fill();
  ctx.save();
  roundRect(ctx, 0, 0, W, H, 20);
  ctx.clip();

  let y = PAD;

  // ---- 头部：枫叶 + MapleBurn + 日期范围 ----
  const leafBg = ctx.createLinearGradient(PAD, y, PAD + 44, y + 44);
  leafBg.addColorStop(0, C.orange);
  leafBg.addColorStop(1, C.red);
  ctx.fillStyle = leafBg;
  roundRect(ctx, PAD, y, 44, 44, 12);
  ctx.fill();
  ctx.fillStyle = "#ffffff";
  drawLeaf(ctx, PAD + 22, y + 22, 30);

  ctx.fillStyle = C.text;
  ctx.font = `700 24px ${FONT}`;
  ctx.textBaseline = "middle";
  ctx.textAlign = "left";
  ctx.fillText("MapleBurn", PAD + 58, y + 16);
  ctx.fillStyle = C.muted;
  ctx.font = `13px ${FONT}`;
  const periodName = rep.period === "week" ? t("report.weekName") : t("report.monthName");
  ctx.fillText(`${rep.start} ~ ${rep.end}`, PAD + 58, y + 34);

  ctx.textAlign = "right";
  ctx.fillStyle = C.muted;
  ctx.font = `14px ${FONT}`;
  ctx.fillText(periodName, W - PAD, y + 22);
  ctx.textAlign = "left";
  y += 96;

  // ---- 大数字 + 环比 ----
  const x = rep.totals;
  ctx.font = `700 56px ${FONT}`;
  const num = fmtNum(x.totalTokens);
  ctx.fillStyle = grad(ctx, PAD, y, PAD + 400, y);
  ctx.fillText(num, PAD, y + 28);
  const numW = ctx.measureText(num).width;
  ctx.font = `18px ${FONT}`;
  ctx.fillStyle = C.muted;
  ctx.fillText("tokens", PAD + numW + 10, y + 34);

  const delta = x.totalTokens - rep.prevTotals.totalTokens;
  if (rep.prevTotals.totalTokens > 0) {
    const pct = ((delta / rep.prevTotals.totalTokens) * 100).toFixed(1);
    const up = delta >= 0;
    const chip = `${up ? "▲" : "▼"} ${up ? "+" : ""}${pct}%  ${t("report.vsPrev")}`;
    ctx.font = `600 15px ${FONT}`;
    const chipW = ctx.measureText(chip).width + 28;
    const chipY = y + 12;
    ctx.fillStyle = up ? "rgba(111,191,115,0.14)" : "rgba(226,84,63,0.14)";
    roundRect(ctx, W - PAD - chipW, chipY, chipW, 32, 16);
    ctx.fill();
    ctx.fillStyle = up ? C.up : C.down;
    ctx.textAlign = "right";
    ctx.fillText(chip, W - PAD - 14, chipY + 17);
    ctx.textAlign = "left";
  }
  y += 120;

  // ---- 统计行 ----
  const hit =
    x.inputTokens + x.cacheCreate + x.cacheRead > 0
      ? ((x.cacheRead / (x.inputTokens + x.cacheCreate + x.cacheRead)) * 100).toFixed(1) + "%"
      : "-";
  const cells: [string, string][] = [
    [t("report.requests"), fmtFull(x.requests)],
    [t("report.activeDays"), String(rep.activeDays)],
    [t("report.outputTokens"), fmtNum(x.outputTokens)],
    [t("widget.cacheHit"), hit],
  ];
  const cellW = (W - PAD * 2 - 3 * 12) / 4;
  cells.forEach(([label, value], i) => {
    const cx = PAD + i * (cellW + 12);
    ctx.fillStyle = C.panel;
    roundRect(ctx, cx, y, cellW, 88, 12);
    ctx.fill();
    ctx.strokeStyle = C.border;
    ctx.lineWidth = 1;
    roundRect(ctx, cx + 0.5, y + 0.5, cellW - 1, 87, 12);
    ctx.stroke();
    ctx.fillStyle = C.muted;
    ctx.font = `12px ${FONT}`;
    ctx.fillText(label, cx + 14, y + 24);
    ctx.fillStyle = C.text;
    ctx.font = `700 22px ${FONT}`;
    ctx.fillText(value, cx + 14, y + 58);
  });
  y += 108;

  // ---- 逐日柱状图 ----
  if (daily.length > 0) {
    ctx.fillStyle = C.muted;
    ctx.font = `13px ${FONT}`;
    ctx.fillText(t("widget.bars.week"), PAD, y + 8);
    const chartY = y + 24;
    const chartH = 150;
    const chartW = W - PAD * 2;
    const max = Math.max(...daily.map((d) => d.totalTokens), 1);
    const gap = 4;
    const bw = (chartW - gap * (daily.length - 1)) / daily.length;
    daily.forEach((d, i) => {
      const bh = Math.max((d.totalTokens / max) * (chartH - 10), d.totalTokens > 0 ? 4 : 2);
      const bx = PAD + i * (bw + gap);
      const by = chartY + chartH - bh;
      ctx.fillStyle = d.totalTokens > 0 ? barGrad(ctx, by, chartY + chartH) : C.panel;
      roundRect(ctx, bx, by, bw, bh, Math.min(3, bw / 2));
      ctx.fill();
    });
    // 首/中/尾日期刻度
    ctx.fillStyle = C.muted;
    ctx.font = `11px ${FONT}`;
    const tick = (i: number, align: CanvasTextAlign) => {
      ctx.textAlign = align;
      ctx.fillText(daily[i].date.slice(5), PAD + i * (bw + gap) + bw / 2, chartY + chartH + 16);
    };
    tick(0, "left");
    if (daily.length > 2) tick(Math.floor(daily.length / 2), "center");
    tick(daily.length - 1, "right");
    ctx.textAlign = "left";
  }
  y += 210;

  // ---- Top 模型 ----
  if (models.length > 0) {
    ctx.fillStyle = C.muted;
    ctx.font = `13px ${FONT}`;
    ctx.fillText(t("report.topModels"), PAD, y + 4);
    y += 16;
    const maxTok = Math.max(...models.map((m) => m.inputTokens + m.outputTokens + m.cacheCreate + m.cacheRead), 1);
    models.forEach((m, i) => {
      const ry = y + i * 40 + 8;
      const total = m.inputTokens + m.outputTokens + m.cacheCreate + m.cacheRead;
      ctx.fillStyle = C.muted;
      ctx.font = `13px ${MONO}`;
      ctx.fillText(String(i + 1), PAD, ry + 8);
      ctx.fillStyle = C.text;
      let name = m.model;
      while (name.length > 4 && ctx.measureText(name).width > 260) name = name.slice(0, -2);
      if (name !== m.model) name += "…";
      ctx.fillText(name, PAD + 22, ry + 8);
      // 占比条
      const barX = PAD + 300;
      const barW = W - PAD - barX - 110;
      ctx.fillStyle = C.panel;
      roundRect(ctx, barX, ry, barW, 16, 8);
      ctx.fill();
      const fw = Math.max((total / maxTok) * barW, 4);
      ctx.fillStyle = grad(ctx, barX, ry, barX + barW, ry);
      roundRect(ctx, barX, ry, fw, 16, 8);
      ctx.fill();
      ctx.fillStyle = C.muted;
      ctx.font = `13px ${FONT}`;
      ctx.textAlign = "right";
      ctx.fillText(fmtNum(total), W - PAD, ry + 8);
      ctx.textAlign = "left";
    });
    y += models.length * 40 + 28;
  }

  // ---- 底部 ----
  ctx.strokeStyle = C.border;
  ctx.beginPath();
  ctx.moveTo(PAD, y - 8);
  ctx.lineTo(W - PAD, y - 8);
  ctx.stroke();
  ctx.fillStyle = C.muted;
  ctx.font = `12px ${FONT}`;
  ctx.fillText("MapleBurn · Claude Code token 用量报告", PAD, y + 12);
  ctx.textAlign = "right";
  ctx.fillText(new Date().toLocaleString(), W - PAD, y + 12);
  ctx.textAlign = "left";

  ctx.restore();
  return canvas;
}

// exportReportImage 渲染卡片并复制到剪贴板；剪贴板不可用时降级为下载
export async function exportReportImage(rep: Report, t: TFunc): Promise<"clipboard" | "download"> {
  const canvas = renderReportCard(rep, t);
  const blob = await new Promise<Blob | null>((resolve) => canvas.toBlob(resolve, "image/png"));
  if (!blob) throw new Error("toBlob failed");
  try {
    await navigator.clipboard.write([new ClipboardItem({ "image/png": blob })]);
    return "clipboard";
  } catch {
    const a = document.createElement("a");
    a.href = URL.createObjectURL(blob);
    a.download = `mapleburn-${rep.period}-${rep.end}.png`;
    a.click();
    URL.revokeObjectURL(a.href);
    return "download";
  }
}
