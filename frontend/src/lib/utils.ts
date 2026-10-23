import { clsx, type ClassValue } from "clsx";
import { twMerge } from "tailwind-merge";

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}

// ---- 数字格式化（支持 K/M/B 与 万/亿 两种口径） ----
let zhUnit = localStorage.getItem("zhUnit") === "1";
export function setZhUnit(v: boolean) {
  zhUnit = v;
  localStorage.setItem("zhUnit", v ? "1" : "0");
}
export function getZhUnit() {
  return zhUnit;
}

export function fmtNum(n: number): string {
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

export const fmtFull = (n: number) => n.toLocaleString("en-US");

// ---- 日期工具 ----
export function fmtDate(d: Date): string {
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;
}
export const todayStr = () => fmtDate(new Date());

export function addDays(dateStr: string, n: number): string {
  const d = new Date(dateStr + "T00:00:00");
  d.setDate(d.getDate() + n);
  return fmtDate(d);
}

export function daysBetween(a: string, b: string): number {
  return Math.round((+new Date(b) - +new Date(a)) / 86400000) + 1;
}

export type Range = { start: string; end: string };

export function rangeFor(key: string): Range {
  const today = todayStr();
  switch (key) {
    case "today":
      return { start: today, end: today };
    case "yesterday": {
      const y = addDays(today, -1);
      return { start: y, end: y };
    }
    case "7d":
      return { start: addDays(today, -6), end: today };
    case "30d":
      return { start: addDays(today, -29), end: today };
    case "week": {
      const dow = (new Date().getDay() + 6) % 7;
      return { start: addDays(today, -dow), end: today };
    }
    case "month":
      return { start: today.slice(0, 8) + "01", end: today };
    case "all":
      return { start: "2000-01-01", end: today };
    default:
      return { start: addDays(today, -29), end: today };
  }
}

export function fmtTs(ms: number): string {
  const d = new Date(ms);
  return `${fmtDate(d)} ${String(d.getHours()).padStart(2, "0")}:${String(d.getMinutes()).padStart(2, "0")}`;
}
