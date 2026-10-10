# aitoken

本地 AI Token 用量统计工具（Windows）。扫描本机 Claude Code 会话日志，统计各模型的 token 消耗与费用，通过本地网页展示。

**纯本地运行**：数据只写入本机 SQLite，唯一的网络请求是手动点击「更新社区价格表」（只下载、不上传）。

## 功能

- **增量扫描**：自动遍历 `~/.claude/projects/**/*.jsonl`，按字节偏移增量解析，每分钟自动扫一次
- **多维度统计**：今日 / 本周 / 本月 / 全部概览，任意日期区间的趋势图、模型明细、项目排行、会话列表
- **会话回看**：点击会话可查看完整的用户/助手消息记录（含每条消息的 token 数）
- **费用折算**：三层价格表 —— 用户自定义 > 社区价格表（[LiteLLM](https://github.com/BerriAI/litellm)，联网拉取后本地缓存）> 内置兜底；改价后可一键重算历史费用
- **显示校准**：可按目标总消耗等比缩放全部显示数字（不改真实记录，随时回滚）
- **多日志目录**：支持添加多个扫描目录（如多个 Claude Code 配置位置）

## 使用

从 Releases 下载 `aitoken.exe`，双击运行即可：自动找可用端口（17820–17840）、打开浏览器，关闭命令行窗口即退出。

数据存放于 `%LOCALAPPDATA%\aitoken\`（数据库、价格表、配置），删除该目录即完全卸载数据。

## 构建

需要 Go 1.26+（前端静态资源已内嵌，无需 Node）：

```sh
go build -o aitoken.exe .
```

## 技术栈

- 后端：Go 标准库 HTTP 服务 + `modernc.org/sqlite`（纯 Go SQLite，无 CGO）
- 前端：原生 HTML/CSS/JS，`go:embed` 内嵌进单文件 exe
- 仅监听 `127.0.0.1`，外部无法访问

## License

[MIT](LICENSE)
