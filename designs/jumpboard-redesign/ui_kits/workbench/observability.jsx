const { Button, Panel, Badge, Input, Switch, Segmented } = DS;

const logRows = [
  ["17:31:08", "info", "配置应用成功，4 条映射"],
  ["17:30:52", "info", "已保存映射「日志收集」"],
  ["17:29:16", "warn", "Redis 缓存健康检查失败：连接超时"],
  ["17:26:04", "info", "转发引擎已启动 · 内置引擎（Go）"],
  ["17:21:47", "error", "配置校验失败：监听端口 9100 已被占用"],
];
function LogsPage({ onAction }) {
  const [tab, setTab] = React.useState("操作日志");
  const [level, setLevel] = React.useState("全部");
  const [query, setQuery] = React.useState("");
  const [readAccess, setReadAccess] = React.useState(true);
  const rows = logRows.filter((row) => (level === "全部" || ({ 错误: "error", 警告: "warn", 信息: "info" })[level] === row[1]) && row[2].includes(query));
  return <div className="page__inner" data-screen-label="日志">
    <div className="toolbar"><div className="log-tabs"><Segmented options={["操作日志", "访问日志"]} value={tab} onChange={setTab} label="日志类型" /></div>{tab === "操作日志" && <Segmented options={["全部", "错误", "警告", "信息"]} value={level} onChange={setLevel} label="日志级别" />}<div className="toolbar__spacer"></div><Button onClick={() => onAction(`已导出${tab}`)}>导出</Button><Button variant="danger-quiet" onClick={() => onAction(`清空${tab}前需要二次确认`)}>清空</Button></div>
    <div className="toolbar"><div className="search"><img src={asset("search")} alt="" /><input className="jb-input" value={query} onChange={(event) => setQuery(event.target.value)} placeholder="搜索当前日志" aria-label="搜索日志" /></div>{tab === "访问日志" && <><div className="toolbar__spacer"></div><span className="toolbar-caption">读取访问日志</span><Switch checked={readAccess} onChange={setReadAccess} label="读取访问日志" /><Button onClick={() => onAction("AI 将只读取日志末尾 200 行进行解读")}>AI 解读</Button></>}</div>
    <Panel className="log-panel">
      <div className="log-list">
        {tab === "操作日志" && rows.map((row, index) => <div className="log-line" key={index}><time className="mono">{row[0]}</time><Badge>{row[1]}</Badge><span>{row[2]}</span></div>)}
        {tab === "访问日志" && readAccess && [1,2,3,4,5].map((index) => <div className="log-line" key={index}><time className="mono">17:31:{12 + index}</time><Badge protocol="HTTP">GET</Badge><span className="mono">127.0.0.1 → git.example.com:8080 · 200 · {index * 124} B</span></div>)}
        {tab === "访问日志" && !readAccess && <div className="mapping-empty">访问日志读取已关闭。开启上方开关后显示日志内容。</div>}
      </div>
      <footer className="panel-footer"><span>{tab} · 最新记录在前</span><span className="toolbar__spacer"></span><span>内容可选中复制</span></footer>
    </Panel>
  </div>;
}
const snapshots = [
  { id: "current", time: "2026-10-29 17:31", note: "应用配置 · 5 条规则 · 当前版本", added: 1, removed: 0 },
  { id: "previous", time: "2026-10-29 16:48", note: "修改 MySQL 上游 · 4 条规则", added: 2, removed: 2 },
  { id: "first", time: "2026-10-28 09:12", note: "首次启动 · 4 条规则", added: 4, removed: 0 },
];
function HistoryPage({ onAction }) {
  const [selected, setSelected] = React.useState("current");
  const snapshot = snapshots.find((item) => item.id === selected);
  return <div className="page__inner" data-screen-label="配置历史">
    <div className="toolbar"><span className="toolbar-caption">每次成功应用自动保存，可回看差异或回滚</span><div className="toolbar__spacer"></div><Button onClick={() => onAction("已导出当前快照对应的 nginx.conf")}>导出配置</Button></div>
    <Panel className="history-layout">
      <div className="snapshot-list">{snapshots.map((item) => <button className="snapshot-item" aria-pressed={selected === item.id} key={item.id} onClick={() => setSelected(item.id)}><span className="snapshot-item__time">{item.time}</span><span className="snapshot-item__note">{item.note}</span>{item.id === "current" && <Badge>当前生效</Badge>}</button>)}</div>
      <div className="snapshot-detail"><div className="snapshot-detail__head"><div><h2 className="section-title">快照内容</h2><span className="rule-meta">{snapshot.time}</span></div><div className="snapshot-detail__actions"><Badge>+{snapshot.added} 行</Badge>{snapshot.removed > 0 && <Badge>−{snapshot.removed} 行</Badge>}<Button variant={snapshot.id === "current" ? "secondary" : "danger-quiet"} disabled={snapshot.id === "current"} onClick={() => onAction("回滚会先校验配置，再替换当前生效版本")}>回滚到此版本</Button></div></div><pre className="config-code">{`# JumpBoard · ${snapshot.time}\nworker_processes 1;\n\nstream {\n  upstream mysql_internal {\n    server 192.168.1.10:3306;\n  }\n\n  server {\n    listen 127.0.0.1:9000;\n    proxy_pass mysql_internal;\n  }\n`}<span className="code-add">{`\n  server {\n    listen 127.0.0.1:5140 udp;\n    proxy_pass 10.0.1.50:514;\n  }`}</span>{`\n}`}</pre></div>
    </Panel>
  </div>;
}
Object.assign(window, { LogsPage, HistoryPage });
