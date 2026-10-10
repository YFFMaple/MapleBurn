// AI Token 统计 —— 前端逻辑
// 通过 fetch 调用本地 Go 服务（127.0.0.1）
const $ = (sel) => document.querySelector(sel);

async function api(path, opts) {
  const res = await fetch(path, opts);
  if (!res.ok) throw new Error(await res.text());
  return res.json();
}
const apiGet = (path) => api(path);
const apiPost = (path, body) =>
  api(path, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: body ? JSON.stringify(body) : undefined,
  });

// ---------- 格式化 ----------
let zhUnit = localStorage.getItem("zhUnit") === "1";
function fmtNum(n) {
  if (zhUnit) {
    if (n >= 1e8) return (n / 1e8).toFixed(2) + "亿";
    if (n >= 1e4) return (n / 1e4).toFixed(1) + "万";
    return String(n);
  }
  if (n >= 1e9) return (n / 1e9).toFixed(2) + "B";
  if (n >= 1e6) return (n / 1e6).toFixed(2) + "M";
  if (n >= 1e3) return (n / 1e3).toFixed(1) + "K";
  return String(n);
}
function fmtFull(n) { return n.toLocaleString("en-US"); }
function fmtCost(c) { return c >= 0.01 ? "$" + c.toFixed(2) : "$" + c.toFixed(4); }
function todayStr() {
  const d = new Date();
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;
}

let selectedDate = todayStr();

// ---------- 范围选择 ----------
function addDays(dateStr, n) {
  const d = new Date(dateStr + "T00:00:00");
  d.setDate(d.getDate() + n);
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;
}
function daysBetween(a, b) {
  return Math.round((new Date(b) - new Date(a)) / 86400000) + 1;
}
function rangeFor(key) {
  const today = todayStr();
  switch (key) {
    case "today": return { start: today, end: today };
    case "yesterday": {
      const y = addDays(today, -1);
      return { start: y, end: y };
    }
    case "7d": return { start: addDays(today, -6), end: today };
    case "30d": return { start: addDays(today, -29), end: today };
    case "week": {
      const dow = (new Date().getDay() + 6) % 7; // 周一为一周开始
      return { start: addDays(today, -dow), end: today };
    }
    case "month": return { start: today.slice(0, 8) + "01", end: today };
    case "all": return { start: "2000-01-01", end: today };
    default: return { start: addDays(today, -29), end: today };
  }
}
let currentRange = rangeFor("30d");
let detailRange = rangeFor("30d");
let lastPreset = "30d";

// ---------- 范围选择器组件（Fluent 风格浮层：预设 + 双月日历） ----------
const rp = {
  el: null, outside: null, onApply: null,
  selStart: null, selEnd: null,
  viewYear: 0, viewMonth: 0, activePreset: null,

  open(anchor, current, onApply, activePreset) {
    this.close();
    this.onApply = onApply;
    this.selStart = current.start;
    this.selEnd = current.end;
    this.activePreset = activePreset || null;
    // 左历显示结束日期的前一个月，让选区尽量落在视图中
    const end = new Date(current.end + "T00:00:00");
    this.viewYear = end.getFullYear();
    this.viewMonth = end.getMonth() - 1;
    if (this.viewMonth < 0) { this.viewMonth = 11; this.viewYear--; }

    this.el = document.createElement("div");
    this.el.className = "range-popover";
    document.body.appendChild(this.el);
    this.render();

    // 定位：右对齐触发按钮，超出底部则向上弹
    const rect = anchor.getBoundingClientRect();
    const pw = this.el.offsetWidth, ph = this.el.offsetHeight;
    let left = Math.max(8, rect.right - pw);
    let top = rect.bottom + 6;
    if (top + ph > window.innerHeight - 8) top = Math.max(8, rect.top - ph - 6);
    this.el.style.left = left + "px";
    this.el.style.top = top + "px";

    setTimeout(() => {
      this.outside = (e) => {
        if (this.el && !this.el.contains(e.target) && !anchor.contains(e.target)) this.close();
      };
      document.addEventListener("mousedown", this.outside);
    });
  },

  close() {
    if (this.el) { this.el.remove(); this.el = null; }
    if (this.outside) { document.removeEventListener("mousedown", this.outside); this.outside = null; }
  },

  render() {
    const presets = [
      ["today", "今天"], ["yesterday", "昨日"], ["7d", "近7天"], ["30d", "近30天"],
      ["week", "本周"], ["month", "本月"], ["all", "全部"],
    ];
    const presetHtml = presets.map(([k, label]) =>
      `<button class="rp-preset ${this.activePreset === k ? "active" : ""}" data-p="${k}">${label}</button>`).join("");

    const selText = this.selStart
      ? (this.selEnd ? `${this.selStart} ~ ${this.selEnd}` : `${this.selStart} ~ 请选择`)
      : "请选择日期";

    this.el.innerHTML = `
      <div class="rp-presets">${presetHtml}</div>
      <div class="rp-main">
        <div class="rp-calendars">${this.renderCalendar(0)}${this.renderCalendar(1)}</div>
        <div class="rp-footer">
          <span class="rp-selection">${selText}</span>
          <div class="rp-footer-btns">
            <button class="btn btn-subtle" data-act="cancel">取消</button>
            <button class="btn btn-accent" data-act="apply">应用</button>
          </div>
        </div>
      </div>`;

    this.el.querySelectorAll(".rp-preset").forEach((b) => {
      b.onclick = () => {
        const r = rangeFor(b.dataset.p);
        this.selStart = r.start;
        this.selEnd = r.end;
        this.activePreset = b.dataset.p;
        const end = new Date(r.end + "T00:00:00");
        this.viewYear = end.getFullYear();
        this.viewMonth = end.getMonth() - 1;
        if (this.viewMonth < 0) { this.viewMonth = 11; this.viewYear--; }
        this.render();
      };
    });
    this.el.querySelectorAll(".rp-day").forEach((d) => {
      d.onclick = () => this.pickDay(d.dataset.d);
    });
    this.el.querySelectorAll(".rp-nav").forEach((n) => {
      n.onclick = () => {
        this.viewMonth += parseInt(n.dataset.nav);
        if (this.viewMonth < 0) { this.viewMonth = 11; this.viewYear--; }
        if (this.viewMonth > 11) { this.viewMonth = 0; this.viewYear++; }
        this.render();
      };
    });
    this.el.querySelector('[data-act="cancel"]').onclick = () => this.close();
    this.el.querySelector('[data-act="apply"]').onclick = () => {
      if (!this.selStart) return;
      if (!this.selEnd) this.selEnd = this.selStart;
      const r = { start: this.selStart, end: this.selEnd };
      const cb = this.onApply;
      this.close();
      cb(r);
    };
  },

  pickDay(d) {
    if (!this.selStart || (this.selStart && this.selEnd)) {
      this.selStart = d;
      this.selEnd = null;
    } else if (d < this.selStart) {
      this.selEnd = this.selStart;
      this.selStart = d;
    } else {
      this.selEnd = d;
    }
    this.activePreset = null;
    this.render();
  },

  renderCalendar(offset) {
    let y = this.viewYear, m = this.viewMonth + offset;
    if (m > 11) { m = 0; y++; }
    const isLeft = offset === 0;
    const cells = monthMatrix(y, m);
    const dow = ["一", "二", "三", "四", "五", "六", "日"]
      .map((d) => `<div class="rp-dow">${d}</div>`).join("");
    const days = cells.map((c) => {
      let cls = "rp-day";
      if (c.other) cls += " other";
      if (c.date === todayStr()) cls += " today";
      if (this.selStart && this.selEnd && c.date > this.selStart && c.date < this.selEnd) cls += " in-range";
      if (c.date === this.selStart || c.date === this.selEnd) cls += " sel";
      return `<div class="${cls}" data-d="${c.date}">${c.day}</div>`;
    }).join("");

    return `
      <div class="rp-cal">
        <div class="rp-cal-header">
          <button class="rp-nav ${isLeft ? "" : "hidden"}" data-nav="-1">‹</button>
          <span class="rp-title">${y}年${m + 1}月</span>
          <button class="rp-nav ${isLeft ? "hidden" : ""}" data-nav="1">›</button>
        </div>
        <div class="rp-grid">${dow}${days}</div>
      </div>`;
  },
};

// 生成一个月历格子（周一起始，含前后月补位）
function monthMatrix(year, month) {
  const first = new Date(year, month, 1);
  const startOffset = (first.getDay() + 6) % 7;
  const base = new Date(year, month, 1 - startOffset);
  const cells = [];
  for (let i = 0; i < 42; i++) {
    const d = new Date(base.getFullYear(), base.getMonth(), base.getDate() + i);
    cells.push({
      date: fmtDate(d),
      day: d.getDate(),
      other: d.getMonth() !== month,
    });
  }
  return cells;
}
function fmtDate(d) {
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;
}

function rangeLabel(r) {
  return r.start === r.end ? r.start : `${r.start} ~ ${r.end}`;
}

function setRangeActive(key) {
  document.querySelectorAll(".range-btn").forEach((b) =>
    b.classList.toggle("active", b.dataset.range === key));
}

async function applyRange(range, label) {
  currentRange = range;
  const span = `${range.start} ~ ${range.end}`;
  document.querySelectorAll(".range-label").forEach((el) => (el.textContent = label || span));
  $("#chart-title").textContent = `趋势（${span}）`;
  $("#range-trigger-text").textContent = span;
  // 明细面板默认跟随全局范围
  detailRange = { start: range.start, end: range.end };
  $("#detail-trigger-text").textContent = rangeLabel(detailRange);
  await Promise.all([loadRangeStats(), loadChart(), loadProjects(), loadDetail(), loadModelRank(), loadSessions()]);
}

// ---------- 概览卡片（跟随范围） ----------
async function loadRangeStats() {
  const r = await apiGet(`/api/range?start=${currentRange.start}&end=${currentRange.end}`);
  const t = r.totals;
  const days = daysBetween(r.start, r.end);

  const tokens = $("#card-tokens");
  tokens.querySelector(".card-value").textContent = fmtNum(t.totalTokens);
  tokens.querySelector(".card-value").title =
    `输入 ${fmtFull(t.inputTokens)} + 输出 ${fmtFull(t.outputTokens)} + 缓存写 ${fmtFull(t.cacheCreate)} + 缓存读 ${fmtFull(t.cacheRead)}`;

  $("#card-requests").querySelector(".card-value").textContent = fmtNum(t.requests);

  const avg = $("#card-daily-avg");
  avg.querySelector(".card-value").textContent = fmtNum(Math.round(t.totalTokens / days));
  avg.querySelector(".card-value").title = fmtFull(Math.round(t.totalTokens / days)) + " tokens/天";

  $("#card-cost").querySelector(".card-value").textContent = fmtCost(t.cost);
}

// ---------- 页脚信息 ----------
async function loadFooter() {
  const ov = await apiGet("/api/overview");
  $("#footer-info").innerHTML =
    `日志目录：${esc(ov.logDir)}<br>数据目录：${esc(ov.dataDir)}（数据库与价格表仅存于本机）`;
}

// ---------- 趋势图 ----------
async function loadChart() {
  // 范围太长时图表最多显示最近 92 天，避免柱子挤成一片
  let { start, end } = currentRange;
  if (daysBetween(start, end) > 92) start = addDays(end, -91);
  const series = await apiGet(`/api/series?start=${start}&end=${end}`);
  const chart = $("#chart");
  chart.innerHTML = "";
  const max = Math.max(...series.map((s) => s.totalTokens), 1);

  for (const s of series) {
    const wrap = document.createElement("div");
    wrap.className = "bar-wrap" + (s.date === selectedDate ? " active" : "");
    const h = Math.max((s.totalTokens / max) * 100, 1.5);
    wrap.innerHTML = `
      <div class="bar-tip">${s.date}<br>${fmtFull(s.totalTokens)} tokens · ${fmtCost(s.cost)}</div>
      <div class="bar" style="height:${h}%"></div>
      <div class="bar-date">${s.date.slice(5)}</div>`;
    wrap.onclick = () => {
      selectedDate = s.date;
      detailRange = { start: s.date, end: s.date };
      $("#detail-trigger-text").textContent = s.date;
      loadDetail();
      chart.querySelectorAll(".bar-wrap").forEach((b) => b.classList.remove("active"));
      wrap.classList.add("active");
    };
    chart.appendChild(wrap);
  }
}

// ---------- 模型明细（范围查询） ----------
// 同一轮刷新中明细面板和模型排行可能请求相同的 /api/models，合并为一个请求
const modelsInflight = new Map();
function fetchModels(start, end) {
  const key = `${start}|${end}`;
  if (!modelsInflight.has(key)) {
    const p = apiGet(`/api/models?start=${start}&end=${end}`);
    modelsInflight.set(key, p);
    p.then(() => modelsInflight.delete(key), () => modelsInflight.delete(key));
  }
  return modelsInflight.get(key);
}

async function loadDetail() {
  const { start, end } = detailRange;
  if (!start || !end) return;
  const rep = await fetchModels(start, end);
  const t = rep.totals;

  $("#daily-summary").innerHTML = `
    <span>请求 <b>${fmtFull(t.requests)}</b></span>
    <span>输入 <b>${fmtFull(t.inputTokens)}</b></span>
    <span>输出 <b>${fmtFull(t.outputTokens)}</b></span>
    <span>缓存读取 <b>${fmtFull(t.cacheRead)}</b></span>
    <span>缓存写入 <b>${fmtFull(t.cacheCreate)}</b></span>
    <span>费用 <b>${fmtCost(t.cost)}</b></span>`;

  const tbody = $("#daily-table tbody");
  tbody.innerHTML = "";
  const models = rep.models || [];
  $("#daily-empty").hidden = models.length > 0;

  for (const m of models) {
    const total = m.inputTokens + m.outputTokens + m.cacheCreate + m.cacheRead;
    const pct = t.totalTokens > 0 ? ((total / t.totalTokens) * 100).toFixed(1) : "0.0";
    const tr = document.createElement("tr");
    tr.innerHTML = `
      <td class="mono">${esc(m.model)}</td>
      <td>${fmtFull(m.requests)}</td>
      <td>${fmtFull(m.inputTokens)}</td>
      <td>${fmtFull(m.outputTokens)}</td>
      <td>${fmtFull(m.cacheCreate)}</td>
      <td>${fmtFull(m.cacheRead)}</td>
      <td>${fmtCost(m.cost)}</td>
      <td style="min-width:110px">${pct}%<div class="pct-bar" style="width:${pct}%"></div></td>`;
    tbody.appendChild(tr);
  }
}

// ---------- 项目排行 ----------
async function loadProjects() {
  const list = await apiGet(`/api/projects?start=${currentRange.start}&end=${currentRange.end}`);
  const tbody = $("#project-table tbody");
  tbody.innerHTML = "";
  const max = Math.max(...(list || []).map((p) => p.totalTokens), 1);
  for (const p of list || []) {
    const pct = ((p.totalTokens / max) * 100).toFixed(1);
    const tr = document.createElement("tr");
    tr.innerHTML = `
      <td class="mono">${esc(p.project)}</td>
      <td>${fmtFull(p.requests)}</td>
      <td>${fmtFull(p.totalTokens)}</td>
      <td>${fmtCost(p.cost)}</td>
      <td style="width:22%"><div class="pct-bar" style="width:${pct}%"></div></td>`;
    tbody.appendChild(tr);
  }
}

// ---------- 模型排行 ----------
async function loadModelRank() {
  const rep = await fetchModels(currentRange.start, currentRange.end);
  const tbody = $("#model-rank-table tbody");
  tbody.innerHTML = "";
  const models = rep.models || [];
  const max = Math.max(...models.map((m) =>
    m.inputTokens + m.outputTokens + m.cacheCreate + m.cacheRead), 1);
  for (const m of models) {
    const total = m.inputTokens + m.outputTokens + m.cacheCreate + m.cacheRead;
    const pct = ((total / max) * 100).toFixed(1);
    const tr = document.createElement("tr");
    tr.innerHTML = `
      <td class="mono">${esc(m.model)}</td>
      <td>${fmtFull(m.requests)}</td>
      <td>${fmtNum(total)}</td>
      <td>${fmtCost(m.cost)}</td>
      <td style="width:22%"><div class="pct-bar" style="width:${pct}%"></div></td>`;
    tbody.appendChild(tr);
  }
}

// ---------- 会话列表与内容 ----------
async function loadSessions() {
  const list = await apiGet(`/api/sessions?start=${currentRange.start}&end=${currentRange.end}&limit=50`);
  const tbody = $("#session-table tbody");
  tbody.innerHTML = "";
  $("#session-empty").hidden = (list || []).length > 0;
  for (const s of list || []) {
    const tr = document.createElement("tr");
    tr.className = "row-click";
    tr.title = "点击查看会话内容";
    tr.innerHTML = `
      <td class="mono" title="${esc(s.sessionId)}">${esc(s.sessionId.slice(0, 8))}…</td>
      <td class="mono">${esc(s.project)}</td>
      <td class="mono">${esc(s.models)}</td>
      <td>${fmtFull(s.requests)}</td>
      <td>${fmtNum(s.totalTokens)}</td>
      <td>${fmtCost(s.cost)}</td>
      <td>${esc(s.lastTs)}</td>`;
    tr.onclick = () => openChat(s.project, s.sessionId);
    tbody.appendChild(tr);
  }
}

async function openChat(project, session) {
  $("#chat-modal").hidden = false;
  $("#chat-title").textContent = `${project} / ${session.slice(0, 8)}…`;
  const body = $("#chat-body");
  body.innerHTML = '<div class="empty">加载中…</div>';
  try {
    const msgs = await apiGet(
      `/api/session-messages?project=${encodeURIComponent(project)}&session=${encodeURIComponent(session)}`);
    body.innerHTML = "";
    if (!msgs || msgs.length === 0) {
      body.innerHTML = '<div class="empty">这个会话没有可展示的文本消息</div>';
      return;
    }
    for (const m of msgs) {
      const div = document.createElement("div");
      div.className = `msg ${m.role}`;
      let meta = m.ts || "";
      if (m.role === "assistant" && m.model) {
        meta += ` · ${m.model} · 入 ${fmtFull(m.inputTokens)} / 出 ${fmtFull(m.outputTokens)}`;
      }
      div.innerHTML = `${esc(m.text)}<span class="msg-meta">${esc(meta)}</span>`;
      body.appendChild(div);
    }
  } catch (e) {
    body.innerHTML = `<div class="empty">加载失败：${esc(e.message)}</div>`;
  }
}

// ---------- 价格表 ----------
let pricingCache = {};

async function loadPricing() {
  const [table, info] = await Promise.all([
    apiGet("/api/pricing"),
    apiGet("/api/pricing/community"),
  ]);
  $("#community-info").textContent = info.fetchedAt
    ? `社区价格表：${fmtFull(info.count)} 个模型，更新于 ${info.fetchedAt}`
    : "社区价格表：尚未更新（当前使用内置兜底价格）";
  pricingCache = table;
  renderPricingTable(table);
}

function renderPricingTable(table) {
  const filter = ($("#p-filter").value || "").trim().toLowerCase();
  const tbody = $("#pricing-table tbody");
  tbody.innerHTML = "";
  // 自定义排前面，其余按名称排序，筛选后最多渲染 300 行
  const names = Object.keys(table).sort((a, b) => {
    const ca = table[a].custom ? 0 : 1, cb = table[b].custom ? 0 : 1;
    return ca - cb || a.localeCompare(b);
  }).filter((n) => !filter || n.toLowerCase().includes(filter));
  const shown = names.slice(0, 300);
  for (const name of shown) {
    const p = table[name];
    const tr = document.createElement("tr");
    tr.innerHTML = `
      <td class="mono">${esc(name)}${p.custom ? '<span class="tag-custom">自定义</span>' : ""}</td>
      <td>${round2(p.input)}</td><td>${round2(p.output)}</td><td>${p.cacheCreate ? round2(p.cacheCreate) : "-"}</td><td>${p.cacheRead ? round2(p.cacheRead) : "-"}</td>
      <td>${p.custom ? `<button class="btn btn-subtle" data-del="${esc(name)}">删除</button>` : ""}</td>`;
    tbody.appendChild(tr);
  }
  if (names.length > shown.length) {
    const tr = document.createElement("tr");
    tr.innerHTML = `<td colspan="6" class="hint">… 还有 ${fmtFull(names.length - shown.length)} 条，请用筛选缩小范围</td>`;
    tbody.appendChild(tr);
  }
  tbody.querySelectorAll("[data-del]").forEach((btn) => {
    btn.onclick = async () => {
      await api(`/api/pricing?model=${encodeURIComponent(btn.dataset.del)}`, { method: "DELETE" });
      loadPricing();
    };
  });
}

function round2(n) { return Math.round(n * 100) / 100; }

async function addPrice() {
  const model = $("#p-model").value.trim();
  if (!model) return;
  await apiPost("/api/pricing", {
    model,
    input: parseFloat($("#p-in").value) || 0,
    output: parseFloat($("#p-out").value) || 0,
    cacheCreate: parseFloat($("#p-cc").value) || 0,
    cacheRead: parseFloat($("#p-cr").value) || 0,
  });
  $("#p-model").value = $("#p-in").value = $("#p-out").value = $("#p-cc").value = $("#p-cr").value = "";
  loadPricing();
}

async function updatePricing() {
  const btn = $("#btn-update-pricing");
  btn.disabled = true;
  btn.textContent = "更新中…";
  try {
    const res = await apiPost("/api/pricing/update");
    btn.textContent = `✓ 已更新 ${fmtFull(res.count)} 个模型`;
    await loadPricing();
  } catch (e) {
    btn.textContent = "更新失败，重试";
    alert("更新社区价格表失败：" + e.message);
  } finally {
    setTimeout(() => { btn.disabled = false; btn.textContent = "⇩ 更新社区价格表"; }, 2000);
  }
}

async function reprice() {
  const btn = $("#btn-reprice");
  btn.disabled = true;
  btn.textContent = "重算中…";
  try {
    const res = await apiPost("/api/reprice");
    btn.textContent = `✓ 已重算 ${fmtFull(res.updated)} 条`;
    await refresh();
  } catch (e) {
    btn.textContent = "重算失败";
    alert("重算历史费用失败：" + e.message);
  } finally {
    setTimeout(() => { btn.disabled = false; btn.textContent = "↺ 重算历史费用"; }, 2000);
  }
}

// ---------- 主题切换 ----------
function applyTheme(light) {
  document.body.classList.toggle("light", light);
  $("#btn-theme").textContent = light ? "☀" : "🌙";
}

// ---------- 完全重新扫描 ----------
async function rescan() {
  if (!confirm("将清空已扫描的统计数据并从日志完全重建，确定吗？\n（价格和目录配置不受影响）")) return;
  const btn = $("#btn-rescan");
  btn.disabled = true;
  btn.textContent = "重建中…";
  try {
    const res = await apiPost("/api/rescan");
    btn.textContent = `✓ 已重建 ${fmtFull(res.newRecords)} 条`;
    $("#settings-modal").hidden = true;
    await refresh();
  } catch (e) {
    btn.textContent = "重建失败";
    alert("重新扫描失败：" + e.message);
  } finally {
    setTimeout(() => { btn.disabled = false; btn.textContent = "↺ 完全重新扫描"; }, 2500);
  }
}

// ---------- 显示数据调整 ----------
let adjustState = null;
let adjustRevealed = false;

async function loadAdjust() {
  adjustState = await apiGet("/api/adjust");
  adjustRevealed = false;
  renderAdjustInfo();
}

function renderAdjustInfo() {
  const adj = adjustState;
  const info = $("#adjust-info");
  // 真实值默认打码，点击才显示，避免旁人一眼看到
  const realText = adjustRevealed ? fmtFull(adj.realTotal) : "••••••••";
  if (adj.active) {
    info.innerHTML = `基准值：<b class="reveal" id="reveal-real" title="点击显示">${realText}</b> tokens<br>` +
      `当前口径：<b>${fmtFull(Math.round(adj.target))}</b> tokens（×${adj.factor.toFixed(3)}）`;
    $("#adjust-input").placeholder = "修改目标总消耗";
  } else {
    info.innerHTML = `基准值：<b class="reveal" id="reveal-real" title="点击显示">${realText}</b> tokens<br>当前未启用校准。`;
    $("#adjust-input").placeholder = "目标总消耗，如 3B / 30亿 / 500万";
  }
  const reveal = $("#reveal-real");
  if (reveal) reveal.onclick = () => { adjustRevealed = !adjustRevealed; renderAdjustInfo(); };
}

async function applyAdjust() {
  const target = $("#adjust-input").value.trim();
  if (!target) return;
  try {
    await apiPost("/api/adjust", { target });
    $("#adjust-modal").hidden = true;
    await refresh();
  } catch (e) { alert("设置失败：" + e.message); }
}

async function resetAdjust() {
  try {
    await api("/api/adjust", { method: "DELETE" });
    $("#adjust-modal").hidden = true;
    await refresh();
  } catch (e) { alert("重置失败：" + e.message); }
}

// ---------- 日志目录设置 ----------
async function loadLogDirs() {
  const res = await apiGet("/api/logdirs");
  const tbody = $("#dir-table tbody");
  tbody.innerHTML = "";
  for (const d of res.dirs || []) {
    const tr = document.createElement("tr");
    tr.innerHTML = `
      <td class="mono" title="${esc(d.path)}">${esc(d.path)}</td>
      <td>${d.exists ? '<span style="color:#6ccb5f">✓ 存在</span>' : '<span style="color:#f14c4c">✗ 不存在</span>'}</td>
      <td>${(res.dirs || []).length > 1 ? `<button class="btn btn-subtle" data-deldir="${esc(d.path)}">删除</button>` : ""}</td>`;
    tbody.appendChild(tr);
  }
  tbody.querySelectorAll("[data-deldir]").forEach((btn) => {
    btn.onclick = async () => {
      try {
        await api(`/api/logdirs?path=${encodeURIComponent(btn.dataset.deldir)}`, { method: "DELETE" });
        await loadLogDirs();
        loadFooter();
        scan(); // 目录变化后重新扫描
      } catch (e) { alert(e.message); }
    };
  });
}

async function addLogDir() {
  const input = $("#dir-input");
  const path = input.value.trim();
  if (!path) return;
  try {
    await apiPost("/api/logdirs", { path });
    input.value = "";
    await loadLogDirs();
    loadFooter();
    scan(); // 新目录立即扫描
  } catch (e) { alert(e.message); }
}

// ---------- 扫描 ----------
let scanning = false;
async function scan() {
  if (scanning) return;
  scanning = true;
  const btn = $("#btn-scan");
  const status = $("#scan-status");
  btn.disabled = true;
  status.textContent = "扫描中…";
  try {
    const res = await apiPost("/api/scan");
    let msg = `扫描完成：${res.filesScanned} 个文件，新增 ${fmtFull(res.newRecords)} 条记录`;
    if (res.skippedDirs && res.skippedDirs.length > 0) {
      msg += `（⚠ ${res.skippedDirs.length} 个目录不存在，见 ⚙ 设置）`;
    }
    status.textContent = msg;
    await refresh();
  } catch (e) {
    status.textContent = "扫描失败：" + e.message;
  } finally {
    btn.disabled = false;
    scanning = false;
  }
}

function esc(s) {
  return String(s).replace(/[&<>"']/g, (c) =>
    ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c]));
}

async function refresh() {
  await Promise.all([loadRangeStats(), loadChart(), loadDetail(), loadProjects(), loadModelRank(), loadSessions()]);
}

// ---------- 启动 ----------
window.addEventListener("load", async () => {
  // 主题（head 里的 light-pre 只是防闪烁标记）
  const light = localStorage.getItem("theme") === "light";
  document.documentElement.classList.remove("light-pre");
  applyTheme(light);
  $("#btn-theme").onclick = () => {
    const toLight = !document.body.classList.contains("light");
    localStorage.setItem("theme", toLight ? "light" : "dark");
    applyTheme(toLight);
  };

  // 数字单位切换（K/M/B ⇄ 万/亿），记住用户选择
  const unitBtn = $("#btn-unit");
  unitBtn.textContent = zhUnit ? "单位：万/亿" : "单位：K/M";
  unitBtn.onclick = () => {
    zhUnit = !zhUnit;
    localStorage.setItem("zhUnit", zhUnit ? "1" : "0");
    unitBtn.textContent = zhUnit ? "单位：万/亿" : "单位：K/M";
    refresh();
  };

  // 范围快捷按钮
  document.querySelectorAll(".range-btn").forEach((btn) => {
    btn.onclick = () => {
      lastPreset = btn.dataset.range;
      setRangeActive(lastPreset);
      applyRange(rangeFor(lastPreset), btn.textContent);
    };
  });
  // 全局范围选择器
  $("#range-trigger").onclick = () =>
    rp.open($("#range-trigger"), currentRange, (r) => {
      lastPreset = null;
      setRangeActive(null);
      applyRange(r);
    }, lastPreset);
  // 明细范围选择器
  $("#detail-trigger").onclick = () =>
    rp.open($("#detail-trigger"), detailRange, (r) => {
      detailRange = r;
      $("#detail-trigger-text").textContent = rangeLabel(r);
      loadDetail();
    }, null);
  $("#btn-scan").onclick = scan;
  $("#btn-pricing").onclick = () => { $("#pricing-modal").hidden = false; loadPricing(); };
  $("#btn-update-pricing").onclick = updatePricing;
  $("#btn-reprice").onclick = reprice;
  $("#p-filter").oninput = () => renderPricingTable(pricingCache);
  $("#modal-close").onclick = () => ($("#pricing-modal").hidden = true);
  $("#pricing-modal").onclick = (e) => {
    if (e.target.id === "pricing-modal") $("#pricing-modal").hidden = true;
  };
  $("#p-add").onclick = addPrice;
  $("#btn-settings").onclick = () => { $("#settings-modal").hidden = false; loadLogDirs(); };
  $("#settings-close").onclick = () => ($("#settings-modal").hidden = true);
  $("#settings-modal").onclick = (e) => {
    if (e.target.id === "settings-modal") $("#settings-modal").hidden = true;
  };
  $("#dir-add").onclick = addLogDir;
  $("#dir-input").addEventListener("keydown", (e) => { if (e.key === "Enter") addLogDir(); });
  $("#btn-rescan").onclick = rescan;
  $("#btn-adjust").onclick = () => { $("#adjust-modal").hidden = false; loadAdjust(); };
  $("#adjust-close").onclick = () => ($("#adjust-modal").hidden = true);
  $("#adjust-modal").onclick = (e) => {
    if (e.target.id === "adjust-modal") $("#adjust-modal").hidden = true;
  };
  $("#adjust-apply").onclick = applyAdjust;
  $("#adjust-input").addEventListener("keydown", (e) => { if (e.key === "Enter") applyAdjust(); });
  $("#adjust-reset").onclick = resetAdjust;
  $("#chat-close").onclick = () => ($("#chat-modal").hidden = true);
  $("#chat-modal").onclick = (e) => {
    if (e.target.id === "chat-modal") $("#chat-modal").hidden = true;
  };

  await Promise.all([applyRange(currentRange, "近30天"), loadFooter()]);
  await scan(); // 启动时自动扫描一次
  // 每分钟自动增量扫描；标签页隐藏时暂停，回到前台立即补一次
  setInterval(() => { if (!document.hidden) scan(); }, 60 * 1000);
  document.addEventListener("visibilitychange", () => { if (!document.hidden) scan(); });
});
