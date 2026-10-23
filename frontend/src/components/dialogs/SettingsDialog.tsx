import { useEffect, useState } from "react";
import { Plus, Trash2, Copy } from "lucide-react";
import { apiDelete, apiGet, apiPost, type LogDirInfo, type ScanResult } from "@/lib/api";
import { fmtFull } from "@/lib/utils";
import { useApp } from "@/store";
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Switch } from "@/components/ui/switch";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";

// Claude Code 里注册 MapleBurn 为 MCP 工具源的配置
const MCP_SNIPPET = `claude mcp add --transport http mapleburn ${location.origin}/mcp`;

export function SettingsDialog({ open, onOpenChange }: { open: boolean; onOpenChange: (o: boolean) => void }) {
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
    if (!confirm("将清空已扫描的统计数据并从日志完全重建，确定吗？\n（目录配置不受影响）")) return;
    setRebuilding(true);
    setMsg("");
    try {
      const res = await apiPost<ScanResult>("/api/rescan");
      setMsg(`✓ 已重建 ${fmtFull(res.newRecords)} 条`);
      bumpRefresh();
    } catch (e) {
      setMsg("重建失败：" + (e as Error).message);
    } finally {
      setRebuilding(false);
    }
  };

  const toggleAutostart = async (v: boolean) => {
    try {
      const r = await apiPost<{ enabled: boolean }>("/api/autostart", { enabled: v });
      setAutostart(r.enabled);
    } catch (e) {
      setMsg("设置失败：" + (e as Error).message);
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
          <DialogTitle>设置</DialogTitle>
          <DialogDescription>
            日志目录（自动探测：CLAUDE_CONFIG_DIR 环境变量 + ~/.claude/projects）、启动行为与集成
          </DialogDescription>
        </DialogHeader>
        <div className="flex gap-2">
          <Input
            placeholder="添加目录，如 D:\backup\.claude\projects 或 ~/logs"
            value={input}
            onChange={(e) => setInput(e.target.value)}
            onKeyDown={(e) => e.key === "Enter" && add()}
          />
          <Button onClick={add}>
            <Plus /> 添加
          </Button>
        </div>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>目录</TableHead>
              <TableHead className="w-20">状态</TableHead>
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
                  {d.exists ? <span className="text-success">✓ 存在</span> : <span className="text-destructive">✗ 不存在</span>}
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
            <div className="text-sm font-medium">开机自启</div>
            <div className="text-xs text-muted-foreground">登录 Windows 后自动在后台运行（托盘图标）</div>
          </div>
          <Switch checked={autostart} onCheckedChange={toggleAutostart} />
        </div>

        <div className="rounded-lg border p-3">
          <div className="mb-1 text-sm font-medium">接入 Claude Code（MCP）</div>
          <p className="mb-2 text-xs text-muted-foreground">
            运行下面的命令后，就可以直接在 Claude Code 里问「我本月用了多少 token」，它会调用 MapleBurn 的数据回答
          </p>
          <div className="flex items-center gap-2">
            <code className="flex-1 overflow-x-auto rounded bg-muted px-2 py-1.5 font-mono text-xs whitespace-nowrap">
              {MCP_SNIPPET}
            </code>
            <Button variant="ghost" size="sm" onClick={copyMcp}>
              <Copy /> {copied ? "✓" : "复制"}
            </Button>
          </div>
        </div>

        <div className="flex items-center justify-between border-t pt-3">
          <span className="text-xs text-muted-foreground">数据有缺失或漂移时，可以清空后从日志完全重建</span>
          <Button variant="secondary" size="sm" onClick={rescan} disabled={rebuilding}>
            {rebuilding ? "重建中…" : "↺ 完全重新扫描"}
          </Button>
        </div>
        {msg && <p className="text-xs text-primary">{msg}</p>}
      </DialogContent>
    </Dialog>
  );
}
