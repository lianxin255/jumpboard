(function () {
const DS = window.JumpboardRedesign_2bf643;
const {
  Button,
  IconButton,
  Switch,
  NavItem
} = DS;
function AppSidebar({
  page,
  onPage,
  running,
  onRunning,
  serviceBusy
}) {
  return React.createElement("aside", {
    className: "sidebar"
  }, React.createElement("div", {
    className: "sidebar__traffic",
    "aria-hidden": "true"
  }, React.createElement("i", {
    className: "traffic-dot"
  }), React.createElement("i", {
    className: "traffic-dot"
  }), React.createElement("i", {
    className: "traffic-dot"
  })), React.createElement("div", {
    className: "brand"
  }, React.createElement("div", {
    className: "brand__mark"
  }, React.createElement("img", {
    src: asset("relay"),
    alt: ""
  })), React.createElement("div", null, React.createElement("div", {
    className: "brand__name"
  }, "JumpBoard"), React.createElement("div", {
    className: "brand__subtitle"
  }, "内网端口转发"))), React.createElement("nav", {
    className: "navigation",
    "aria-label": "主导航"
  }, navigationGroups.map(group => React.createElement("div", {
    className: "navigation__group",
    key: group.label
  }, React.createElement("div", {
    className: "navigation__label"
  }, group.label), group.items.map(item => React.createElement(NavItem, {
    key: item.id,
    icon: asset(item.icon),
    label: item.label,
    count: item.count,
    active: page === item.id,
    onClick: () => onPage(item.id)
  }))))), React.createElement("div", {
    className: "sidebar__bottom"
  }, React.createElement(NavItem, {
    icon: asset("settings"),
    label: "设置",
    active: page === "settings",
    onClick: () => onPage("settings")
  }), React.createElement("div", {
    className: "service-control"
  }, React.createElement("div", {
    className: "service-control__top"
  }, React.createElement("span", {
    className: "service-control__title"
  }, React.createElement("i", {
    className: `service-control__dot ${running ? "" : "is-off"}`
  }), serviceBusy ? "正在切换…" : running ? "服务运行中" : "服务已停止"), React.createElement(Switch, {
    checked: running,
    disabled: serviceBusy,
    onChange: onRunning,
    label: "转发服务开关"
  })), React.createElement("div", {
    className: "service-control__note"
  }, "内置引擎 · ", running ? "4 条规则监听中" : "所有转发已停止"))));
}
function WindowBar({
  page,
  onAction,
  onNewMapping,
  onClearChat,
  actionState
}) {
  const [title, subtitle] = pageMeta[page];
  const busy = actionState === "checking" || actionState === "applying";
  return React.createElement("header", {
    className: "window-bar"
  }, React.createElement("div", {
    className: "window-bar__title"
  }, React.createElement("h1", {
    className: "page-title"
  }, title), React.createElement("div", {
    className: "window-bar__subtitle"
  }, subtitle)), React.createElement("div", {
    className: "window-bar__actions"
  }, actionState === "checking" && React.createElement("span", {
    className: "action-status"
  }, "正在检查端口与上游"), actionState === "applying" && React.createElement("span", {
    className: "action-status"
  }, "正在应用配置"), actionState === "checked" && React.createElement("span", {
    className: "action-status action-status--success"
  }, "预检通过"), page === "mappings" && React.createElement(React.Fragment, null, React.createElement(Button, {
    disabled: busy,
    onClick: () => onAction("pre-check")
  }, "预检"), React.createElement(Button, {
    variant: "primary",
    disabled: busy,
    onClick: () => onAction("apply")
  }, actionState === "applying" ? "应用中…" : "应用配置")), page === "mappings" && React.createElement(Button, {
    variant: "secondary",
    disabled: busy,
    onClick: onNewMapping
  }, "新建映射"), page === "chat" && React.createElement(Button, {
    variant: "quiet",
    onClick: onClearChat
  }, "清空对话")));
}
Object.assign(window, {
  AppSidebar,
  WindowBar
});
})();
