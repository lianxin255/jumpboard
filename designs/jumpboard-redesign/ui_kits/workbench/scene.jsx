const { Badge } = DS;

function ScenePicker({ scene, scenes, onSelect, onManage }) {
  const [open, setOpen] = React.useState(false);
  const rootRef = React.useRef(null);
  const listId = "scene-options";
  React.useEffect(() => {
    const onPointer = (event) => { if (rootRef.current && !rootRef.current.contains(event.target)) setOpen(false); };
    const onKey = (event) => { if (event.key === "Escape") { setOpen(false); rootRef.current?.querySelector("button")?.focus(); } };
    document.addEventListener("pointerdown", onPointer);
    document.addEventListener("keydown", onKey);
    return () => { document.removeEventListener("pointerdown", onPointer); document.removeEventListener("keydown", onKey); };
  }, []);
  return <div className="scene-picker" ref={rootRef}>
    <button className="scene-trigger" type="button" aria-haspopup="listbox" aria-expanded={open} aria-controls={listId} onClick={() => setOpen(!open)}>
      <img src={asset("relay")} alt="" /><span className="scene-trigger__label">场景</span><strong>{scene}</strong><span className={`scene-chevron ${open ? "is-open" : ""}`} aria-hidden="true">⌄</span>
    </button>
    <Button variant="quiet" onClick={onManage}>管理场景</Button>
    {open && <div className="scene-menu" id={listId} role="listbox" aria-label="选择场景">
      <div className="scene-menu__caption">切换整套映射，当前改动会保留</div>
      {scenes.map((item) => <button key={item.name} type="button" role="option" aria-selected={scene === item.name} className="scene-option" onClick={() => { setOpen(false); onSelect(item.name); }}>
        <span className="scene-option__check">{scene === item.name ? "✓" : ""}</span><span className="scene-option__name">{item.name}</span><span className="scene-option__count">{item.count} 条规则</span>{item.name === scene && <Badge>当前</Badge>}
      </button>)}
    </div>}
  </div>;
}
Object.assign(window, { ScenePicker });
