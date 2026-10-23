import { useEffect, useState } from "react";
import { Panel, PanelGroup, PanelResizeHandle } from "react-resizable-panels";
import { apiGet, type FileStat, type ModelStat, type ProjectStat, type ToolStat, type Totals } from "@/lib/api";
import { fmtFull, fmtNum } from "@/lib/utils";
import { useApp } from "@/store";
import { useT } from "@/i18n";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { PctBar } from "@/components/PctBar";

// 四个排行面板：模型 / 项目 / 工具 / 文件热点，宽度可拖拽调整
export function RankPanels() {
  const t = useT();
  const { range, refreshTick } = useApp();
  const [models, setModels] = useState<ModelStat[]>([]);
  const [projects, setProjects] = useState<ProjectStat[]>([]);
  const [tools, setTools] = useState<ToolStat[]>([]);
  const [files, setFiles] = useState<FileStat[]>([]);

  useEffect(() => {
    const q = `start=${range.start}&end=${range.end}`;
    apiGet<{ models: ModelStat[]; totals: Totals }>(`/api/models?${q}`)
      .then((r) => setModels(r.models || []))
      .catch(() => {});
    apiGet<ProjectStat[]>(`/api/projects?${q}`).then((r) => setProjects(r || [])).catch(() => {});
    apiGet<ToolStat[]>(`/api/tools?${q}`).then((r) => setTools(r || [])).catch(() => {});
    apiGet<FileStat[]>(`/api/files?${q}`).then((r) => setFiles(r || [])).catch(() => {});
  }, [range, refreshTick]);

  const modelMax = Math.max(...models.map((m) => m.inputTokens + m.outputTokens + m.cacheCreate + m.cacheRead), 1);
  const projMax = Math.max(...projects.map((p) => p.totalTokens), 1);
  const toolMax = Math.max(...tools.map((x) => x.count), 1);
  const fileMax = Math.max(...files.map((f) => f.count), 1);

  return (
    <PanelGroup direction="horizontal" className="gap-0" autoSaveId="rank-panels">
      <Panel defaultSize={26} minSize={16}>
        <Card className="mr-1.5 h-full">
          <CardHeader>
            <CardTitle>{t("rank.models")}</CardTitle>
          </CardHeader>
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t("table.model")}</TableHead>
                  <TableHead className="w-14">{t("table.requests")}</TableHead>
                  <TableHead className="w-16">{t("table.total")}</TableHead>
                  <TableHead className="w-20"></TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {models.map((m) => {
                  const total = m.inputTokens + m.outputTokens + m.cacheCreate + m.cacheRead;
                  return (
                    <TableRow key={m.model}>
                      <TableCell className="max-w-0 truncate font-mono text-xs" title={m.model}>{m.model}</TableCell>
                      <TableCell className="text-xs">{fmtFull(m.requests)}</TableCell>
                      <TableCell className="text-xs">{fmtNum(total)}</TableCell>
                      <TableCell><PctBar pct={(total / modelMax) * 100} /></TableCell>
                    </TableRow>
                  );
                })}
              </TableBody>
            </Table>
          </CardContent>
        </Card>
      </Panel>
      <Handle />
      <Panel defaultSize={25} minSize={16}>
        <Card className="mx-1.5 h-full">
          <CardHeader>
            <CardTitle>{t("rank.projects")}</CardTitle>
          </CardHeader>
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t("rank.project")}</TableHead>
                  <TableHead className="w-14">{t("table.requests")}</TableHead>
                  <TableHead className="w-16">{t("table.total")}</TableHead>
                  <TableHead className="w-20"></TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {projects.map((p) => (
                  <TableRow key={p.project}>
                    <TableCell className="max-w-0 truncate font-mono text-xs" title={p.project}>{p.project}</TableCell>
                    <TableCell className="text-xs">{fmtFull(p.requests)}</TableCell>
                    <TableCell className="text-xs">{fmtNum(p.totalTokens)}</TableCell>
                    <TableCell><PctBar pct={(p.totalTokens / projMax) * 100} /></TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </CardContent>
        </Card>
      </Panel>
      <Handle />
      <Panel defaultSize={24} minSize={16}>
        <Card className="mx-1.5 h-full">
          <CardHeader>
            <CardTitle>{t("rank.tools")}</CardTitle>
          </CardHeader>
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t("rank.tool")}</TableHead>
                  <TableHead className="w-16">{t("rank.count")}</TableHead>
                  <TableHead className="w-14">{t("rank.errors")}</TableHead>
                  <TableHead className="w-20"></TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {tools.map((x) => (
                  <TableRow key={x.tool}>
                    <TableCell className="max-w-0 truncate font-mono text-xs" title={x.tool}>{x.tool}</TableCell>
                    <TableCell className="text-xs">{fmtFull(x.count)}</TableCell>
                    <TableCell className="text-xs">
                      {x.errors > 0 ? (
                        <span
                          className="text-destructive"
                          title={t("rank.errorRate", { pct: ((x.errors / Math.max(x.count, x.errors)) * 100).toFixed(1) })}
                        >
                          {fmtFull(x.errors)}
                        </span>
                      ) : (
                        <span className="text-muted-foreground">0</span>
                      )}
                    </TableCell>
                    <TableCell><PctBar pct={(x.count / toolMax) * 100} /></TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </CardContent>
        </Card>
      </Panel>
      <Handle />
      <Panel defaultSize={25} minSize={16}>
        <Card className="ml-1.5 h-full">
          <CardHeader>
            <CardTitle>{t("rank.files")}</CardTitle>
          </CardHeader>
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t("rank.file")}</TableHead>
                  <TableHead className="w-14">{t("rank.ops")}</TableHead>
                  <TableHead className="w-14">{t("rank.count")}</TableHead>
                  <TableHead className="w-20"></TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {files.map((f) => (
                  <TableRow key={f.path}>
                    <TableCell className="max-w-0 truncate font-mono text-xs" title={f.path}>
                      {shortPath(f.path)}
                    </TableCell>
                    <TableCell className="text-xs text-muted-foreground">{f.ops}</TableCell>
                    <TableCell className="text-xs">{fmtFull(f.count)}</TableCell>
                    <TableCell><PctBar pct={(f.count / fileMax) * 100} /></TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
            {files.length === 0 && (
              <div className="py-6 text-center text-xs text-muted-foreground">{t("rank.noFiles")}</div>
            )}
          </CardContent>
        </Card>
      </Panel>
    </PanelGroup>
  );
}

// 路径太长时只保留末两段
function shortPath(p: string) {
  const parts = p.split(/[\\/]/).filter(Boolean);
  return parts.length > 2 ? "…/" + parts.slice(-2).join("/") : p;
}

function Handle() {
  return (
    <PanelResizeHandle className="w-1.5 rounded-full bg-transparent transition-colors hover:bg-primary/40 data-[resize-handle-state=drag]:bg-primary/60" />
  );
}
