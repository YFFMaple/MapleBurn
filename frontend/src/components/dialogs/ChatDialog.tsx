import { useEffect, useState } from "react";
import { apiGet, type SessionMessage } from "@/lib/api";
import { fmtFull } from "@/lib/utils";
import { useApp } from "@/store";
import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { cn } from "@/lib/utils";

export function ChatDialog() {
  const { chat, closeChat } = useApp();
  const [msgs, setMsgs] = useState<SessionMessage[] | null>(null);
  const [error, setError] = useState("");

  useEffect(() => {
    if (!chat) return;
    setMsgs(null);
    setError("");
    apiGet<SessionMessage[]>(
      `/api/session-messages?project=${encodeURIComponent(chat.project)}&session=${encodeURIComponent(chat.session)}`
    )
      .then((m) => setMsgs(m || []))
      .catch((e) => setError((e as Error).message));
  }, [chat]);

  return (
    <Dialog open={!!chat} onOpenChange={(o) => !o && closeChat()}>
      <DialogContent open={!!chat} className="flex h-[80vh] max-w-3xl flex-col">
        <DialogHeader>
          <DialogTitle>
            {chat ? `${chat.project} / ${chat.session.slice(0, 8)}…` : "会话内容"}
          </DialogTitle>
        </DialogHeader>
        <div className="flex-1 space-y-3 overflow-y-auto pr-1">
          {error && <div className="py-8 text-center text-xs text-destructive">加载失败:{error}</div>}
          {!error && msgs === null && <div className="py-8 text-center text-xs text-muted-foreground">加载中…</div>}
          {!error && msgs?.length === 0 && (
            <div className="py-8 text-center text-xs text-muted-foreground">这个会话没有可展示的文本消息</div>
          )}
          {msgs?.map((m, i) => (
            <div
              key={i}
              className={cn(
                "max-w-[85%] rounded-lg px-3 py-2 text-sm leading-relaxed whitespace-pre-wrap break-words",
                m.role === "user" ? "ml-auto bg-primary/15" : "bg-muted"
              )}
            >
              {m.text}
              <div className="mt-1 text-[10px] text-muted-foreground">
                {m.ts}
                {m.role === "assistant" && m.model && (
                  <> · {m.model} · 入 {fmtFull(m.inputTokens)} / 出 {fmtFull(m.outputTokens)}</>
                )}
              </div>
            </div>
          ))}
        </div>
      </DialogContent>
    </Dialog>
  );
}
