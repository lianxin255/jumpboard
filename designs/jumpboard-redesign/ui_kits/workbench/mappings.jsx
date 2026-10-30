const { Button, IconButton, Panel, Badge, Status, Input, Switch, Segmented, Notice, Dialog, DataTable } = DS;

function MappingsPage({ mappings, setMappings, dirty, setDirty, onAction, onNewMapping, onEditMapping, onDeleteMapping, onChat, onSettings }) {
  const [query, setQuery] = React.useState("");
  const [filter, setFilter] = React.useState("全部");
  const [group, setGroup] = React.useState("全部分组");
  const [scene, setScene] = React.useState("默认");
  const [detailId, setDetailId] = React.useState(null);
  const shown = mappings.filter((mapping) => (filter === "全部" || (filter === "已启用" ? mapping.enabled : !mapping.enabled)) && (group === "全部分组" || mapping.group === group) && `${mapping.name} ${mapping.local} ${mapping.upstream}`.toLowerCase().includes(query.toLowerCase()));
  const detail = mappings.find((mapping) => mapping.id === detailId);
  const toggleMapping = (id, enabled) => { setMappings(mappings.map((mapping) => mapping.id === id ? { ...mapping, enabled } : mapping)); setDirty(true); };
  return <div className="page__inner" data-screen-label="映射工作台">
    <div className="toolbar">
      <select className="scene-select" value={scene} onChange={(event) => setScene(event.target.value)} aria-label="当前场景"><option>默认</option><option>开发环境</option></select>
      <Button variant="quiet" onClick={onSettings}>管理场景</Button>
      <div className="toolbar__spacer"></div>
      <Button onClick={() => onAction("模板选择：MySQL、Redis、MongoDB、PostgreSQL、MQTT、RDP、SSH、WebSocket 等")}>从模板新建</Button>
      <Button onClick={() => onAction("粘贴导入会预览变更，逐条校验后才会写入")}>粘贴导入</Button>
    </div>
    <div className="toolbar">
      <div className="search"><img src={asset("search")} alt="" /><input className="jb-input" value={query} onChange={(event) => setQuery(event.target.value)} placeholder="搜索名称、端口或上游地址" aria-label="搜索映射" /></div>
      <Segmented options={["全部", "已启用", "已停用"]} value={filter} onChange={setFilter} label="筛选映射" />
      <div className="toolbar__spacer"></div>
      <Button variant="quiet" icon={asset("chat")} onClick={onChat}>AI 建映射</Button>
      <Button variant="quiet" onClick={() => onAction("AI 筛选只发送映射摘要，不包含密钥")}>AI 筛选</Button>
    </div>
    <div className="toolbar toolbar--chips">
      <div className="group-chips" role="group" aria-label="按分组筛选">
        {["全部", ...new Set(mappings.map((mapping) => mapping.group))].map((item) => <button key={item} type="button" className="group-chip" aria-pressed={(item === "全部" && group === "全部分组") || item === group} onClick={() => setGroup(item === "全部" ? "全部分组" : item)}>{item}</button>)}
      </div>
    </div>
    {dirty && <div className="pending-notice"><Notice><strong>有未生效的改动。</strong> 当前列表已保存，点击右上角「应用配置」后对外生效。</Notice></div>}
    <Panel className="mapping-panel">
      <DataTable label="端口转发规则" columns={[{ key: "rule", label: "规则", width: "21%" }, { key: "protocol", label: "协议", width: "72px" }, { key: "forward", label: "转发关系" }, { key: "health", label: "健康", width: "92px" }, { key: "enabled", label: "启用", width: "62px" }, { key: "actions", label: "", width: "76px" }]}>
        {shown.map((mapping) => <tr key={mapping.id} className={detailId === mapping.id ? "is-selected" : ""}>
          <td><button className="rule-name" onClick={() => setDetailId(detailId === mapping.id ? null : mapping.id)}>{mapping.name}</button><span className="rule-meta">{mapping.group}</span></td>
          <td><Badge protocol={mapping.protocol}>{mapping.protocol}</Badge></td>
          <td><div className="forward-relation"><div className="forward-relation__row mono" title={`${mapping.local} → ${mapping.upstream}`}><span>{mapping.local}</span><span className="forward-relation__arrow">→</span><span>{mapping.upstream}</span></div><span className="forward-relation__traffic">{mapping.traffic}</span></div></td>
          <td><Status tone={mapping.tone}>{mapping.health}</Status></td>
          <td><Switch checked={mapping.enabled} onChange={(enabled) => toggleMapping(mapping.id, enabled)} label={`启用-${mapping.name}`} /></td>
          <td><div className="row-actions"><IconButton icon={asset("edit")} label={`编辑-${mapping.name}`} onClick={() => onEditMapping(mapping)} /><IconButton icon={asset("delete")} label={`删除-${mapping.name}`} danger onClick={() => onDeleteMapping(mapping)} /></div></td>
        </tr>)}
      </DataTable>
      {shown.length === 0 && <div className="mapping-empty">没有符合当前搜索和筛选条件的映射。</div>}
      <footer className="panel-footer"><span>当前场景「{scene}」 · 显示 {shown.length} / {mappings.length} 条规则</span><div className="panel-footer__actions"><Button variant="quiet" onClick={() => { setMappings(mappings.map((mapping) => ({ ...mapping, enabled: true }))); setDirty(true); }}>全部启用</Button><Button variant="quiet" onClick={() => { setMappings(mappings.map((mapping) => ({ ...mapping, enabled: false }))); setDirty(true); }}>全部停用</Button></div></footer>
    </Panel>
    {detail && <Panel className="detail-drawer">
      <div className="detail-drawer__header"><h2 className="section-title">{detail.name}</h2><Badge protocol={detail.protocol}>{detail.protocol}</Badge><span className="toolbar__spacer"></span><Status tone={detail.enabled ? "success" : "neutral"}>{detail.enabled ? "已启用" : "已停用"}</Status><IconButton icon={asset("unchecked")} label="关闭详情" onClick={() => setDetailId(null)} /></div>
      <div className="detail-drawer__body"><div className="detail-metric"><small>本地监听</small><strong className="mono">{detail.local}</strong></div><div className="detail-metric"><small>上游目标</small><strong className="mono">{detail.upstream}</strong></div><div className="detail-metric"><small>连接与流量</small><strong>{detail.traffic}</strong></div></div>
      <div className="detail-drawer__footer"><Button onClick={() => onAction(`已复制 ${detail.local}`)}>复制转发地址</Button><Button onClick={() => onAction("已创建规则副本")}>克隆规则</Button><Button onClick={() => onEditMapping(detail)}>编辑规则</Button></div>
    </Panel>}
  </div>;
}
function MappingDialog({ mapping, onClose, onSave, onApply }) {
  const [name, setName] = React.useState(mapping?.name || "");
  const [port, setPort] = React.useState(mapping?.local.split(":").pop() || "9000");
  const [host, setHost] = React.useState(mapping?.upstream.split(":").slice(0, -1).join(":") || "192.168.1.10");
  const [upstreamPort, setUpstreamPort] = React.useState(mapping?.upstream.split(":").pop() || "3306");
  const [protocol, setProtocol] = React.useState(mapping?.protocol || "TCP");
  const [error, setError] = React.useState("");
  const validate = () => {
    if (!name.trim()) return "请填写规则名称。";
    if (![port, upstreamPort].every((value) => Number(value) >= 1 && Number(value) <= 65535)) return "端口需为 1–65535 之间的整数。";
    if (!host.trim()) return "请填写上游地址。";
    return "";
  };
  const save = (apply) => { const issue = validate(); if (issue) { setError(issue); return; } onSave({ id: mapping?.id || Date.now(), name, protocol, local: `127.0.0.1:${port}`, upstream: `${host}:${upstreamPort}`, health: "未检查", tone: "neutral", enabled: true, group: mapping?.group || "未分组", traffic: "0 连接 · 0 B" }, apply); };
  return <Dialog title={mapping ? "编辑映射" : "新建映射"} onClose={onClose} footer={<><span className="dialog-form-footer-note">保存只入库；应用配置后生效</span><Button onClick={onClose}>取消</Button><Button onClick={() => save(false)}>保存</Button><Button variant="primary" onClick={() => save(true)}>保存并应用</Button></>}>
    <div className="jb-field-row"><Input id="mapping-name" label="名称" value={name} onChange={(event) => setName(event.target.value)} placeholder="例如：内网 MySQL" /><Input id="mapping-group" label="分组（可选）" defaultValue={mapping?.group || ""} placeholder="例如：数据库" /></div>
    <div className="dialog-protocol"><span className="text-secondary">协议</span><Segmented options={["TCP", "UDP", "HTTP", "HTTPS"]} value={protocol} onChange={setProtocol} label="映射协议" /></div>
    <div className="jb-section" style={{ marginBottom: 0 }}>
      <Input id="listen-port" label="本地监听端口" value={port} onChange={(event) => setPort(event.target.value)} inputMode="numeric" />
    </div>
    <div className="dialog-form-grid"><Input id="upstream-host" label="上游地址" value={host} onChange={(event) => setHost(event.target.value)} /><Input id="upstream-port" label="上游端口" value={upstreamPort} onChange={(event) => setUpstreamPort(event.target.value)} inputMode="numeric" /></div>
    <div className="dialog-route-preview mono">127.0.0.1:{port || "—"} <span className="forward-relation__arrow">→</span> {host || "—"}:{upstreamPort || "—"}</div>
    <div className="jb-setting-row"><div className="jb-setting-copy"><span>健康检查</span><small>定时探测上游可达性，在列表中显示状态</small></div><Switch checked={true} onChange={() => {}} label="启用健康检查" /></div>
    {error && <Notice tone="danger">{error}</Notice>}
  </Dialog>;
}
Object.assign(window, { MappingsPage, MappingDialog });
