import { useEffect, useState } from "react";
import { apiGet, type SessionStat } from "@/lib/api";
import { fmtFull, fmtNum } from "@/lib/utils";
import { useApp } from "@/store";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";

export function SessionsTable() {
  const { range, refreshTick, openChat } = useApp();
  const [list, setList] = useState<SessionStat[]>([]);

  useEffect(() => {
    apiGet<SessionStat[]>(`/api/sessions?start=${range.start}&end=${range.end}&limit=50`)
      .then((r) => setList(r || []))
      .catch(() => {});
  }, [range, refreshTick]);

  return (
    <Card>
      <CardHeader>
        <CardTitle>会话列表</CardTitle>
        <span className="text-xs text-muted-foreground">点击行查看会话内容</span>
      </CardHeader>
      <CardContent>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>会话</TableHead>
              <TableHead>项目</TableHead>
              <TableHead>模型</TableHead>
              <TableHead>请求数</TableHead>
              <TableHead>总 tokens</TableHead>
              <TableHead>最后活跃</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {list.map((s) => (
              <TableRow
                key={s.sessionId + s.project}
                className="cursor-pointer"
                onClick={() => openChat(s.project, s.sessionId)}
              >
                <TableCell className="font-mono text-xs" title={s.sessionId}>
                  {s.sessionId.slice(0, 8)}…
                </TableCell>
                <TableCell className="font-mono text-xs">{s.project}</TableCell>
                <TableCell className="max-w-[200px] truncate font-mono text-xs" title={s.models}>
                  {s.models}
                </TableCell>
                <TableCell>{fmtFull(s.requests)}</TableCell>
                <TableCell>{fmtNum(s.totalTokens)}</TableCell>
                <TableCell className="text-xs text-muted-foreground">{s.lastTs}</TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
        {list.length === 0 && (
          <div className="py-8 text-center text-xs text-muted-foreground">该范围没有会话记录</div>
        )}
      </CardContent>
    </Card>
  );
}
