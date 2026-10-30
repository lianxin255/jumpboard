const { Button, IconButton, Panel, Badge, Segmented } = DS;

function TerminalPage({ onAction }) {
  const [newKind, setNewKind] = React.useState("本地终端");
  const [tabs, setTabs] = React.useState([{ id: 1, name: "本地终端", kind: "本地终端" }]);
  const [active, setActive] = React.useState(1);
  const newTab = () => { const id = Date.now(); setTabs([...tabs, { id, name: newKind, kind: newKind }]); setActive(id); };
  const activeTab = tabs.find((tab) => tab.id === active);
  return <div className="page__inner" data-screen-label="终端">
    <div className="toolbar"><Segmented options={["本地终端", "端口探测", "串口"]} value={newKind} onChange={setNewKind} label="新会话类型" /><Button variant="primary" onClick={newTab}>新建会话</Button><div className="toolbar__spacer"></div>{tabs.length > 0 && <Button variant="quiet" onClick={() => setTabs([])}>全部关闭</Button>}</div>
    <Panel className="terminal-workspace">
      <div className="terminal-tabs">{tabs.map((tab) => <div className={`terminal-tab ${tab.id === active ? "is-active" : ""}`} key={tab.id}><button className="terminal-tab__select" aria-pressed={tab.id === active} onClick={() => setActive(tab.id)}>{tab.name}</button><button className="terminal-tab__close" aria-label={`关闭-${tab.name}`} onClick={() => { const remaining = tabs.filter((item) => item.id !== tab.id); setTabs(remaining); if (active === tab.id) setActive(remaining[0]?.id || null); }}>×</button></div>)}{tabs.length > 0 && <button className="terminal-new-tab" aria-label="新建会话" onClick={newTab}>+</button>}</div>
      <div className="terminal-content">
        {!activeTab && <div className="terminal-empty">选择上方会话类型，点击「新建会话」开始。</div>}
        {activeTab?.kind === "本地终端" && <><p className="terminal-muted">JumpBoard 本地终端 · zsh</p><p className="terminal-muted">输入 exit 结束当前会话。</p><br /><p><span className="terminal-prompt">lianxin@jumpboard</span> <span>~</span> % nc -vz 127.0.0.1 9000</p><p>Connection to 127.0.0.1 port 9000 [tcp/*] succeeded!</p><p><span className="terminal-prompt">lianxin@jumpboard</span> <span>~</span> % <span className="terminal-cursor">▋</span></p></>}
        {activeTab?.kind === "端口探测" && <><p className="terminal-muted">探测转发端口 · 不启动 SSH 会话</p><br /><p>目标：MySQL 内网 · 127.0.0.1:9000</p><p className="terminal-prompt">✓ 连接成功 · 12 ms</p><br /><p>最近 5 次探测均成功，上游可达。</p></>}
        {activeTab?.kind === "串口" && <><p className="terminal-muted">串口会话 · /dev/cu.usbserial-0001 · 115200</p><br /><p>等待串口数据…</p></>}
      </div>
      <footer className="terminal-status"><span>{activeTab ? `${activeTab.kind} · 已连接` : "没有活动会话"}</span><span>UTF-8</span><span className="toolbar__spacer"></span><span>本地处理，不上传终端内容</span></footer>
    </Panel>
  </div>;
}
function ChatPage({ onAction, onImport, onClear }) {
  const [input, setInput] = React.useState("");
  const [drafts, setDrafts] = React.useState([{ id: "mysql", name: "内网 MySQL", protocol: "TCP", local: "127.0.0.1:9000", upstream: "192.168.1.10:3306", explanation: "把内网 MySQL 暴露到本机 9000 端口。" }, { id: "redis", name: "内网 Redis", protocol: "TCP", local: "127.0.0.1:9001", upstream: "192.168.1.11:6379", explanation: "把内网 Redis 暴露到本机 9001 端口。" }]);
  const send = () => { if (!input.trim()) return; setInput(""); onAction("消息已加入对话；原型不会向外部服务发送内容"); };
  return <div className="page__inner chat-layout" data-screen-label="AI 对话与草稿审阅">
    <Panel className="chat-panel">
      <div className="chat-panel__heading"><h2 className="section-title">对话</h2><p>AI 只生成草稿，不会直接修改或应用配置</p></div>
      <div className="conversation">
        <div className="chat-message chat-message--user"><div className="chat-message__body"><span className="chat-message__role">你</span><p>把内网 MySQL 和 Redis 分别暴露到 9000/9001</p></div></div>
        <div className="chat-message"><div className="chat-message__avatar"><img src={asset("chat")} alt="" /></div><div className="chat-message__body"><span className="chat-message__role">JumpBoard AI</span><p>已根据你的描述生成两条 TCP 映射。请在右侧检查监听端口和上游地址，单条修改或整批导入。</p><p style={{ marginTop: 8 }}>导入后还需要点击「应用配置」才会生效。</p></div></div>
      </div>
      <div className="chat-composer"><div className="chat-composer__row"><textarea className="jb-input" aria-label="消息" placeholder="描述要转发的服务，或接着上一条继续聊" value={input} onChange={(event) => setInput(event.target.value)}></textarea><Button variant="primary" disabled={!input.trim()} onClick={send}>发送</Button></div><p className="chat-composer__hint">仅发送当前对话；映射列表不会随消息上传</p></div>
    </Panel>
    <Panel className="draft-panel">
      <div className="draft-panel__heading"><h2 className="section-title">待审草稿 <span className="text-tertiary">{drafts.length}</span></h2><p>审阅、修改或丢弃，再导入当前场景</p></div>
      <div className="draft-list">{drafts.map((draft) => <div className="draft-item" key={draft.id}><div className="draft-item__top"><strong>{draft.name}</strong><Badge protocol={draft.protocol}>{draft.protocol}</Badge><div className="draft-item__actions"><IconButton icon={asset("edit")} label={`填入表单-${draft.name}`} onClick={() => onAction("草稿已填入普通映射表单，保存前可修改")} /><IconButton icon={asset("delete")} label={`丢弃-${draft.name}`} onClick={() => setDrafts(drafts.filter((item) => item.id !== draft.id))} /></div></div><div className="draft-route mono"><span>监听 {draft.local}</span><span>上游 {draft.upstream}</span></div><p>{draft.explanation}</p></div>)}{drafts.length === 0 && <div className="mapping-empty">暂无待审草稿。继续对话可以生成新的映射建议。</div>}</div>
      <div className="draft-panel__footer"><span className="toolbar-caption">导入到当前场景「默认」</span><Button variant="primary" disabled={drafts.length === 0} onClick={() => { onImport(drafts); setDrafts([]); }}>导入 {drafts.length > 0 ? `${drafts.length} 条` : ""}草稿</Button></div>
    </Panel>
  </div>;
}
Object.assign(window, { TerminalPage, ChatPage });
