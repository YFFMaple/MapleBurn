# MapleBurn

本地 AI 用量统计工具（Windows）。扫描本机 Claude Code 会话日志，统计各模型的 token 消耗，通过本地网页展示。

**纯本地运行**：数据只写入本机 SQLite；观测代理只在开启时转发你指向它的流量，不会主动联网。

## 功能

- **增量扫描**：自动遍历 `~/.claude/projects/**/*.jsonl`，按字节偏移增量解析，每分钟自动扫一次
- **多维度统计**：今日 / 本周 / 本月 / 全部概览，任意日期区间的趋势图、模型明细、项目排行、会话列表
- **会话全文搜索**：FTS5 索引消息文本，中英混合子串匹配，点击结果直达会话
- **工具使用统计**：按天+项目+工具聚合调用次数与失败次数（Bash/Read/Edit…）
- **文件热点**：AI 读/写/编辑最多的文件排行
- **效率分析**：缓存命中率、平均每请求 token，指导实际省钱
- **热力图**：周一到周日 × 24 小时分布 + GitHub 风格年度贡献图
- **请求流水**：逐条请求记录，支持模型/项目筛选与分页，点行直达会话内容
- **周期报告**：周报/月报，环比涨跌 + Top 模型，一键复制文本版
- **观测代理（开关）**：本地透明代理，只记录网络层请求（状态码、延迟、各服务商 token），不修改任何内容；开关自动修改/还原 Claude Code 的 `ANTHROPIC_BASE_URL`（原值备份，退出程序自动还原）；服务商注册表支持在线下载更新
- **MCP server**：把统计数据暴露为 MCP 工具，在 Claude Code 里直接问「我本月用了多少 token」
- **系统托盘 + 开机自启**：托盘图标常驻，悬停显示今日用量，可注册 Windows 开机自启
- **桌面小组件**：WebView2 常驻小窗（托盘菜单开关），今日总量 / 缓存命中率 / 逐小时趋势 / 代理状态一屏实时滚动，支持置顶与位置记忆，与 Web 面板同源数据
- **多语言**：界面中英文一键切换，默认跟随系统语言
- **会话回看**：点击会话可查看完整的用户/助手消息记录（含每条消息的 token 数）
- **显示校准**：可按目标总消耗等比缩放全部显示数字（不改真实记录，随时回滚）
- **多日志目录**：支持添加多个扫描目录（如多个 Claude Code 配置位置）

## 使用

从 Releases 下载 `mapleburn.exe`，双击运行即可：自动找可用端口（17820–17840）、打开浏览器，托盘图标常驻，右键托盘可打开面板或退出。

数据存放于 `%LOCALAPPDATA%\mapleburn\`（数据库、配置、服务商注册表），删除该目录即完全卸载数据。旧版 `aitoken` 的数据目录会在首次启动时自动迁移。

## 构建

需要 Go 1.26+ 和 Node 20+：

```sh
cd frontend && npm install && npm run build && cd ..
go build -o mapleburn.exe .
```

前端构建产物在 `frontend/dist`，由 `go:embed` 内嵌进单文件 exe。

Release 构建（隐藏控制台窗口，只留托盘图标，日志写到数据目录 `mapleburn.log`）：

```sh
go build -ldflags "-H windowsgui" -o mapleburn.exe .
```

## 接入 Claude Code（MCP）

在 Claude Code 里运行（设置弹窗里有一键复制）：

```sh
claude mcp add --transport http mapleburn http://127.0.0.1:17820/mcp
```

之后可以直接问「我上周用了多少 token」「搜一下讨论某某的会话」，Claude Code 会调用 MapleBurn 的数据回答。

## 技术栈

- 后端：Go 标准库 HTTP 服务 + `modernc.org/sqlite`（纯 Go SQLite，无 CGO）
- 前端：React + TypeScript + Vite + Tailwind CSS 4 + shadcn/ui 风格组件，Zustand 状态管理，Motion 动画，Lucide 图标，react-resizable-panels 分栏，dnd-kit 拖拽排序
- 仅监听 `127.0.0.1`，外部无法访问

## License

[MIT](LICENSE)
