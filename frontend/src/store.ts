import { create } from "zustand";
import { rangeFor, type Range } from "@/lib/utils";

interface ChatTarget {
  project: string;
  session: string;
}

interface AppState {
  // 全局时间范围（仪表盘各面板 + 请求流水共用）
  range: Range;
  preset: string | null;
  setRange: (r: Range, preset: string | null) => void;

  // 扫描/重建后 +1，所有面板以此为依赖重新取数
  refreshTick: number;
  bumpRefresh: () => void;

  tab: "dashboard" | "events" | "search" | "proxy";
  setTab: (t: AppState["tab"]) => void;

  // 会话内容弹窗（流水页/会话列表共用）
  chat: ChatTarget | null;
  openChat: (project: string, session: string) => void;
  closeChat: () => void;
}

export const useApp = create<AppState>((set) => ({
  range: rangeFor("30d"),
  preset: "30d",
  setRange: (range, preset) => set({ range, preset }),

  refreshTick: 0,
  bumpRefresh: () => set((s) => ({ refreshTick: s.refreshTick + 1 })),

  tab: "dashboard",
  setTab: (tab) => set({ tab }),

  chat: null,
  openChat: (project, session) => set({ chat: { project, session } }),
  closeChat: () => set({ chat: null }),
}));
