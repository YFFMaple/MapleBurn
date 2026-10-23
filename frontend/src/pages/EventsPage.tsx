import { useCallback, useEffect, useState } from "react";
import { ChevronLeft, ChevronRight, RefreshCw, Search } from "lucide-react";
import { apiGet, type UsageRow } from "@/lib/api";
import { fmtFull, fmtNum, fmtTs } from "@/lib/utils";
import { useApp } from "@/store";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";

const LIMIT = 50;

export function EventsPage() {
  const { range, refreshTick, openChat } = useApp();
  const [model, setModel] = useState("");
  const [project, setProject] = useState("");
  const [offset, setOffset] = useState(0);
  const [events, setEvents] = useState<UsageRow[]>([]);
  const [total, setTotal] = useState(0);

  const load = useCallback(async () => {
    const q = new URLSearchParams({
      start: range.start,
      end: range.end,
      limit: String(LIMIT),
      offset: String(offset),
    });
    if (model.trim()) q.set("model", model.trim());
    if (project.trim()) q.set("project", project.trim());
    try {
      const res = await apiGet<{ events: UsageRow[]; total: number }>(`/api/events?${q}`);
      setEvents(res.events || []);
      setTotal(res.total || 0);
    } catch {
      /* 保持旧数据 */
    }
  }, [range, offset, model, project]);

  useEffect(() => {
    load();
  }, [load, refreshTick]);

  const page = Math.floor(offset / LIMIT) + 1;
  const pages = Math.max(1, Math.ceil(total / LIMIT));

  return (
    <Card>
      <CardHeader>
        <CardTitle>请求流水（{range.start} ~ {range.end}）</CardTitle>
        <div className="flex items-center gap-2">
          <Input
            className="h-8 w-36 text-xs"
            placeholder="模型"
            value={model}
            onChange={(e) => setModel(e.target.value)}
            onKeyDown={(e) => e.key === "Enter" && (setOffset(0), load())}
          />
          <Input
            className="h-8 w-36 text-xs"
            placeholder="项目"
            value={project}
            onChange={(e) => setProject(e.target.value)}
            onKeyDown={(e) => e.key === "Enter" && (setOffset(0), load())}
          />
          <Button variant="secondary" size="sm" onClick={() => (setOffset(0), load())}>
            <Search /> 筛选
          </Button>
          <Button variant="ghost" size="sm" onClick={load}>
            <RefreshCw />
          </Button>
        </div>
      </CardHeader>
      <CardContent>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>时间</TableHead>
              <TableHead>模型</TableHead>
              <TableHead>项目</TableHead>
              <TableHead>输入</TableHead>
              <TableHead>输出</TableHead>
              <TableHead>缓存写</TableHead>
              <TableHead>缓存读</TableHead>
              <TableHead>合计</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {events.map((e) => {
              const sum = e.inputTokens + e.outputTokens + e.cacheCreate + e.cacheRead;
              return (
                <TableRow
                  key={`${e.ts}-${e.sessionId}`}
                  className="cursor-pointer"
                  onClick={() => openChat(e.project, e.sessionId)}
                >
                  <TableCell className="font-mono text-xs whitespace-nowrap">{fmtTs(e.ts)}</TableCell>
                  <TableCell className="font-mono text-xs">{e.model}</TableCell>
                  <TableCell className="font-mono text-xs">{e.project}</TableCell>
                  <TableCell>{fmtFull(e.inputTokens)}</TableCell>
                  <TableCell>{fmtFull(e.outputTokens)}</TableCell>
                  <TableCell>{fmtFull(e.cacheCreate)}</TableCell>
                  <TableCell>{fmtFull(e.cacheRead)}</TableCell>
                  <TableCell>{fmtNum(sum)}</TableCell>
                </TableRow>
              );
            })}
          </TableBody>
        </Table>
        {events.length === 0 && (
          <div className="py-10 text-center text-xs text-muted-foreground">该范围没有记录</div>
        )}
        <div className="mt-4 flex items-center justify-center gap-4">
          <Button
            variant="secondary"
            size="sm"
            disabled={offset <= 0}
            onClick={() => setOffset(Math.max(0, offset - LIMIT))}
          >
            <ChevronLeft /> 上一页
          </Button>
          <span className="text-xs text-muted-foreground">
            第 {page} / {pages} 页 · 共 {fmtFull(total)} 条
          </span>
          <Button variant="secondary" size="sm" disabled={offset + LIMIT >= total} onClick={() => setOffset(offset + LIMIT)}>
            下一页 <ChevronRight />
          </Button>
        </div>
      </CardContent>
    </Card>
  );
}
