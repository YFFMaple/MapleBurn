import { useState } from "react";
import { LayoutDashboard, ScrollText, Radar, Search } from "lucide-react";
import { useApp } from "@/store";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Header } from "@/components/Header";
import { DashboardPage } from "@/pages/DashboardPage";
import { EventsPage } from "@/pages/EventsPage";
import { ProxyPage } from "@/pages/ProxyPage";
import { SearchPage } from "@/pages/SearchPage";
import { ChatDialog } from "@/components/dialogs/ChatDialog";
import { SettingsDialog } from "@/components/dialogs/SettingsDialog";
import { AdjustDialog } from "@/components/dialogs/AdjustDialog";
import { ReportDialog } from "@/components/dialogs/ReportDialog";

export default function App() {
  const { tab, setTab } = useApp();
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [adjustOpen, setAdjustOpen] = useState(false);
  const [reportOpen, setReportOpen] = useState(false);

  return (
    <div className="flex min-h-screen flex-col">
      <Header
        onOpenSettings={() => setSettingsOpen(true)}
        onOpenAdjust={() => setAdjustOpen(true)}
        onOpenReport={() => setReportOpen(true)}
      />

      <div className="border-b px-5 py-2">
        <Tabs value={tab} onValueChange={(v) => setTab(v as typeof tab)}>
          <TabsList>
            <TabsTrigger value="dashboard">
              <LayoutDashboard /> 仪表盘
            </TabsTrigger>
            <TabsTrigger value="events">
              <ScrollText /> 请求流水
            </TabsTrigger>
            <TabsTrigger value="search">
              <Search /> 搜索
            </TabsTrigger>
            <TabsTrigger value="proxy">
              <Radar /> 观测代理
            </TabsTrigger>
          </TabsList>
        </Tabs>
      </div>

      <main className="mx-auto w-full max-w-[1400px] flex-1 p-5">
        {tab === "dashboard" && <DashboardPage />}
        {tab === "events" && <EventsPage />}
        {tab === "search" && <SearchPage />}
        {tab === "proxy" && <ProxyPage />}
      </main>

      <ChatDialog />
      <SettingsDialog open={settingsOpen} onOpenChange={setSettingsOpen} />
      <AdjustDialog open={adjustOpen} onOpenChange={setAdjustOpen} />
      <ReportDialog open={reportOpen} onOpenChange={setReportOpen} />
    </div>
  );
}
