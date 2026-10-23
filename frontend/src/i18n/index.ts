import { create } from "zustand";

export type Lang = "zh" | "en";

// ---- 字典：zh 是基准，en 与其结构一致（tsc 会校验完整性） ----
const zh = {
  "app.subtitle": "本地 Claude Code 用量",

  // 页签
  "tabs.dashboard": "仪表盘",
  "tabs.events": "请求流水",
  "tabs.search": "搜索",
  "tabs.proxy": "观测代理",

  // 顶栏
  "header.report": "报告",
  "header.themeTip": "切换主题",
  "header.settingsTip": "设置",
  "header.adjustTip": "统计校准",
  "header.scan": "扫描日志",
  "header.scanning": "扫描中…",
  "header.scanDone": "{files} 个文件 · 新增 {records} 条",
  "header.scanSkipped": "（⚠ {n} 个目录不存在）",
  "header.scanFail": "扫描失败：{msg}",

  // 范围栏
  "range.today": "今天",
  "range.yesterday": "昨日",
  "range.7d": "近7天",
  "range.30d": "近30天",
  "range.week": "本周",
  "range.month": "本月",
  "range.all": "全部",
  "range.apply": "应用",
  "range.current": "当前：",

  // 概览卡片
  "cards.totalTokens": "总 Tokens",
  "cards.requests": "请求次数",
  "cards.dailyAvg": "日均 Tokens",
  "cards.tokensPerDay": "tokens/天",

  // 趋势图
  "trend.title": "趋势（{range}）",
  "trend.hint": "点击柱子查看当天明细",
  "trend.tokens": "tokens",

  // 模型明细
  "detail.title": "模型明细（{range}）",
  "detail.requests": "请求",
  "detail.input": "输入",
  "detail.output": "输出",
  "detail.cacheRead": "缓存读取",
  "detail.cacheCreate": "缓存写入",
  "detail.share": "占比",
  "table.model": "模型",
  "table.requests": "请求数",
  "table.input": "输入",
  "table.output": "输出",
  "table.cacheCreate": "缓存写入",
  "table.cacheRead": "缓存读取",
  "table.total": "总量",
  "common.empty": "该范围没有记录",

  // 排行
  "rank.models": "模型排行",
  "rank.projects": "项目排行",
  "rank.tools": "工具使用",
  "rank.files": "文件热点",
  "rank.tool": "工具",
  "rank.count": "次数",
  "rank.errors": "失败",
  "rank.errorRate": "失败率 {pct}%",
  "rank.project": "项目",
  "rank.file": "文件",
  "rank.ops": "操作",
  "rank.noFiles": "该范围没有文件操作",

  // 时段热力图
  "heatmap.title": "时段热力图",
  "heatmap.hint": "一周各时段的 token 分布，越深越集中",
  "heatmap.cellTip": "{dow} {hour}时 · {n} tokens",
  "dow.0": "周一",
  "dow.1": "周二",
  "dow.2": "周三",
  "dow.3": "周四",
  "dow.4": "周五",
  "dow.5": "周六",
  "dow.6": "周日",

  // 年度热力
  "yearmap.title": "年度热力",
  "yearmap.hint": "近一年共 {total} tokens · 颜色越深用量越大",
  "yearmap.cellTip": "{date} · {n} tokens",
  "yearmap.less": "少",
  "yearmap.more": "多",

  // 会话列表
  "sessions.title": "会话列表",
  "sessions.hint": "点击行查看会话内容",
  "sessions.session": "会话",
  "sessions.models": "模型",
  "sessions.totalTokens": "总 tokens",
  "sessions.lastActive": "最后活跃",
  "sessions.empty": "该范围没有会话记录",

  // 效率分析
  "efficiency.title": "效率分析",
  "efficiency.summary": "缓存命中率 {hit}% · 平均每请求 {avg} tokens",
  "efficiency.overall": "整体缓存命中率（缓存读 ÷ 全部输入侧 token）",
  "efficiency.avgOutput": "平均输出",
  "efficiency.avgTotal": "平均每请求总量",
  "efficiency.hitRate": "缓存命中率",
  "efficiency.empty": "该范围没有记录",

  // 仪表盘拖拽
  "dashboard.dragTip": "拖拽调整区块顺序",

  // 请求流水
  "events.title": "请求流水（{range}）",
  "events.modelPh": "模型",
  "events.projectPh": "项目",
  "events.filter": "筛选",
  "events.time": "时间",
  "events.sum": "合计",
  "events.empty": "该范围没有记录",
  "events.pager": "第 {page} / {pages} 页 · 共 {total} 条",
  "events.prev": "上一页",
  "events.next": "下一页",
  "events.rowTip": "点击查看会话内容",

  // 搜索页
  "search.title": "会话全文搜索",
  "search.hint": "搜索历史会话的消息内容，点击结果直达会话",
  "search.placeholder": "输入关键词，中英文均可，如：热力图 / heatmap / 重构",
  "search.button": "搜索",
  "search.searching": "搜索中…",
  "search.empty": "没有找到包含「{q}」的消息",
  "search.me": "我",
  "search.assistant": "助手",

  // 会话弹窗
  "chat.title": "会话内容",
  "chat.loading": "加载中…",
  "chat.empty": "这个会话没有可展示的文本消息",
  "chat.error": "加载失败：{msg}",
  "chat.io": "入 {input} / 出 {output}",

  // 设置弹窗
  "settings.title": "设置",
  "settings.desc": "日志目录（自动探测：CLAUDE_CONFIG_DIR 环境变量 + ~/.claude/projects）、启动行为与集成",
  "settings.addPh": "添加目录，如 D:\\backup\\.claude\\projects 或 ~/logs",
  "settings.add": "添加",
  "settings.dir": "目录",
  "settings.status": "状态",
  "settings.exists": "✓ 存在",
  "settings.missing": "✗ 不存在",
  "settings.autostart": "开机自启",
  "settings.autostartDesc": "登录 Windows 后自动在后台运行（托盘图标）",
  "settings.mcp": "接入 Claude Code（MCP）",
  "settings.mcpDesc": "运行下面的命令后，就可以直接在 Claude Code 里问「我本月用了多少 token」，它会调用 MapleBurn 的数据回答",
  "settings.copy": "复制",
  "settings.copied": "✓",
  "settings.rescanHint": "数据有缺失或漂移时，可以清空后从日志完全重建",
  "settings.rescan": "↺ 完全重新扫描",
  "settings.rebuilding": "重建中…",
  "settings.rescanConfirm": "将清空已扫描的统计数据并从日志完全重建，确定吗？\n（目录配置不受影响）",
  "settings.rescanDone": "✓ 已重建 {n} 条",
  "settings.rescanFail": "重建失败：{msg}",
  "settings.fail": "设置失败：{msg}",

  // 校准弹窗
  "adjust.title": "统计校准",
  "adjust.desc": "按比例调整界面呈现的统计口径，原始记录不受影响，可随时还原",
  "adjust.baseline": "基准值：",
  "adjust.currentFactor": "当前口径：{target} tokens（×{factor}）",
  "adjust.inactive": "当前未启用校准。",
  "adjust.placeholder": "目标总消耗，如 3B / 30亿 / 500万",
  "adjust.placeholderActive": "修改目标总消耗",
  "adjust.apply": "应用",
  "adjust.reset": "↺ 还原真实数据",
  "adjust.setFail": "设置失败：{msg}",
  "adjust.resetFail": "重置失败：{msg}",
  "adjust.revealTip": "点击显示/隐藏",

  // 报告弹窗
  "report.week": "本周",
  "report.month": "本月",
  "report.title": "{period}报告（{range}）",
  "report.titleDefault": "周期报告",
  "report.copy": "复制",
  "report.copied": "✓ 已复制",
  "report.totalTokens": "总 Tokens",
  "report.requests": "请求次数",
  "report.activeDays": "活跃天数",
  "report.outputTokens": "输出 Tokens",
  "report.prevZero": "上期为 0",
  "report.vsPrev": "vs 上期",
  "report.prev": "上期 {n}",
  "report.dailyAvg": "日均 {n}",
  "report.topModels": "Top 模型",
  "report.weekName": "周报",
  "report.monthName": "月报",

  // 观测代理页
  "proxy.title": "观测代理",
  "proxy.on": "已开启",
  "proxy.off": "已关闭",
  "proxy.desc": "开启后自动把 Claude Code 的 ANTHROPIC_BASE_URL 指向代理地址（原值自动备份），透明转发到真实上游，只记录用量和延迟，不修改任何内容；关闭开关或退出程序时自动还原，绝不影响正常使用。改动需重启 Claude Code 生效。",
  "proxy.addr": "代理地址",
  "proxy.upstream": "真实上游",
  "proxy.save": "保存",
  "proxy.provider": "当前服务商",
  "proxy.autoMatch": "按上游域名自动匹配",
  "proxy.running": "运行中",
  "proxy.stopped": "未运行",
  "proxy.detected": "检测到 Claude Code 当前配置的上游是 {url}",
  "proxy.msgOn": "已开启：Claude Code 的 base URL 已自动指向代理（原值已备份），重启 Claude Code 后生效",
  "proxy.msgOff": "已关闭：Claude Code 的 base URL 已自动还原，重启 Claude Code 后生效",
  "proxy.msgFail": "操作失败：{msg}",
  "proxy.copied": "已复制：{text}",
  "proxy.summaryTitle": "近 30 天代理流量",
  "proxy.summaryTotal": "共 {n} 次请求",
  "proxy.summaryEmpty": "还没有代理流量。开启开关并把客户端指向代理地址后，这里会实时出现数据。",
  "proxy.providerModel": "服务商 / 模型",
  "proxy.requestsTitle": "代理请求流水",
  "proxy.providerCol": "服务商",
  "proxy.path": "路径",
  "proxy.statusCol": "状态",
  "proxy.latency": "耗时",
  "proxy.failed": "失败",
  "proxy.empty": "暂无记录",
  "proxy.registryTitle": "服务商注册表",
  "proxy.registryCount": "{n} 个",
  "proxy.fetchPh": "注册表 URL（JSON，含 providers 数组），下载后合并进本地",
  "proxy.fetch": "下载更新",
  "proxy.fetchDone": "注册表已更新，共 {n} 个服务商",
  "proxy.fetchFail": "更新失败：{msg}",
  "proxy.adapterTip": "adapter: {adapter} · 匹配: {hosts}",

  // 桌面小组件
  "widget.today": "今日 tokens",
  "widget.week": "本周",
  "widget.cacheHit": "缓存命中率",
  "widget.hourly": "今日逐小时",
  "widget.pin": "置顶",
  "widget.unpin": "取消置顶",
  "widget.openFull": "打开完整面板",
  "widget.proxyOn": "代理已开启",
  "widget.proxyOff": "代理已关闭",

  // 通用
  "common.total": "总 tokens",
  "common.ms": "ms",
} as const;

export type Key = keyof typeof zh;

const en: Record<Key, string> = {
  "app.subtitle": "Local Claude Code usage",

  "tabs.dashboard": "Dashboard",
  "tabs.events": "Requests",
  "tabs.search": "Search",
  "tabs.proxy": "Proxy",

  "header.report": "Report",
  "header.themeTip": "Toggle theme",
  "header.settingsTip": "Settings",
  "header.adjustTip": "Calibration",
  "header.scan": "Scan logs",
  "header.scanning": "Scanning…",
  "header.scanDone": "{files} files · {records} new records",
  "header.scanSkipped": " (⚠ {n} dirs missing)",
  "header.scanFail": "Scan failed: {msg}",

  "range.today": "Today",
  "range.yesterday": "Yesterday",
  "range.7d": "7D",
  "range.30d": "30D",
  "range.week": "This week",
  "range.month": "This month",
  "range.all": "All",
  "range.apply": "Apply",
  "range.current": "Current: ",

  "cards.totalTokens": "Total Tokens",
  "cards.requests": "Requests",
  "cards.dailyAvg": "Daily Avg Tokens",
  "cards.tokensPerDay": "tokens/day",

  "trend.title": "Trend ({range})",
  "trend.hint": "Click a bar for that day's details",
  "trend.tokens": "tokens",

  "detail.title": "Model details ({range})",
  "detail.requests": "Requests",
  "detail.input": "Input",
  "detail.output": "Output",
  "detail.cacheRead": "Cache read",
  "detail.cacheCreate": "Cache write",
  "detail.share": "Share",
  "table.model": "Model",
  "table.requests": "Requests",
  "table.input": "Input",
  "table.output": "Output",
  "table.cacheCreate": "Cache write",
  "table.cacheRead": "Cache read",
  "table.total": "Total",
  "common.empty": "No records in this range",

  "rank.models": "Models",
  "rank.projects": "Projects",
  "rank.tools": "Tools",
  "rank.files": "Hot files",
  "rank.tool": "Tool",
  "rank.count": "Count",
  "rank.errors": "Errors",
  "rank.errorRate": "Error rate {pct}%",
  "rank.project": "Project",
  "rank.file": "File",
  "rank.ops": "Ops",
  "rank.noFiles": "No file operations in this range",

  "heatmap.title": "Hourly heatmap",
  "heatmap.hint": "Token distribution by hour of week; darker means busier",
  "heatmap.cellTip": "{dow} {hour}:00 · {n} tokens",
  "dow.0": "Mon",
  "dow.1": "Tue",
  "dow.2": "Wed",
  "dow.3": "Thu",
  "dow.4": "Fri",
  "dow.5": "Sat",
  "dow.6": "Sun",

  "yearmap.title": "Yearly heat",
  "yearmap.hint": "{total} tokens in the past year · darker means more",
  "yearmap.cellTip": "{date} · {n} tokens",
  "yearmap.less": "Less",
  "yearmap.more": "More",

  "sessions.title": "Sessions",
  "sessions.hint": "Click a row to view conversation",
  "sessions.session": "Session",
  "sessions.models": "Models",
  "sessions.totalTokens": "Total tokens",
  "sessions.lastActive": "Last active",
  "sessions.empty": "No sessions in this range",

  "efficiency.title": "Efficiency",
  "efficiency.summary": "Cache hit {hit}% · avg {avg} tokens per request",
  "efficiency.overall": "Overall cache hit rate (cache read ÷ all input-side tokens)",
  "efficiency.avgOutput": "Avg output",
  "efficiency.avgTotal": "Avg total per request",
  "efficiency.hitRate": "Cache hit rate",
  "efficiency.empty": "No records in this range",

  "dashboard.dragTip": "Drag to reorder sections",

  "events.title": "Request stream ({range})",
  "events.modelPh": "Model",
  "events.projectPh": "Project",
  "events.filter": "Filter",
  "events.time": "Time",
  "events.sum": "Total",
  "events.empty": "No records in this range",
  "events.pager": "Page {page} / {pages} · {total} total",
  "events.prev": "Prev",
  "events.next": "Next",
  "events.rowTip": "Click to view conversation",

  "search.title": "Full-text session search",
  "search.hint": "Search message content of past sessions; click a result to open it",
  "search.placeholder": "Keywords, e.g. heatmap / refactor",
  "search.button": "Search",
  "search.searching": "Searching…",
  "search.empty": "No messages containing \"{q}\"",
  "search.me": "Me",
  "search.assistant": "Assistant",

  "chat.title": "Conversation",
  "chat.loading": "Loading…",
  "chat.empty": "No displayable text messages in this session",
  "chat.error": "Failed to load: {msg}",
  "chat.io": "in {input} / out {output}",

  "settings.title": "Settings",
  "settings.desc": "Log dirs (auto-detect: CLAUDE_CONFIG_DIR + ~/.claude/projects), startup behavior and integrations",
  "settings.addPh": "Add a dir, e.g. D:\\backup\\.claude\\projects or ~/logs",
  "settings.add": "Add",
  "settings.dir": "Directory",
  "settings.status": "Status",
  "settings.exists": "✓ exists",
  "settings.missing": "✗ missing",
  "settings.autostart": "Run at startup",
  "settings.autostartDesc": "Start in background on Windows login (tray icon)",
  "settings.mcp": "Connect Claude Code (MCP)",
  "settings.mcpDesc": "After running the command below, ask Claude Code \"how many tokens did I use this month\" — it answers via MapleBurn",
  "settings.copy": "Copy",
  "settings.copied": "✓",
  "settings.rescanHint": "If stats are missing or drifting, wipe and rebuild from logs",
  "settings.rescan": "↺ Full rescan",
  "settings.rebuilding": "Rebuilding…",
  "settings.rescanConfirm": "This wipes scanned stats and rebuilds from logs. Continue?\n(Dir config is kept)",
  "settings.rescanDone": "✓ Rebuilt {n} records",
  "settings.rescanFail": "Rebuild failed: {msg}",
  "settings.fail": "Failed: {msg}",

  "adjust.title": "Calibration",
  "adjust.desc": "Scales all displayed numbers by a factor; raw records are untouched and it can be reverted anytime",
  "adjust.baseline": "Baseline: ",
  "adjust.currentFactor": "Current scale: {target} tokens (×{factor})",
  "adjust.inactive": "Calibration is off.",
  "adjust.placeholder": "Target total, e.g. 3B / 500M",
  "adjust.placeholderActive": "Update target total",
  "adjust.apply": "Apply",
  "adjust.reset": "↺ Restore real data",
  "adjust.setFail": "Failed: {msg}",
  "adjust.resetFail": "Reset failed: {msg}",
  "adjust.revealTip": "Click to show/hide",

  "report.week": "This week",
  "report.month": "This month",
  "report.title": "{period} report ({range})",
  "report.titleDefault": "Period report",
  "report.copy": "Copy",
  "report.copied": "✓ Copied",
  "report.totalTokens": "Total Tokens",
  "report.requests": "Requests",
  "report.activeDays": "Active days",
  "report.outputTokens": "Output Tokens",
  "report.prevZero": "prev period was 0",
  "report.vsPrev": "vs prev",
  "report.prev": "prev {n}",
  "report.dailyAvg": "daily avg {n}",
  "report.topModels": "Top models",
  "report.weekName": "weekly report",
  "report.monthName": "monthly report",

  "proxy.title": "Observation proxy",
  "proxy.on": "On",
  "proxy.off": "Off",
  "proxy.desc": "When enabled, Claude Code's ANTHROPIC_BASE_URL is pointed at the proxy automatically (original value is backed up). Traffic is forwarded transparently; only usage and latency are recorded, nothing is modified. Turning it off or quitting the app restores the original value — your usage is never affected. Restart Claude Code to take effect.",
  "proxy.addr": "Proxy address",
  "proxy.upstream": "Real upstream",
  "proxy.save": "Save",
  "proxy.provider": "Provider",
  "proxy.autoMatch": "auto-matched by upstream host",
  "proxy.running": "Running",
  "proxy.stopped": "Stopped",
  "proxy.detected": "Detected Claude Code's current upstream: {url}",
  "proxy.msgOn": "Enabled: Claude Code's base URL now points at the proxy (original backed up). Restart Claude Code to take effect",
  "proxy.msgOff": "Disabled: Claude Code's base URL has been restored. Restart Claude Code to take effect",
  "proxy.msgFail": "Failed: {msg}",
  "proxy.copied": "Copied: {text}",
  "proxy.summaryTitle": "Proxy traffic (last 30 days)",
  "proxy.summaryTotal": "{n} requests",
  "proxy.summaryEmpty": "No proxy traffic yet. Enable the switch and point your client at the proxy address; data appears here in real time.",
  "proxy.providerModel": "Provider / Model",
  "proxy.requestsTitle": "Proxy request stream",
  "proxy.providerCol": "Provider",
  "proxy.path": "Path",
  "proxy.statusCol": "Status",
  "proxy.latency": "Latency",
  "proxy.failed": "Failed",
  "proxy.empty": "No records",
  "proxy.registryTitle": "Provider registry",
  "proxy.registryCount": "{n}",
  "proxy.fetchPh": "Registry URL (JSON with a providers array); merged into local on download",
  "proxy.fetch": "Download",
  "proxy.fetchDone": "Registry updated, {n} providers",
  "proxy.fetchFail": "Update failed: {msg}",
  "proxy.adapterTip": "adapter: {adapter} · matches: {hosts}",

  "widget.today": "Today",
  "widget.week": "This week",
  "widget.cacheHit": "Cache hit rate",
  "widget.hourly": "Today by hour",
  "widget.pin": "Pin on top",
  "widget.unpin": "Unpin",
  "widget.openFull": "Open full dashboard",
  "widget.proxyOn": "Proxy on",
  "widget.proxyOff": "Proxy off",

  "common.total": "Total tokens",
  "common.ms": "ms",
};

const dicts: Record<Lang, Record<Key, string>> = { zh, en };

function detect(): Lang {
  const saved = localStorage.getItem("lang");
  if (saved === "zh" || saved === "en") return saved;
  return navigator.language.toLowerCase().startsWith("zh") ? "zh" : "en";
}

interface I18nState {
  lang: Lang;
  setLang: (l: Lang) => void;
}

export const useI18nStore = create<I18nState>((set) => ({
  lang: detect(),
  setLang: (lang) => {
    localStorage.setItem("lang", lang);
    set({ lang });
  },
}));

type Vars = Record<string, string | number>;

// t 支持 {var} 插值
export function t(lang: Lang, key: Key, vars?: Vars): string {
  let s = dicts[lang][key] ?? dicts.zh[key] ?? key;
  if (vars) {
    for (const [k, v] of Object.entries(vars)) {
      s = s.replaceAll(`{${k}}`, String(v));
    }
  }
  return s;
}

// React hook：订阅语言变化
export function useT() {
  const lang = useI18nStore((s) => s.lang);
  return (key: Key, vars?: Vars) => t(lang, key, vars);
}
