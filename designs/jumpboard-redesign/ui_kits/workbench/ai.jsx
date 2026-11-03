const { Button, IconButton, Panel, Badge, Notice, Dialog } = DS;

function ChatPage({ onAction, onImport, onClear, onApplyDraft, scene }) {
  const [input, setInput] = React.useState("");
  const [busy, setBusy] = React.useState(false);
  const [clearOpen, setClearOpen] = React.useState(false);
  const [copied, setCopied] = React.useState("");
  const [retrying, setRetrying] = React.useState(false);
  const [demoState, setDemoState] = React.useState("正常流程");
  const [drafts, setDrafts] = React.useState([
    { id: "modify-mysql", kind: "修改", name: "MySQL 内网", protocol: "TCP", explanation: "保留原上游，仅调整本地监听端口，避免与现有服务冲突。", changes: [{ field: "监听端口", before: "127.0.0.1:9000", after: "127.0.0.1:9002" }] },
    { id: "new-redis", kind: "新增", name: "内网 Redis", protocol: "TCP", local: "127.0.0.1:9001", upstream: "192.168.1.11:6379", explanation: "把内网 Redis 暴露到本机 9001 端口。" },
    { id: "conflict-git", kind: "修改", name: "GitLab Web", protocol: "HTTP", conflict: "原规则已在对话后被修改，请重新生成草稿，避免覆盖你的改动。", changes: [{ field: "上游地址", before: "192.168.1.20:80", after: "192.168.1.30:80" }] },
  ].filter((item) => demoState === "规则冲突" ? item.id === "conflict-git" : item.id !== "conflict-git"));
  const send = () => { if (!input.trim() || busy) return; setBusy(true); setInput(""); setTimeout(() => { setBusy(false); onAction("已生成修改建议；原型不会向外部服务发送内容"); }, 850); };
  const retry = () => { if (retrying) return; setRetrying(true); setTimeout(() => { setRetrying(false); onAction("已基于原问题重新生成，没有重复添加用户消息"); }, 900); };
  const copy = (id) => { setCopied(id); onAction("已复制可读回复，不包含草稿 JSON"); setTimeout(() => setCopied(""), 1800); };
  const discard = (id) => setDrafts(drafts.filter((item) => item.id !== id));
  return <div className="page__inner chat-layout" data-screen-label="AI 对话与草稿审阅">
    <Panel className="chat-panel">
      <div className="chat-panel__heading"><div><h2 className="section-title">对话</h2><p>当前场景「{scene}」 · 本地保存最近 50 轮 · 发送最近 12 轮</p></div><div className="chat-panel__actions"><select className="demo-select" value={demoState} onChange={(event) => { setDemoState(event.target.value); setDrafts(event.target.value === "规则冲突" ? [{ id: "conflict-git", kind: "修改", name: "GitLab Web", protocol: "HTTP", conflict: "原规则已在对话后被修改，请重新生成草稿，避免覆盖你的改动。", changes: [{ field: "上游地址", before: "192.168.1.20:80", after: "192.168.1.30:80" }] }] : [{ id: "modify-mysql", kind: "修改", name: "MySQL 内网", protocol: "TCP", explanation: "保留原上游，仅调整本地监听端口，避免与现有服务冲突。", changes: [{ field: "监听端口", before: "127.0.0.1:9000", after: "127.0.0.1:9002" }] }, { id: "new-redis", kind: "新增", name: "内网 Redis", protocol: "TCP", local: "127.0.0.1:9001", upstream: "192.168.1.11:6379", explanation: "把内网 Redis 暴露到本机 9001 端口。" }]); }} aria-label="演示状态"><option>正常流程</option><option>请求失败</option><option>规则冲突</option></select><Button disabled={busy} onClick={() => setClearOpen(true)}>清空对话</Button></div></div>
      <div className="conversation">
        <div className="chat-message chat-message--user"><div className="chat-message__body"><span className="chat-message__role">你</span><p>把 MySQL 的监听端口改成 9002，其他配置不动。</p><div className="chat-message__tools"><button onClick={() => onAction("已复制原问题")}>复制</button></div></div></div>
        <div className="chat-message"><div className="chat-message__avatar"><img src={asset("chat")} alt="" /></div><div className="chat-message__body"><span className="chat-message__role">JumpBoard AI</span><p>已找到「MySQL 内网」，只修改监听端口，其他字段保持不变。请在右侧确认差异。</p><p style={{ marginTop: 8 }}>写入草稿后还需要点击「应用配置」才会生效。</p><div className="chat-message__tools"><button onClick={() => copy("reply")}>{copied === "reply" ? "已复制" : "复制"}</button><button disabled={retrying} onClick={retry}>{retrying ? "正在重试…" : "重试"}</button></div></div></div>
        {demoState === "请求失败" && <div className="chat-error"><span>上次请求超时，已保留原问题。</span><Button disabled={retrying} onClick={retry}>{retrying ? "正在重试…" : "重试"}</Button></div>}
      </div>
      <div className="chat-composer"><div className="chat-composer__row"><textarea className="jb-input" aria-label="消息" placeholder="继续修改规则，或描述新的转发需求" value={input} onChange={(event) => setInput(event.target.value)}></textarea><Button variant="primary" disabled={!input.trim() || busy} onClick={send}>{busy ? "回复中…" : "发送"}</Button></div><p className="chat-composer__hint">仅发送当前场景规则摘要和所选轮数，不包含 API Key</p></div>
    </Panel>
    <Panel className="draft-panel">
      <div className="draft-panel__heading"><h2 className="section-title">待审草稿 <span className="text-tertiary">{drafts.length}</span></h2><p>逐条审阅，写入后仍需应用配置</p></div>
      <div className="draft-list">{drafts.map((draft) => <div className={`draft-item ${draft.conflict ? "is-conflict" : ""}`} key={draft.id}>
        <div className="draft-item__top"><strong>{draft.name}</strong><span className={`draft-kind draft-kind--${draft.kind === "修改" ? "edit" : "new"}`}>{draft.kind}</span><Badge protocol={draft.protocol}>{draft.protocol}</Badge><div className="draft-item__actions"><IconButton icon={asset("edit")} label={`填入表单-${draft.name}`} disabled={!!draft.conflict} onClick={() => onAction("已填入普通映射表单，保存前可修改")} /><IconButton icon={asset("delete")} label={`丢弃-${draft.name}`} onClick={() => discard(draft.id)} /></div></div>
        {draft.kind === "新增" ? <div className="draft-route mono"><span>监听 {draft.local}</span><span>上游 {draft.upstream}</span></div> : <div className="draft-changes">{draft.changes.map((change) => <div className="draft-change" key={change.field}><span className="draft-change__field">{change.field}</span><div className="draft-change__values mono"><span className="draft-before">{change.before}</span><span className="draft-change__arrow">→</span><span className="draft-after">{change.after}</span></div></div>)}</div>}
        <p>{draft.explanation}</p>{draft.conflict && <Notice tone="danger">{draft.conflict}</Notice>}
      </div>)}{drafts.length === 0 && <div className="mapping-empty">暂无待审草稿。继续对话可以生成新的映射或修改建议。</div>}</div>
      <div className="draft-panel__footer"><span className="toolbar-caption">写入当前场景「{scene}」 · 不会自动生效</span><Button variant="primary" disabled={drafts.length === 0 || drafts.some((item) => item.conflict)} onClick={() => { onApplyDraft(drafts); setDrafts([]); }}>审阅并写入 {drafts.length > 0 ? `${drafts.length} 条` : ""}草稿</Button></div>
    </Panel>
    {clearOpen && <Dialog title="清空当前场景对话" onClose={() => setClearOpen(false)} footer={<><Button onClick={() => setClearOpen(false)}>取消</Button><Button variant="primary" onClick={() => { setClearOpen(false); setDrafts([]); onClear(); }}>清空对话</Button></>}><p className="body-copy">仅清除「{scene}」的对话和待审草稿，已保存的映射、其他场景和配置历史不受影响。</p></Dialog>}
  </div>;
}
Object.assign(window, { ChatPage });
