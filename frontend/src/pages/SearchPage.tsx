import { useState } from "react";
import { Search as SearchIcon, User, Bot } from "lucide-react";
import { apiGet, type SearchHit } from "@/lib/api";
import { useApp } from "@/store";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/table";

// 把片段里的查询词高亮（大小写不敏感）
function Highlight({ text, query }: { text: string; query: string }) {
  if (!query) return <>{text}</>;
  const parts = text.split(new RegExp(`(${query.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")})`, "gi"));
  return (
    <>
      {parts.map((p, i) =>
        p.toLowerCase() === query.toLowerCase() ? (
          <mark key={i} className="rounded-sm bg-primary/30 px-0.5 text-foreground">
            {p}
          </mark>
        ) : (
          p
        )
      )}
    </>
  );
}

export function SearchPage() {
  const openChat = useApp((s) => s.openChat);
  const [q, setQ] = useState("");
  const [hits, setHits] = useState<SearchHit[] | null>(null);
  const [searching, setSearching] = useState(false);

  const search = async () => {
    const query = q.trim();
    if (!query) return;
    setSearching(true);
    try {
      const res = await apiGet<{ hits: SearchHit[] }>(`/api/search?q=${encodeURIComponent(query)}&limit=50`);
      setHits(res.hits || []);
    } catch {
      setHits([]);
    } finally {
      setSearching(false);
    }
  };

  return (
    <Card>
      <CardHeader>
        <CardTitle>会话全文搜索</CardTitle>
        <span className="text-xs text-muted-foreground">搜索历史会话的消息内容，点击结果直达会话</span>
      </CardHeader>
      <CardContent className="flex flex-col gap-4">
        <div className="flex gap-2">
          <Input
            placeholder="输入关键词，中英文均可，如：热力图 / heatmap / 重构"
            value={q}
            onChange={(e) => setQ(e.target.value)}
            onKeyDown={(e) => e.key === "Enter" && search()}
            autoFocus
          />
          <Button onClick={search} disabled={searching}>
            <SearchIcon /> {searching ? "搜索中…" : "搜索"}
          </Button>
        </div>

        {hits !== null && hits.length === 0 && (
          <div className="py-10 text-center text-xs text-muted-foreground">
            没有找到包含「{q}」的消息
          </div>
        )}
        <div className="flex flex-col gap-2">
          {hits?.map((h, i) => (
            <button
              key={i}
              onClick={() => openChat(h.project, h.sessionId)}
              className="rounded-lg border bg-card p-3 text-left transition-colors hover:border-primary/50 cursor-pointer"
            >
              <div className="mb-1 flex items-center gap-2 text-xs text-muted-foreground">
                {h.role === "user" ? <User className="size-3.5" /> : <Bot className="size-3.5" />}
                <span className="font-mono">{h.project}</span>
                <span>·</span>
                <span className="font-mono">{h.sessionId.slice(0, 8)}…</span>
                <span>·</span>
                <span>{h.ts}</span>
                <Badge variant={h.role === "user" ? "default" : "secondary"}>
                  {h.role === "user" ? "我" : "助手"}
                </Badge>
              </div>
              <div className="line-clamp-3 text-sm leading-relaxed text-muted-foreground">
                <Highlight text={h.text} query={q.trim()} />
              </div>
            </button>
          ))}
        </div>
      </CardContent>
    </Card>
  );
}
