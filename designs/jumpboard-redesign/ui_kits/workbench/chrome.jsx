const { Button, IconButton, Switch, NavItem } = DS;

function AppSidebar({ page, onPage, running, onRunning }) {
  return <aside className="sidebar">
    <div className="sidebar__traffic" aria-hidden="true"><i className="traffic-dot"></i><i className="traffic-dot"></i><i className="traffic-dot"></i></div>
    <div className="brand"><div className="brand__mark"><img src={asset("relay")} alt="" /></div><div><div className="brand__name">JumpBoard</div><div className="brand__subtitle">内网端口转发</div></div></div>
    <nav className="navigation" aria-label="主导航">
      {navigationGroups.map((group) => <div className="navigation__group" key={group.label}><div className="navigation__label">{group.label}</div>{group.items.map((item) => <NavItem key={item.id} icon={asset(item.icon)} label={item.label} count={item.count} active={page === item.id} onClick={() => onPage(item.id)} />)}</div>)}
    </nav>
    <div className="sidebar__bottom">
      <NavItem icon={asset("settings")} label="设置" active={page === "settings"} onClick={() => onPage("settings")} />
      <div className="service-control">
        <div className="service-control__top"><span className="service-control__title"><i className={`service-control__dot ${running ? "" : "is-off"}`}></i>{running ? "服务运行中" : "服务已停止"}</span><Switch checked={running} onChange={onRunning} label="转发服务开关" /></div>
        <div className="service-control__note">内置引擎 · {running ? "4 条规则监听中" : "所有转发已停止"}</div>
      </div>
    </div>
  </aside>;
}
function WindowBar({ page, onAction, onNewMapping, onClearChat }) {
  const [title, subtitle] = pageMeta[page];
  return <header className="window-bar">
    <div className="window-bar__title"><h1 className="page-title">{title}</h1><div className="window-bar__subtitle">{subtitle}</div></div>
    <div className="window-bar__actions">
      {page === "mappings" && <><Button onClick={() => onAction("预检通过：端口可用，上游地址可解析")}>预检</Button><Button variant="primary" onClick={() => onAction("配置已应用，转发规则已生效")}>应用配置</Button></>}
      {page === "mappings" && <Button variant="secondary" onClick={onNewMapping}>新建映射</Button>}
      {page === "chat" && <Button variant="quiet" onClick={onClearChat}>清空对话</Button>}
    </div>
  </header>;
}
Object.assign(window, { AppSidebar, WindowBar });
