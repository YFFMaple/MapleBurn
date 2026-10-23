import { useEffect, useState } from "react";
import { apiDelete, apiGet, apiPost, type Adjustment } from "@/lib/api";
import { fmtFull } from "@/lib/utils";
import { useApp } from "@/store";
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";

export function AdjustDialog({ open, onOpenChange }: { open: boolean; onOpenChange: (o: boolean) => void }) {
  const bumpRefresh = useApp((s) => s.bumpRefresh);
  const [adj, setAdj] = useState<Adjustment | null>(null);
  const [revealed, setRevealed] = useState(false);
  const [target, setTarget] = useState("");
  const [msg, setMsg] = useState("");

  useEffect(() => {
    if (!open) return;
    setRevealed(false);
    setMsg("");
    apiGet<Adjustment>("/api/adjust").then(setAdj).catch(() => {});
  }, [open]);

  const apply = async () => {
    if (!target.trim()) return;
    setMsg("");
    try {
      await apiPost("/api/adjust", { target: target.trim() });
      onOpenChange(false);
      bumpRefresh();
    } catch (e) {
      setMsg("设置失败：" + (e as Error).message);
    }
  };

  const reset = async () => {
    try {
      await apiDelete("/api/adjust");
      onOpenChange(false);
      bumpRefresh();
    } catch (e) {
      setMsg("重置失败：" + (e as Error).message);
    }
  };

  const realText = revealed && adj ? fmtFull(adj.realTotal) : "••••••••";

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent open={open} className="max-w-md">
        <DialogHeader>
          <DialogTitle>统计校准</DialogTitle>
          <DialogDescription>按比例调整界面呈现的统计口径，原始记录不受影响，可随时还原</DialogDescription>
        </DialogHeader>
        {adj && (
          <div className="rounded-md bg-muted p-3 text-xs leading-relaxed">
            基准值：
            <b
              className="cursor-pointer select-none text-primary"
              title="点击显示/隐藏"
              onClick={() => setRevealed(!revealed)}
            >
              {realText}
            </b>{" "}
            tokens
            <br />
            {adj.active ? (
              <>
                当前口径：<b>{fmtFull(Math.round(adj.target))}</b> tokens（×{adj.factor.toFixed(3)}）
              </>
            ) : (
              "当前未启用校准。"
            )}
          </div>
        )}
        <div className="flex gap-2">
          <Input
            placeholder={adj?.active ? "修改目标总消耗" : "目标总消耗，如 3B / 30亿 / 500万"}
            value={target}
            onChange={(e) => setTarget(e.target.value)}
            onKeyDown={(e) => e.key === "Enter" && apply()}
          />
          <Button onClick={apply}>应用</Button>
        </div>
        <div className="flex justify-end">
          <Button variant="ghost" size="sm" onClick={reset}>
            ↺ 还原真实数据
          </Button>
        </div>
        {msg && <p className="text-xs text-destructive">{msg}</p>}
      </DialogContent>
    </Dialog>
  );
}
