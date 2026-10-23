import { useEffect, useState } from "react";
import { DndContext, closestCenter, type DragEndEvent } from "@dnd-kit/core";
import { SortableContext, useSortable, verticalListSortingStrategy, arrayMove } from "@dnd-kit/sortable";
import { CSS } from "@dnd-kit/utilities";
import { GripVertical } from "lucide-react";
import { motion } from "motion/react";
import { useApp } from "@/store";
import { useT } from "@/i18n";
import { RangeBar } from "@/components/RangeBar";
import { StatCards } from "@/components/StatCards";
import { TrendChart } from "@/components/TrendChart";
import { ModelDetail } from "@/components/ModelDetail";
import { RankPanels } from "@/components/RankPanels";
import { Heatmap } from "@/components/Heatmap";
import { SessionsTable } from "@/components/SessionsTable";
import { EfficiencyPanel } from "@/components/EfficiencyPanel";
import { YearHeatmap } from "@/components/YearHeatmap";

// 可拖拽排序的区块清单，顺序持久化在 localStorage
const SECTION_DEFS = [
  { id: "trend", label: "趋势图" },
  { id: "detail", label: "模型明细" },
  { id: "efficiency", label: "效率分析" },
  { id: "ranks", label: "排行" },
  { id: "heatmap", label: "时段热力图" },
  { id: "yearmap", label: "年度热力" },
  { id: "sessions", label: "会话列表" },
] as const;
type SectionId = (typeof SECTION_DEFS)[number]["id"];

function loadOrder(): SectionId[] {
  try {
    const saved = JSON.parse(localStorage.getItem("dashOrder") || "[]") as SectionId[];
    const valid = saved.filter((id) => SECTION_DEFS.some((d) => d.id === id));
    const missing = SECTION_DEFS.map((d) => d.id).filter((id) => !valid.includes(id));
    return [...valid, ...missing];
  } catch {
    return SECTION_DEFS.map((d) => d.id);
  }
}

export function DashboardPage() {
  const range = useApp((s) => s.range);
  const [order, setOrder] = useState<SectionId[]>(loadOrder);
  // 模型明细的范围：默认跟全局，点趋势图柱子锁定到当天
  const [detailRange, setDetailRange] = useState(range);
  // 全局范围变化时明细解除日期锁定
  useEffect(() => setDetailRange(range), [range]);

  const onDragEnd = (e: DragEndEvent) => {
    const { active, over } = e;
    if (!over || active.id === over.id) return;
    setOrder((prev) => {
      const next = arrayMove(prev, prev.indexOf(active.id as SectionId), prev.indexOf(over.id as SectionId));
      localStorage.setItem("dashOrder", JSON.stringify(next));
      return next;
    });
  };

  const sections: Record<SectionId, React.ReactNode> = {
    trend: <TrendChart onPickDate={(d) => setDetailRange({ start: d, end: d })} />,
    detail: <ModelDetail detailRange={detailRange} />,
    efficiency: <EfficiencyPanel />,
    ranks: <RankPanels />,
    heatmap: <Heatmap />,
    yearmap: <YearHeatmap />,
    sessions: <SessionsTable />,
  };

  return (
    <div className="flex flex-col gap-4">
      <RangeBar />
      <StatCards />
      <DndContext collisionDetection={closestCenter} onDragEnd={onDragEnd}>
        <SortableContext items={order} strategy={verticalListSortingStrategy}>
          {order.map((id) => (
            <SortableSection key={id} id={id}>
              {sections[id]}
            </SortableSection>
          ))}
        </SortableContext>
      </DndContext>
    </div>
  );
}

function SortableSection({ id, children }: { id: string; children: React.ReactNode }) {
  const t = useT();
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({ id });
  return (
    <motion.div
      ref={setNodeRef}
      style={{ transform: CSS.Transform.toString(transform), transition }}
      className={isDragging ? "dnd-dragging" : undefined}
      initial={{ opacity: 0, y: 10 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.25 }}
      layout
    >
      <div className="group/sec relative">
        <button
          {...attributes}
          {...listeners}
          className="absolute -left-1 top-3 z-10 cursor-grab rounded p-0.5 text-muted-foreground/0 transition-colors group-hover/sec:text-muted-foreground hover:!text-foreground active:cursor-grabbing"
          title={t("dashboard.dragTip")}
        >
          <GripVertical className="size-4" />
        </button>
        {children}
      </div>
    </motion.div>
  );
}
