import { useEffect, useState } from "react";
import { apiGet, type SessionStat } from "@/lib/api";
import { fmtFull, fmtNum } from "@/lib/utils";
import { useApp } from "@/store";
import { useT } from "@/i18n";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";

export function SessionsTable() {
  const t = useT();
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
        <CardTitle>{t("sessions.title")}</CardTitle>
        <span className="text-xs text-muted-foreground">{t("sessions.hint")}</span>
      </CardHeader>
      <CardContent>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t("sessions.session")}</TableHead>
              <TableHead>{t("rank.project")}</TableHead>
              <TableHead>{t("table.model")}</TableHead>
              <TableHead>{t("table.requests")}</TableHead>
              <TableHead>{t("sessions.totalTokens")}</TableHead>
              <TableHead>{t("sessions.lastActive")}</TableHead>
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
          <div className="py-8 text-center text-xs text-muted-foreground">{t("sessions.empty")}</div>
        )}
      </CardContent>
    </Card>
  );
}
