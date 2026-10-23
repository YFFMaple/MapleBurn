import { useEffect, useState } from "react";
import { apiDelete, apiGet, apiPost, type Adjustment } from "@/lib/api";
import { fmtFull } from "@/lib/utils";
import { useApp } from "@/store";
import { useT } from "@/i18n";
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";

export function AdjustDialog({ open, onOpenChange }: { open: boolean; onOpenChange: (o: boolean) => void }) {
  const t = useT();
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
      setMsg(t("adjust.setFail", { msg: (e as Error).message }));
    }
  };

  const reset = async () => {
    try {
      await apiDelete("/api/adjust");
      onOpenChange(false);
      bumpRefresh();
    } catch (e) {
      setMsg(t("adjust.resetFail", { msg: (e as Error).message }));
    }
  };

  const realText = revealed && adj ? fmtFull(adj.realTotal) : "••••••••";

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent open={open} className="max-w-md">
        <DialogHeader>
          <DialogTitle>{t("adjust.title")}</DialogTitle>
          <DialogDescription>{t("adjust.desc")}</DialogDescription>
        </DialogHeader>
        {adj && (
          <div className="rounded-md bg-muted p-3 text-xs leading-relaxed">
            {t("adjust.baseline")}
            <b
              className="cursor-pointer select-none text-primary"
              title={t("adjust.revealTip")}
              onClick={() => setRevealed(!revealed)}
            >
              {realText}
            </b>{" "}
            tokens
            <br />
            {adj.active ? (
              t("adjust.currentFactor", { target: fmtFull(Math.round(adj.target)), factor: adj.factor.toFixed(3) })
            ) : (
              t("adjust.inactive")
            )}
          </div>
        )}
        <div className="flex gap-2">
          <Input
            placeholder={adj?.active ? t("adjust.placeholderActive") : t("adjust.placeholder")}
            value={target}
            onChange={(e) => setTarget(e.target.value)}
            onKeyDown={(e) => e.key === "Enter" && apply()}
          />
          <Button onClick={apply}>{t("adjust.apply")}</Button>
        </div>
        <div className="flex justify-end">
          <Button variant="ghost" size="sm" onClick={reset}>
            {t("adjust.reset")}
          </Button>
        </div>
        {msg && <p className="text-xs text-destructive">{msg}</p>}
      </DialogContent>
    </Dialog>
  );
}
