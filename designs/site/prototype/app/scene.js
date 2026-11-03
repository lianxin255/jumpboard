(function () {
const DS = window.JumpboardRedesign_2bf643;
const {
  Badge
} = DS;
function ScenePicker({
  scene,
  scenes,
  onSelect,
  onManage
}) {
  const [open, setOpen] = React.useState(false);
  const rootRef = React.useRef(null);
  const listId = "scene-options";
  React.useEffect(() => {
    const onPointer = event => {
      if (rootRef.current && !rootRef.current.contains(event.target)) setOpen(false);
    };
    const onKey = event => {
      if (event.key === "Escape") {
        setOpen(false);
        rootRef.current?.querySelector("button")?.focus();
      }
    };
    document.addEventListener("pointerdown", onPointer);
    document.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("pointerdown", onPointer);
      document.removeEventListener("keydown", onKey);
    };
  }, []);
  return React.createElement("div", {
    className: "scene-picker",
    ref: rootRef
  }, React.createElement("button", {
    className: "scene-trigger",
    type: "button",
    "aria-haspopup": "listbox",
    "aria-expanded": open,
    "aria-controls": listId,
    onClick: () => setOpen(!open)
  }, React.createElement("img", {
    src: asset("relay"),
    alt: ""
  }), React.createElement("span", {
    className: "scene-trigger__label"
  }, "场景"), React.createElement("strong", null, scene), React.createElement("span", {
    className: `scene-chevron ${open ? "is-open" : ""}`,
    "aria-hidden": "true"
  }, "⌄")), React.createElement(Button, {
    variant: "quiet",
    onClick: onManage
  }, "管理场景"), open && React.createElement("div", {
    className: "scene-menu",
    id: listId,
    role: "listbox",
    "aria-label": "选择场景"
  }, React.createElement("div", {
    className: "scene-menu__caption"
  }, "切换整套映射，当前改动会保留"), scenes.map(item => React.createElement("button", {
    key: item.name,
    type: "button",
    role: "option",
    "aria-selected": scene === item.name,
    className: "scene-option",
    onClick: () => {
      setOpen(false);
      onSelect(item.name);
    }
  }, React.createElement("span", {
    className: "scene-option__check"
  }, scene === item.name ? "✓" : ""), React.createElement("span", {
    className: "scene-option__name"
  }, item.name), React.createElement("span", {
    className: "scene-option__count"
  }, item.count, " 条规则"), item.name === scene && React.createElement(Badge, null, "当前")))));
}
Object.assign(window, {
  ScenePicker
});
})();
