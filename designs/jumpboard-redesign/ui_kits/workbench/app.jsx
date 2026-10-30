const { Button, Dialog } = DS;

function App() {
  const [page, setPageState] = React.useState(() => {
    const requested = new URLSearchParams(window.location.search).get("page");
    return Object.keys(pageMeta).includes(requested) ? requested : "mappings";
  });
  const setPage = (next) => {
    setPageState(next);
    const url = new URL(window.location.href);
    url.searchParams.set("page", next);
    window.history.replaceState({}, "", url);
  };
  const [theme, setTheme] = React.useState("亮色");
  React.useEffect(() => {
    document.documentElement.dataset.theme = theme === "暗色" ? "dark" : "light";
  }, [theme]);
  const [running, setRunning] = React.useState(true);
  const [dirty, setDirty] = React.useState(true);
  const [mappings, setMappings] = React.useState(initialMappings);
  const [mappingDialog, setMappingDialog] = React.useState(null);
  const [confirmStop, setConfirmStop] = React.useState(false);
  const [deleteCandidate, setDeleteCandidate] = React.useState(null);
  const [confirmImport, setConfirmImport] = React.useState(null);
  const [toast, setToast] = React.useState("");
  const notify = (message) => setToast(message);
  React.useEffect(() => { if (!toast) return; const timer = setTimeout(() => setToast(""), 3200); return () => clearTimeout(timer); }, [toast]);
  const saveMapping = (mapping, apply) => {
    setMappings((previous) => previous.some((item) => item.id === mapping.id) ? previous.map((item) => item.id === mapping.id ? mapping : item) : [...previous, mapping]);
    setMappingDialog(null);
    setDirty(!apply);
    notify(apply ? "映射已保存，配置已应用" : "映射已保存，等待应用配置");
  };
  return <div className="prototype-shell">
    <div className="prototype-meta"><strong>JumpBoard / 界面重设计</strong><span className="prototype-meta__rule"></span><span>克制专业 · 原生桌面工作台</span><span className="prototype-meta__spacer"></span><span>方案外观</span><div className="prototype-theme">{["亮色", "暗色"].map((choice) => <button key={choice} aria-pressed={theme === choice} onClick={() => setTheme(choice)}>{choice}</button>)}</div></div>
    <div className="desktop-window">
      <AppSidebar page={page} onPage={setPage} running={running} onRunning={(next) => { if (!next) setConfirmStop(true); else { setRunning(true); notify("转发服务已启动"); } }} />
      <main className="main">
        <WindowBar page={page} onAction={(message) => { if (message.includes("配置已应用")) setDirty(false); notify(message); }} onNewMapping={() => setMappingDialog({ mapping: null })} onClearChat={() => notify("清空前需要确认，避免误删当前对话")} />
        <div className="page">
          {page === "mappings" && <MappingsPage mappings={mappings} setMappings={setMappings} dirty={dirty} setDirty={setDirty} onAction={notify} onNewMapping={() => setMappingDialog({ mapping: null })} onEditMapping={(mapping) => setMappingDialog({ mapping })} onDeleteMapping={setDeleteCandidate} onChat={() => setPage("chat")} onSettings={() => setPage("settings")} />}
          {page === "logs" && <LogsPage onAction={notify} />}
          {page === "history" && <HistoryPage onAction={notify} />}
          {page === "terminal" && <TerminalPage onAction={notify} />}
          {page === "chat" && <ChatPage onAction={notify} onClear={() => notify("清空前需要确认")} onImport={setConfirmImport} />}
          {page === "settings" && <SettingsPage theme={theme} setTheme={setTheme} onAction={notify} />}
        </div>
      </main>
    </div>
    <div className="prototype-notes"><span>交互提案 · 非生产界面 · 不调用外部服务</span><span>六页导航、新建/编辑映射、草稿导入与设置分组均可体验</span></div>
    {mappingDialog && <MappingDialog mapping={mappingDialog.mapping} onClose={() => setMappingDialog(null)} onSave={saveMapping} />}
    {confirmStop && <Dialog title="停止转发服务" onClose={() => setConfirmStop(false)} footer={<><Button onClick={() => setConfirmStop(false)}>取消</Button><Button variant="danger" onClick={() => { setRunning(false); setConfirmStop(false); notify("转发服务已停止，所有转发已失效"); }}>停止服务</Button></>}><p className="body-copy">停止后所有转发规则立即失效，外部无法访问。已有配置会保留，重新启动后可恢复。</p></Dialog>}
    {deleteCandidate && <Dialog title="删除映射" onClose={() => setDeleteCandidate(null)} footer={<><Button onClick={() => setDeleteCandidate(null)}>取消</Button><Button variant="danger" onClick={() => { setMappings(mappings.filter((item) => item.id !== deleteCandidate.id)); setDirty(true); notify(`已删除「${deleteCandidate.name}」`); setDeleteCandidate(null); }}>确认删除</Button></>}><p className="body-copy">「{deleteCandidate.name}」将被删除，需要重新创建。正在运行的配置要点击「应用配置」后才会同步删除。</p></Dialog>}
    {confirmImport && <Dialog title="导入映射草稿" onClose={() => setConfirmImport(null)} footer={<><Button onClick={() => setConfirmImport(null)}>取消</Button><Button variant="primary" onClick={() => { setMappings([...mappings, ...confirmImport.map((draft, index) => ({ ...draft, id: Date.now() + index, enabled: true, health: "未检查", tone: "neutral", group: "AI 导入", traffic: "0 连接 · 0 B" }))]); setDirty(true); setPage("mappings"); notify(`已导入 ${confirmImport.length} 条草稿，等待应用配置`); setConfirmImport(null); }}>导入</Button></>}><p className="body-copy">把 {confirmImport.length} 条草稿写入当前场景，使用与手动新建相同的校验。导入后点击「应用配置」才会生效。</p></Dialog>}
    {toast && <div className="prototype-toast" role="status">{toast}</div>}
  </div>;
}
ReactDOM.createRoot(document.getElementById("root")).render(<App />);
