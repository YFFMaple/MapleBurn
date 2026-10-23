import { useCallback, useEffect, useRef, useState } from "react";
import { Moon, Sun, RefreshCw, Settings, FileText, SlidersHorizontal } from "lucide-react";
import { MapleLeaf } from "@/components/MapleLeaf";
import { apiPost, type Overview, type ScanResult, apiGet } from "@/lib/api";
import { fmtFull, getZhUnit, setZhUnit } from "@/lib/utils";
import { useApp } from "@/store";
import { Button } from "@/components/ui/button";

export function Header({
  onOpenSettings,
  onOpenAdjust,
  onOpenReport,
}: {
  onOpenSettings: () => void;
  onOpenAdjust: () => void;
  onOpenReport: () => void;
}) {
  const bumpRefresh = useApp((s) => s.bumpRefresh);
  const [light, setLight] = useState(() => localStorage.getItem("theme") === "light");
  const [zh, setZh] = useState(getZhUnit());
  const [scanning, setScanning] = useState(false);
  const [scanMsg, setScanMsg] = useState("");
  const [footer, setFooter] = useState<Overview | null>(null);
  const scanningRef = useRef(false);

  const scan = useCallback(async () => {
    if (scanningRef.current) return;
    scanningRef.current = true;
    setScanning(true);
    setScanMsg("扫描中…");
    try {
      const res = await apiPost<ScanResult>("/api/scan");
      let msg = `${res.filesScanned} 个文件 · 新增 ${fmtFull(res.newRecords)} 条`;
      if (res.skippedDirs?.length) msg += `（⚠ ${res.skippedDirs.length} 个目录不存在）`;
      setScanMsg(msg);
      bumpRefresh();
    } catch (e) {
      setScanMsg("扫描失败：" + (e as Error).message);
    } finally {
      scanningRef.current = false;
      setScanning(false);
    }
  }, [bumpRefresh]);

  useEffect(() => {
    apiGet<Overview>("/api/overview").then(setFooter).catch(() => {});
    scan(); // 启动自动扫描一次
    const timer = setInterval(() => {
      if (!document.hidden) scan();
    }, 60_000);
    const onVis = () => {
      if (!document.hidden) scan();
    };
    document.addEventListener("visibilitychange", onVis);
    return () => {
      clearInterval(timer);
      document.removeEventListener("visibilitychange", onVis);
    };
  }, [scan]);

  const toggleTheme = () => {
    const toLight = !light;
    setLight(toLight);
    localStorage.setItem("theme", toLight ? "light" : "dark");
    document.documentElement.classList.toggle("light", toLight);
    document.documentElement.classList.toggle("dark", !toLight);
  };

  return (
    <header className="sticky top-0 z-40 border-b bg-background/80 backdrop-blur">
      <div className="flex h-14 items-center justify-between px-5">
        <div className="flex items-center gap-3">
          <div className="flex size-8 items-center justify-center rounded-lg bg-gradient-to-br from-[#ef6a4a] to-[#d43a2f] text-white shadow-sm">
            <MapleLeaf className="size-5" />
          </div>
          <div className="leading-tight">
            <h1 className="text-sm font-semibold">
              燃枫 <span className="font-normal text-muted-foreground">MapleBurn</span>
            </h1>
            <p className="max-w-[420px] truncate text-xs text-muted-foreground" title={footer?.logDir}>
              {footer ? footer.logDir : "本地 Claude Code 用量"}
            </p>
          </div>
        </div>

        <div className="flex items-center gap-1.5">
          <span className="mr-1 hidden text-xs text-muted-foreground md:inline">{scanMsg}</span>
          <Button variant="ghost" size="sm" onClick={onOpenReport}>
            <FileText /> 报告
          </Button>
          <Button
            variant="ghost"
            size="sm"
            onClick={() => {
              const v = !zh;
              setZh(v);
              setZhUnit(v);
              bumpRefresh();
            }}
          >
            {zh ? "万/亿" : "K/M"}
          </Button>
          <Button variant="ghost" size="icon" onClick={toggleTheme} title="切换主题">
            {light ? <Sun /> : <Moon />}
          </Button>
          <Button variant="ghost" size="icon" onClick={onOpenSettings} title="日志目录设置">
            <Settings />
          </Button>
          <Button variant="ghost" size="icon" onClick={onOpenAdjust} title="统计校准">
            <SlidersHorizontal />
          </Button>
          <Button size="sm" onClick={scan} disabled={scanning}>
            <RefreshCw className={scanning ? "animate-spin" : ""} /> 扫描日志
          </Button>
        </div>
      </div>
    </header>
  );
}
