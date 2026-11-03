const { Button, IconButton, Switch, NavItem } = DS;

function AppSidebar({ page, onPage, running, onRunning, serviceBusy }) {
  return <aside className="sidebar">
    <div className="sidebar__traffic" aria-hidden="true"><i className="traffic-dot"></i><i className="traffic-dot"></i><i className="traffic-dot"></i></div>
    <div className="brand"><div className="brand__mark"><img src={asset("relay")} alt="" /></div><div><div className="brand__name">JumpBoard</div><div className="brand__subtitle">内网端口转发</div></div></div>
    <nav className="navigation" aria-label="主导航">
      {navigationGroups.map((group) => <div className="navigation__group" key={group.label}><div className="navigation__label">{group.label}</div>{group.items.map((item) => <NavItem key={item.id} icon={asset(item.icon)} label={item.label} count={item.count} active={page === item.id} onClick={() => onPage(item.id)} />)}</div>)}
    </nav>
    <div className="sidebar__bottom">
      <NavItem icon={asset("settings")} label="设置" active={page === "settings"} onClick={() => onPage("settings")} />
      <div className="service-control">
        <div className="service-control__top"><span className="service-control__title"><i className={`service-control__dot ${running ? "" : "is-off"}`}></i>{serviceBusy ? "正在切换…" : running ? "服务运行中" : "服务已停止"}</span><Switch checked={running} disabled={serviceBusy} onChange={onRunning} label="转发服务开关" /></div>
        <div className="service-control__note">内置引擎 · {running ? "4 条规则监听中" : "所有转发已停止"}</div>
      </div>
    </div>
  </aside>;
}
function WindowBar({ page, onAction, onNewMapping, actionState }) {
  const [title, subtitle] = pageMeta[page];
  const busy = actionState === "checking" || actionState === "applying";
  return <header className="window-bar">
    <div className="window-bar__title"><h1 className="page-title">{title}</h1><div className="window-bar__subtitle">{subtitle}</div></div>
    <div className="window-bar__actions">
      {actionState === "checking" && <span className="action-status">正在检查端口与上游</span>}
      {actionState === "applying" && <span className="action-status">正在应用配置</span>}
      {actionState === "checked" && <span className="action-status action-status--success">预检通过</span>}
      {page === "mappings" && <><Button disabled={busy} onClick={() => onAction("pre-check")}>预检</Button><Button variant="primary" disabled={busy} onClick={() => onAction("apply")}>{actionState === "applying" ? "应用中…" : "应用配置"}</Button></>}
      {page === "mappings" && <Button variant="secondary" disabled={busy} onClick={onNewMapping}>新建映射</Button>}
    </div>
  </header>;
}
Object.assign(window, { AppSidebar, WindowBar });
