import { useCallback, useEffect, useState } from "react";
import { ChevronLeft, ChevronRight, RefreshCw, Search } from "lucide-react";
import { apiGet, type UsageRow } from "@/lib/api";
import { fmtFull, fmtNum, fmtTs } from "@/lib/utils";
import { useApp } from "@/store";
import { useT } from "@/i18n";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";

const LIMIT = 50;

export function EventsPage() {
  const t = useT();
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
        <CardTitle>{t("events.title", { range: `${range.start} ~ ${range.end}` })}</CardTitle>
        <div className="flex items-center gap-2">
          <Input
            className="h-8 w-36 text-xs"
            placeholder={t("events.modelPh")}
            value={model}
            onChange={(e) => setModel(e.target.value)}
            onKeyDown={(e) => e.key === "Enter" && (setOffset(0), load())}
          />
          <Input
            className="h-8 w-36 text-xs"
            placeholder={t("events.projectPh")}
            value={project}
            onChange={(e) => setProject(e.target.value)}
            onKeyDown={(e) => e.key === "Enter" && (setOffset(0), load())}
          />
          <Button variant="secondary" size="sm" onClick={() => (setOffset(0), load())}>
            <Search /> {t("events.filter")}
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
              <TableHead>{t("events.time")}</TableHead>
              <TableHead>{t("table.model")}</TableHead>
              <TableHead>{t("rank.project")}</TableHead>
              <TableHead>{t("table.input")}</TableHead>
              <TableHead>{t("table.output")}</TableHead>
              <TableHead>{t("table.cacheCreate")}</TableHead>
              <TableHead>{t("table.cacheRead")}</TableHead>
              <TableHead>{t("events.sum")}</TableHead>
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
          <div className="py-10 text-center text-xs text-muted-foreground">{t("events.empty")}</div>
        )}
        <div className="mt-4 flex items-center justify-center gap-4">
          <Button
            variant="secondary"
            size="sm"
            disabled={offset <= 0}
            onClick={() => setOffset(Math.max(0, offset - LIMIT))}
          >
            <ChevronLeft /> {t("events.prev")}
          </Button>
          <span className="text-xs text-muted-foreground">
            {t("events.pager", { page, pages, total: fmtFull(total) })}
          </span>
          <Button variant="secondary" size="sm" disabled={offset + LIMIT >= total} onClick={() => setOffset(offset + LIMIT)}>
            {t("events.next")} <ChevronRight />
          </Button>
        </div>
      </CardContent>
    </Card>
  );
}
