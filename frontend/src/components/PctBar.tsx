export function PctBar({ pct, label }: { pct: number; label?: string }) {
  return (
    <div className="flex items-center gap-2">
      {label && <span className="w-11 text-xs tabular-nums text-muted-foreground">{label}</span>}
      <div className="h-1.5 flex-1 overflow-hidden rounded-full bg-muted">
        <div
          className="h-full rounded-full bg-primary transition-[width] duration-300"
          style={{ width: `${Math.min(pct, 100)}%` }}
        />
      </div>
    </div>
  );
}
