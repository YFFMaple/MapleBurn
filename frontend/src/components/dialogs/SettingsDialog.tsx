import { useEffect, useState } from "react";
import { Plus, Trash2, Copy } from "lucide-react";
import { apiDelete, apiGet, apiPost, type LogDirInfo, type ScanResult } from "@/lib/api";
import { fmtFull } from "@/lib/utils";
import { useApp } from "@/store";
import { useT } from "@/i18n";
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Switch } from "@/components/ui/switch";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";

// Claude Code 里注册 MapleBurn 为 MCP 工具源的配置
const MCP_SNIPPET = `claude mcp add --transport http mapleburn ${location.origin}/mcp`;

export function SettingsDialog({ open, onOpenChange }: { open: boolean; onOpenChange: (o: boolean) => void }) {
  const t = useT();
  const bumpRefresh = useApp((s) => s.bumpRefresh);
  const [dirs, setDirs] = useState<LogDirInfo[]>([]);
  const [input, setInput] = useState("");
  const [msg, setMsg] = useState("");
  const [rebuilding, setRebuilding] = useState(false);
  const [autostart, setAutostart] = useState(false);
  const [copied, setCopied] = useState(false);

  const load = async () => {
    try {
      const res = await apiGet<{ dirs: LogDirInfo[] }>("/api/logdirs");
      setDirs(res.dirs || []);
    } catch {
      /* ignore */
    }
  };

  useEffect(() => {
    if (open) {
      setMsg("");
      load();
      apiGet<{ enabled: boolean }>("/api/autostart").then((r) => setAutostart(r.enabled)).catch(() => {});
    }
  }, [open]);

  const scan = () =>
    apiPost("/api/scan").then(bumpRefresh).catch(() => {});

  const add = async () => {
    const path = input.trim();
    if (!path) return;
    try {
      await apiPost("/api/logdirs", { path });
      setInput("");
      await load();
      scan();
    } catch (e) {
      setMsg((e as Error).message);
    }
  };

  const remove = async (path: string) => {
    try {
      await apiDelete(`/api/logdirs?path=${encodeURIComponent(path)}`);
      await load();
      scan();
    } catch (e) {
      setMsg((e as Error).message);
    }
  };

  const rescan = async () => {
    if (!confirm(t("settings.rescanConfirm"))) return;
    setRebuilding(true);
    setMsg("");
    try {
      const res = await apiPost<ScanResult>("/api/rescan");
      setMsg(t("settings.rescanDone", { n: fmtFull(res.newRecords) }));
      bumpRefresh();
    } catch (e) {
      setMsg(t("settings.rescanFail", { msg: (e as Error).message }));
    } finally {
      setRebuilding(false);
    }
  };

  const toggleAutostart = async (v: boolean) => {
    try {
      const r = await apiPost<{ enabled: boolean }>("/api/autostart", { enabled: v });
      setAutostart(r.enabled);
    } catch (e) {
      setMsg(t("settings.fail", { msg: (e as Error).message }));
    }
  };

  const copyMcp = async () => {
    await navigator.clipboard.writeText(MCP_SNIPPET);
    setCopied(true);
    setTimeout(() => setCopied(false), 1500);
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent open={open} className="max-w-2xl">
        <DialogHeader>
          <DialogTitle>{t("settings.title")}</DialogTitle>
          <DialogDescription>
            {t("settings.desc")}
          </DialogDescription>
        </DialogHeader>
        <div className="flex gap-2">
          <Input
            placeholder={t("settings.addPh")}
            value={input}
            onChange={(e) => setInput(e.target.value)}
            onKeyDown={(e) => e.key === "Enter" && add()}
          />
          <Button onClick={add}>
            <Plus /> {t("settings.add")}
          </Button>
        </div>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t("settings.dir")}</TableHead>
              <TableHead className="w-20">{t("settings.status")}</TableHead>
              <TableHead className="w-16"></TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {dirs.map((d) => (
              <TableRow key={d.path}>
                <TableCell className="max-w-0 truncate font-mono text-xs" title={d.path}>
                  {d.path}
                </TableCell>
                <TableCell className="text-xs">
                  {d.exists ? <span className="text-success">{t("settings.exists")}</span> : <span className="text-destructive">{t("settings.missing")}</span>}
                </TableCell>
                <TableCell>
                  {dirs.length > 1 && (
                    <Button variant="ghost" size="sm" onClick={() => remove(d.path)}>
                      <Trash2 />
                    </Button>
                  )}
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>

        <div className="flex items-center justify-between rounded-lg border p-3">
          <div>
            <div className="text-sm font-medium">{t("settings.autostart")}</div>
            <div className="text-xs text-muted-foreground">{t("settings.autostartDesc")}</div>
          </div>
          <Switch checked={autostart} onCheckedChange={toggleAutostart} />
        </div>

        <div className="rounded-lg border p-3">
          <div className="mb-1 text-sm font-medium">{t("settings.mcp")}</div>
          <p className="mb-2 text-xs text-muted-foreground">
            {t("settings.mcpDesc")}
          </p>
          <div className="flex items-center gap-2">
            <code className="flex-1 overflow-x-auto rounded bg-muted px-2 py-1.5 font-mono text-xs whitespace-nowrap">
              {MCP_SNIPPET}
            </code>
            <Button variant="ghost" size="sm" onClick={copyMcp}>
              <Copy /> {copied ? t("settings.copied") : t("settings.copy")}
            </Button>
          </div>
        </div>

        <div className="flex items-center justify-between border-t pt-3">
          <span className="text-xs text-muted-foreground">{t("settings.rescanHint")}</span>
          <Button variant="secondary" size="sm" onClick={rescan} disabled={rebuilding}>
            {rebuilding ? t("settings.rebuilding") : t("settings.rescan")}
          </Button>
        </div>
        {msg && <p className="text-xs text-primary">{msg}</p>}
      </DialogContent>
    </Dialog>
  );
}
