/* @ds-bundle: {"format":3,"namespace":"JumpboardRedesign_2bf643","components":[{"name":"Badge","sourcePath":"components/core/Badge.jsx"},{"name":"Button","sourcePath":"components/core/Button.jsx"},{"name":"IconButton","sourcePath":"components/core/IconButton.jsx"},{"name":"Panel","sourcePath":"components/core/Panel.jsx"},{"name":"Status","sourcePath":"components/core/Status.jsx"},{"name":"DataTable","sourcePath":"components/data/DataTable.jsx"},{"name":"Dialog","sourcePath":"components/feedback/Dialog.jsx"},{"name":"Notice","sourcePath":"components/feedback/Notice.jsx"},{"name":"Input","sourcePath":"components/forms/Input.jsx"},{"name":"Segmented","sourcePath":"components/forms/Segmented.jsx"},{"name":"Switch","sourcePath":"components/forms/Switch.jsx"},{"name":"NavItem","sourcePath":"components/navigation/NavItem.jsx"}],"sourceHashes":{"components/core/Badge.jsx":"a78cfbbff654","components/core/Button.jsx":"f5e996a8ff14","components/core/IconButton.jsx":"38396f636bb0","components/core/Panel.jsx":"71581cb70457","components/core/Status.jsx":"fded9035c7c1","components/data/DataTable.jsx":"a059f638d9ec","components/feedback/Dialog.jsx":"1ac9d3f06dc6","components/feedback/Notice.jsx":"4ba1fdca8308","components/forms/Input.jsx":"ee333f071401","components/forms/Segmented.jsx":"c2ff6447b35a","components/forms/Switch.jsx":"8c363d51542e","components/navigation/NavItem.jsx":"18f8256c1f4a","ui_kits/workbench/app.jsx":"568312dc5cd0","ui_kits/workbench/chrome.jsx":"776b3f963589","ui_kits/workbench/data.jsx":"cedc5073ec58","ui_kits/workbench/mappings.jsx":"be9b77450d8c","ui_kits/workbench/observability.jsx":"9fb679d31124","ui_kits/workbench/scene.jsx":"25abc784f552","ui_kits/workbench/settings.jsx":"05700dcea691","ui_kits/workbench/tools.jsx":"a71b60e776c5"},"inlinedExternals":[],"unexposedExports":[]} */

(() => {

const __ds_ns = (window.JumpboardRedesign_2bf643 = window.JumpboardRedesign_2bf643 || {});

const __ds_scope = {};

(__ds_ns.__errors = __ds_ns.__errors || []);

// components/core/Badge.jsx
try { (() => {
function Badge({
  children,
  protocol,
  className = ""
}) {
  return /*#__PURE__*/React.createElement("span", {
    className: `jb-badge ${protocol ? `jb-badge--${protocol.toLowerCase()}` : ""} ${className}`
  }, children);
}
Object.assign(__ds_scope, { Badge });
})(); } catch (e) { __ds_ns.__errors.push({ path: "components/core/Badge.jsx", error: String((e && e.message) || e) }); }

// components/core/Button.jsx
try { (() => {
function _extends() { return _extends = Object.assign ? Object.assign.bind() : function (n) { for (var e = 1; e < arguments.length; e++) { var t = arguments[e]; for (var r in t) ({}).hasOwnProperty.call(t, r) && (n[r] = t[r]); } return n; }, _extends.apply(null, arguments); }
function Button({
  children,
  variant = "secondary",
  icon,
  className = "",
  ...props
}) {
  return /*#__PURE__*/React.createElement("button", _extends({
    className: `jb-button jb-button--${variant} ${className}`
  }, props), icon && /*#__PURE__*/React.createElement("img", {
    src: icon,
    alt: "",
    "aria-hidden": "true"
  }), children);
}
Object.assign(__ds_scope, { Button });
})(); } catch (e) { __ds_ns.__errors.push({ path: "components/core/Button.jsx", error: String((e && e.message) || e) }); }

// components/core/IconButton.jsx
try { (() => {
function _extends() { return _extends = Object.assign ? Object.assign.bind() : function (n) { for (var e = 1; e < arguments.length; e++) { var t = arguments[e]; for (var r in t) ({}).hasOwnProperty.call(t, r) && (n[r] = t[r]); } return n; }, _extends.apply(null, arguments); }
function IconButton({
  icon,
  label,
  danger = false,
  className = "",
  ...props
}) {
  return /*#__PURE__*/React.createElement("button", _extends({
    type: "button",
    className: `jb-icon-button ${danger ? "jb-icon-button--danger" : ""} ${className}`,
    "aria-label": label,
    title: label
  }, props), /*#__PURE__*/React.createElement("img", {
    src: icon,
    alt: "",
    "aria-hidden": "true"
  }));
}
Object.assign(__ds_scope, { IconButton });
})(); } catch (e) { __ds_ns.__errors.push({ path: "components/core/IconButton.jsx", error: String((e && e.message) || e) }); }

// components/core/Panel.jsx
try { (() => {
function _extends() { return _extends = Object.assign ? Object.assign.bind() : function (n) { for (var e = 1; e < arguments.length; e++) { var t = arguments[e]; for (var r in t) ({}).hasOwnProperty.call(t, r) && (n[r] = t[r]); } return n; }, _extends.apply(null, arguments); }
function Panel({
  children,
  className = "",
  ...props
}) {
  return /*#__PURE__*/React.createElement("section", _extends({
    className: `jb-panel ${className}`
  }, props), children);
}
Object.assign(__ds_scope, { Panel });
})(); } catch (e) { __ds_ns.__errors.push({ path: "components/core/Panel.jsx", error: String((e && e.message) || e) }); }

// components/core/Status.jsx
try { (() => {
function Status({
  children,
  tone = "neutral"
}) {
  return /*#__PURE__*/React.createElement("span", {
    className: `jb-status ${tone !== "neutral" ? `jb-status--${tone}` : ""}`
  }, children);
}
Object.assign(__ds_scope, { Status });
})(); } catch (e) { __ds_ns.__errors.push({ path: "components/core/Status.jsx", error: String((e && e.message) || e) }); }

// components/data/DataTable.jsx
try { (() => {
function DataTable({
  columns,
  children,
  label
}) {
  return /*#__PURE__*/React.createElement("table", {
    className: "jb-table",
    "aria-label": label
  }, /*#__PURE__*/React.createElement("thead", null, /*#__PURE__*/React.createElement("tr", null, columns.map(column => /*#__PURE__*/React.createElement("th", {
    key: column.key,
    style: {
      width: column.width
    }
  }, column.label)))), /*#__PURE__*/React.createElement("tbody", null, children));
}
Object.assign(__ds_scope, { DataTable });
})(); } catch (e) { __ds_ns.__errors.push({ path: "components/data/DataTable.jsx", error: String((e && e.message) || e) }); }

// components/feedback/Dialog.jsx
try { (() => {
function Dialog({
  title,
  children,
  footer,
  onClose
}) {
  const panelRef = React.useRef(null);
  const closeRef = React.useRef(onClose);
  closeRef.current = onClose;
  React.useEffect(() => {
    const previous = document.activeElement;
    const panel = panelRef.current;
    const previousInert = [];
    let root = panel;
    while (root?.parentElement && root.parentElement !== document.body) root = root.parentElement;
    [...document.body.children].filter(element => element !== root && !["SCRIPT", "STYLE", "LINK"].includes(element.tagName)).forEach(element => {
      previousInert.push([element, element.inert]);
      element.inert = true;
    });
    panel?.focus();
    const onKey = event => {
      if (event.key === "Escape") {
        event.preventDefault();
        closeRef.current();
        return;
      }
      if (event.key !== "Tab") return;
      const focusable = [...panel.querySelectorAll('button:not(:disabled), input:not(:disabled), select:not(:disabled), textarea:not(:disabled), [tabindex="0"]')];
      if (!focusable.length) {
        event.preventDefault();
        panel.focus();
        return;
      }
      const first = focusable[0];
      const last = focusable[focusable.length - 1];
      if (event.shiftKey && (document.activeElement === first || document.activeElement === panel)) {
        event.preventDefault();
        last.focus();
      } else if (!event.shiftKey && document.activeElement === last) {
        event.preventDefault();
        first.focus();
      }
    };
    document.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("keydown", onKey);
      previousInert.forEach(([element, inert]) => {
        element.inert = inert;
      });
      previous?.focus();
    };
  }, []);
  return /*#__PURE__*/React.createElement("div", {
    className: "jb-dialog-backdrop",
    onMouseDown: event => {
      if (event.target === event.currentTarget) closeRef.current();
    }
  }, /*#__PURE__*/React.createElement("section", {
    ref: panelRef,
    tabIndex: -1,
    className: "jb-dialog",
    role: "dialog",
    "aria-modal": "true",
    "aria-label": title
  }, /*#__PURE__*/React.createElement("header", {
    className: "jb-dialog__header"
  }, /*#__PURE__*/React.createElement("h2", {
    className: "section-title"
  }, title), /*#__PURE__*/React.createElement("span", {
    style: {
      flex: 1
    }
  }), /*#__PURE__*/React.createElement("button", {
    className: "jb-icon-button",
    onClick: onClose,
    "aria-label": "\u5173\u95ED"
  }, "\xD7")), /*#__PURE__*/React.createElement("div", {
    className: "jb-dialog__body"
  }, children), footer && /*#__PURE__*/React.createElement("footer", {
    className: "jb-dialog__footer"
  }, footer)));
}
Object.assign(__ds_scope, { Dialog });
})(); } catch (e) { __ds_ns.__errors.push({ path: "components/feedback/Dialog.jsx", error: String((e && e.message) || e) }); }

// components/feedback/Notice.jsx
try { (() => {
function Notice({
  children,
  tone = "warning",
  action
}) {
  return /*#__PURE__*/React.createElement("div", {
    className: `jb-notice ${tone === "danger" ? "jb-notice--danger" : ""}`,
    role: tone === "danger" ? "alert" : "status"
  }, /*#__PURE__*/React.createElement("span", {
    className: "jb-notice__body"
  }, children), action);
}
Object.assign(__ds_scope, { Notice });
})(); } catch (e) { __ds_ns.__errors.push({ path: "components/feedback/Notice.jsx", error: String((e && e.message) || e) }); }

// components/forms/Input.jsx
try { (() => {
function _extends() { return _extends = Object.assign ? Object.assign.bind() : function (n) { for (var e = 1; e < arguments.length; e++) { var t = arguments[e]; for (var r in t) ({}).hasOwnProperty.call(t, r) && (n[r] = t[r]); } return n; }, _extends.apply(null, arguments); }
function Input({
  label,
  hint,
  id,
  className = "",
  ...props
}) {
  return /*#__PURE__*/React.createElement("div", {
    className: `jb-field ${className}`
  }, label && /*#__PURE__*/React.createElement("label", {
    htmlFor: id
  }, label), /*#__PURE__*/React.createElement("input", _extends({
    id: id,
    className: "jb-input"
  }, props)), hint && /*#__PURE__*/React.createElement("small", null, hint));
}
Object.assign(__ds_scope, { Input });
})(); } catch (e) { __ds_ns.__errors.push({ path: "components/forms/Input.jsx", error: String((e && e.message) || e) }); }

// components/forms/Segmented.jsx
try { (() => {
function Segmented({
  options,
  value,
  onChange,
  label
}) {
  return /*#__PURE__*/React.createElement("div", {
    className: "jb-segmented",
    role: "group",
    "aria-label": label
  }, options.map(option => /*#__PURE__*/React.createElement("button", {
    type: "button",
    className: "jb-segment",
    key: option,
    "aria-pressed": option === value,
    onClick: () => onChange(option)
  }, option)));
}
Object.assign(__ds_scope, { Segmented });
})(); } catch (e) { __ds_ns.__errors.push({ path: "components/forms/Segmented.jsx", error: String((e && e.message) || e) }); }

// components/forms/Switch.jsx
try { (() => {
function _extends() { return _extends = Object.assign ? Object.assign.bind() : function (n) { for (var e = 1; e < arguments.length; e++) { var t = arguments[e]; for (var r in t) ({}).hasOwnProperty.call(t, r) && (n[r] = t[r]); } return n; }, _extends.apply(null, arguments); }
function Switch({
  checked,
  onChange,
  label,
  ...props
}) {
  return /*#__PURE__*/React.createElement("button", _extends({
    type: "button",
    className: "jb-switch",
    role: "switch",
    "aria-checked": checked,
    "aria-label": label,
    onClick: () => onChange(!checked)
  }, props));
}
Object.assign(__ds_scope, { Switch });
})(); } catch (e) { __ds_ns.__errors.push({ path: "components/forms/Switch.jsx", error: String((e && e.message) || e) }); }

// components/navigation/NavItem.jsx
try { (() => {
function _extends() { return _extends = Object.assign ? Object.assign.bind() : function (n) { for (var e = 1; e < arguments.length; e++) { var t = arguments[e]; for (var r in t) ({}).hasOwnProperty.call(t, r) && (n[r] = t[r]); } return n; }, _extends.apply(null, arguments); }
function NavItem({
  icon,
  label,
  count,
  active = false,
  ...props
}) {
  return /*#__PURE__*/React.createElement("button", _extends({
    type: "button",
    className: "jb-nav-item",
    "aria-current": active ? "page" : undefined
  }, props), /*#__PURE__*/React.createElement("img", {
    src: icon,
    alt: "",
    "aria-hidden": "true"
  }), /*#__PURE__*/React.createElement("span", null, label), count !== undefined && /*#__PURE__*/React.createElement("span", {
    className: "jb-nav-item__count"
  }, count));
}
Object.assign(__ds_scope, { NavItem });
})(); } catch (e) { __ds_ns.__errors.push({ path: "components/navigation/NavItem.jsx", error: String((e && e.message) || e) }); }

// ui_kits/workbench/app.jsx
try { (() => {
const {
  Button,
  Dialog
} = DS;
function App() {
  const [page, setPageState] = React.useState(() => {
    const requested = new URLSearchParams(window.location.search).get("page");
    return Object.keys(pageMeta).includes(requested) ? requested : "mappings";
  });
  const setPage = next => {
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
  const [serviceBusy, setServiceBusy] = React.useState(false);
  const [actionState, setActionState] = React.useState("idle");
  const [scene, setScene] = React.useState("默认");
  const [sceneCandidate, setSceneCandidate] = React.useState(null);
  const [mappingsByScene, setMappingsByScene] = React.useState(() => ({
    默认: {
      mappings: initialMappings,
      dirty: true
    },
    开发环境: {
      mappings: initialMappings.slice(0, 3).map((mapping, index) => ({
        ...mapping,
        id: `dev-${mapping.id}`,
        name: ["开发 MySQL", "开发 GitLab", "开发 Redis"][index],
        local: mapping.local.replace(/9\d{3}/, port => String(Number(port) + 100)),
        health: "未检查",
        tone: "neutral",
        group: "开发环境"
      })),
      dirty: false
    }
  }));
  const mappings = mappingsByScene[scene].mappings;
  const dirty = mappingsByScene[scene].dirty;
  const scenes = [{
    name: "默认",
    count: mappingsByScene.默认.mappings.length
  }, {
    name: "开发环境",
    count: mappingsByScene.开发环境.mappings.length
  }];
  const setMappings = next => setMappingsByScene(previous => ({
    ...previous,
    [scene]: {
      ...previous[scene],
      mappings: typeof next === "function" ? next(previous[scene].mappings) : next
    }
  }));
  const setDirty = next => setMappingsByScene(previous => ({
    ...previous,
    [scene]: {
      ...previous[scene],
      dirty: typeof next === "function" ? next(previous[scene].dirty) : next
    }
  }));
  const [mappingDialog, setMappingDialog] = React.useState(null);
  const [confirmStop, setConfirmStop] = React.useState(false);
  const [deleteCandidate, setDeleteCandidate] = React.useState(null);
  const [confirmImport, setConfirmImport] = React.useState(null);
  const [toast, setToast] = React.useState("");
  const notify = message => setToast(message);
  React.useEffect(() => {
    if (!toast) return;
    const timer = setTimeout(() => setToast(""), 3200);
    return () => clearTimeout(timer);
  }, [toast]);
  const runAction = action => {
    if (actionState === "checking" || actionState === "applying") return;
    if (action === "pre-check") {
      setActionState("checking");
      setTimeout(() => {
        setActionState("checked");
        notify("预检通过：端口可用，上游地址可解析");
      }, 850);
    } else if (action === "apply") {
      setActionState("applying");
      setTimeout(() => {
        setActionState("idle");
        setDirty(false);
        notify("配置已应用，转发规则已生效");
      }, 1100);
    } else {
      notify(action);
    }
  };
  const switchService = next => {
    if (serviceBusy) return;
    if (!next) {
      setConfirmStop(true);
      return;
    }
    setServiceBusy(true);
    setTimeout(() => {
      setRunning(true);
      setServiceBusy(false);
      notify("转发服务已启动");
    }, 800);
  };
  const chooseScene = next => {
    if (next === scene) return;
    if (dirty) {
      setSceneCandidate(next);
      return;
    }
    setScene(next);
    notify(`已切换到「${next}」`);
  };
  const saveMapping = (mapping, apply) => {
    setMappings(previous => previous.some(item => item.id === mapping.id) ? previous.map(item => item.id === mapping.id ? mapping : item) : [...previous, mapping]);
    setMappingDialog(null);
    setDirty(!apply);
    notify(apply ? "映射已保存，配置已应用" : "映射已保存，等待应用配置");
  };
  return /*#__PURE__*/React.createElement("div", {
    className: "prototype-shell"
  }, /*#__PURE__*/React.createElement("div", {
    className: "prototype-meta"
  }, /*#__PURE__*/React.createElement("strong", null, "JumpBoard / \u754C\u9762\u91CD\u8BBE\u8BA1"), /*#__PURE__*/React.createElement("span", {
    className: "prototype-meta__rule"
  }), /*#__PURE__*/React.createElement("span", null, "\u514B\u5236\u4E13\u4E1A \xB7 \u539F\u751F\u684C\u9762\u5DE5\u4F5C\u53F0"), /*#__PURE__*/React.createElement("span", {
    className: "prototype-meta__spacer"
  }), /*#__PURE__*/React.createElement("span", null, "\u65B9\u6848\u5916\u89C2"), /*#__PURE__*/React.createElement("div", {
    className: "prototype-theme"
  }, ["亮色", "暗色"].map(choice => /*#__PURE__*/React.createElement("button", {
    key: choice,
    "aria-pressed": theme === choice,
    onClick: () => setTheme(choice)
  }, choice)))), /*#__PURE__*/React.createElement("div", {
    className: "desktop-window"
  }, /*#__PURE__*/React.createElement(AppSidebar, {
    page: page,
    onPage: setPage,
    running: running,
    onRunning: switchService,
    serviceBusy: serviceBusy
  }), /*#__PURE__*/React.createElement("main", {
    className: "main"
  }, /*#__PURE__*/React.createElement(WindowBar, {
    page: page,
    onAction: runAction,
    actionState: actionState,
    onNewMapping: () => setMappingDialog({
      mapping: null
    }),
    onClearChat: () => notify("清空前需要确认，避免误删当前对话")
  }), /*#__PURE__*/React.createElement("div", {
    className: "page"
  }, page === "mappings" && /*#__PURE__*/React.createElement(MappingsPage, {
    mappings: mappings,
    setMappings: setMappings,
    dirty: dirty,
    setDirty: setDirty,
    scene: scene,
    scenes: scenes,
    onSceneSelect: chooseScene,
    onAction: runAction,
    actionState: actionState,
    onNewMapping: () => setMappingDialog({
      mapping: null
    }),
    onEditMapping: mapping => setMappingDialog({
      mapping
    }),
    onDeleteMapping: setDeleteCandidate,
    onChat: () => setPage("chat"),
    onSettings: () => setPage("settings")
  }), page === "logs" && /*#__PURE__*/React.createElement(LogsPage, {
    onAction: notify
  }), page === "history" && /*#__PURE__*/React.createElement(HistoryPage, {
    onAction: notify
  }), page === "terminal" && /*#__PURE__*/React.createElement(TerminalPage, {
    onAction: notify
  }), page === "chat" && /*#__PURE__*/React.createElement(ChatPage, {
    onAction: notify,
    onClear: () => notify("清空前需要确认"),
    onImport: setConfirmImport
  }), page === "settings" && /*#__PURE__*/React.createElement(SettingsPage, {
    theme: theme,
    setTheme: setTheme,
    onAction: notify
  })))), /*#__PURE__*/React.createElement("div", {
    className: "prototype-notes"
  }, /*#__PURE__*/React.createElement("span", null, "\u4EA4\u4E92\u63D0\u6848 \xB7 \u975E\u751F\u4EA7\u754C\u9762 \xB7 \u4E0D\u8C03\u7528\u5916\u90E8\u670D\u52A1"), /*#__PURE__*/React.createElement("span", null, "\u516D\u9875\u5BFC\u822A\u3001\u65B0\u5EFA/\u7F16\u8F91\u6620\u5C04\u3001\u8349\u7A3F\u5BFC\u5165\u4E0E\u8BBE\u7F6E\u5206\u7EC4\u5747\u53EF\u4F53\u9A8C")), sceneCandidate && /*#__PURE__*/React.createElement(Dialog, {
    title: "\u5207\u6362\u573A\u666F",
    onClose: () => setSceneCandidate(null),
    footer: /*#__PURE__*/React.createElement(React.Fragment, null, /*#__PURE__*/React.createElement(Button, {
      onClick: () => setSceneCandidate(null)
    }, "\u53D6\u6D88"), /*#__PURE__*/React.createElement(Button, {
      variant: "primary",
      onClick: () => {
        setScene(sceneCandidate);
        setSceneCandidate(null);
        notify(`已切换到「${sceneCandidate}」，未应用改动已保留`);
      }
    }, "\u4FDD\u7559\u6539\u52A8\u5E76\u5207\u6362"))
  }, /*#__PURE__*/React.createElement("p", {
    className: "body-copy"
  }, "\u5F53\u524D\u573A\u666F\u6709\u672A\u5E94\u7528\u6539\u52A8\u3002\u5207\u6362\u540E\u8FD9\u4E9B\u6539\u52A8\u4F1A\u4FDD\u7559\u5728\u300C", scene, "\u300D\uFF0C\u4E0D\u4F1A\u6DF7\u5165\u300C", sceneCandidate, "\u300D\u3002")), mappingDialog && /*#__PURE__*/React.createElement(MappingDialog, {
    mapping: mappingDialog.mapping,
    onClose: () => setMappingDialog(null),
    onSave: saveMapping
  }), confirmStop && /*#__PURE__*/React.createElement(Dialog, {
    title: "\u505C\u6B62\u8F6C\u53D1\u670D\u52A1",
    onClose: () => setConfirmStop(false),
    footer: /*#__PURE__*/React.createElement(React.Fragment, null, /*#__PURE__*/React.createElement(Button, {
      onClick: () => setConfirmStop(false)
    }, "\u53D6\u6D88"), /*#__PURE__*/React.createElement(Button, {
      variant: "danger",
      onClick: () => {
        setConfirmStop(false);
        setServiceBusy(true);
        setTimeout(() => {
          setRunning(false);
          setServiceBusy(false);
          notify("转发服务已停止，所有转发已失效");
        }, 800);
      }
    }, "\u505C\u6B62\u670D\u52A1"))
  }, /*#__PURE__*/React.createElement("p", {
    className: "body-copy"
  }, "\u505C\u6B62\u540E\u6240\u6709\u8F6C\u53D1\u89C4\u5219\u7ACB\u5373\u5931\u6548\uFF0C\u5916\u90E8\u65E0\u6CD5\u8BBF\u95EE\u3002\u5DF2\u6709\u914D\u7F6E\u4F1A\u4FDD\u7559\uFF0C\u91CD\u65B0\u542F\u52A8\u540E\u53EF\u6062\u590D\u3002")), deleteCandidate && /*#__PURE__*/React.createElement(Dialog, {
    title: "\u5220\u9664\u6620\u5C04",
    onClose: () => setDeleteCandidate(null),
    footer: /*#__PURE__*/React.createElement(React.Fragment, null, /*#__PURE__*/React.createElement(Button, {
      onClick: () => setDeleteCandidate(null)
    }, "\u53D6\u6D88"), /*#__PURE__*/React.createElement(Button, {
      variant: "danger",
      onClick: () => {
        setMappings(mappings.filter(item => item.id !== deleteCandidate.id));
        setDirty(true);
        notify(`已删除「${deleteCandidate.name}」`);
        setDeleteCandidate(null);
      }
    }, "\u786E\u8BA4\u5220\u9664"))
  }, /*#__PURE__*/React.createElement("p", {
    className: "body-copy"
  }, "\u300C", deleteCandidate.name, "\u300D\u5C06\u88AB\u5220\u9664\uFF0C\u9700\u8981\u91CD\u65B0\u521B\u5EFA\u3002\u6B63\u5728\u8FD0\u884C\u7684\u914D\u7F6E\u8981\u70B9\u51FB\u300C\u5E94\u7528\u914D\u7F6E\u300D\u540E\u624D\u4F1A\u540C\u6B65\u5220\u9664\u3002")), confirmImport && /*#__PURE__*/React.createElement(Dialog, {
    title: "\u5BFC\u5165\u6620\u5C04\u8349\u7A3F",
    onClose: () => setConfirmImport(null),
    footer: /*#__PURE__*/React.createElement(React.Fragment, null, /*#__PURE__*/React.createElement(Button, {
      onClick: () => setConfirmImport(null)
    }, "\u53D6\u6D88"), /*#__PURE__*/React.createElement(Button, {
      variant: "primary",
      onClick: () => {
        setMappings([...mappings, ...confirmImport.map((draft, index) => ({
          ...draft,
          id: Date.now() + index,
          enabled: true,
          health: "未检查",
          tone: "neutral",
          group: "AI 导入",
          traffic: "0 连接 · 0 B"
        }))]);
        setDirty(true);
        setPage("mappings");
        notify(`已导入 ${confirmImport.length} 条草稿，等待应用配置`);
        setConfirmImport(null);
      }
    }, "\u5BFC\u5165"))
  }, /*#__PURE__*/React.createElement("p", {
    className: "body-copy"
  }, "\u628A ", confirmImport.length, " \u6761\u8349\u7A3F\u5199\u5165\u5F53\u524D\u573A\u666F\uFF0C\u4F7F\u7528\u4E0E\u624B\u52A8\u65B0\u5EFA\u76F8\u540C\u7684\u6821\u9A8C\u3002\u5BFC\u5165\u540E\u70B9\u51FB\u300C\u5E94\u7528\u914D\u7F6E\u300D\u624D\u4F1A\u751F\u6548\u3002")), toast && /*#__PURE__*/React.createElement("div", {
    className: "prototype-toast",
    role: "status"
  }, toast));
}
ReactDOM.createRoot(document.getElementById("root")).render(/*#__PURE__*/React.createElement(App, null));
})(); } catch (e) { __ds_ns.__errors.push({ path: "ui_kits/workbench/app.jsx", error: String((e && e.message) || e) }); }

// ui_kits/workbench/chrome.jsx
try { (() => {
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
  return /*#__PURE__*/React.createElement("aside", {
    className: "sidebar"
  }, /*#__PURE__*/React.createElement("div", {
    className: "sidebar__traffic",
    "aria-hidden": "true"
  }, /*#__PURE__*/React.createElement("i", {
    className: "traffic-dot"
  }), /*#__PURE__*/React.createElement("i", {
    className: "traffic-dot"
  }), /*#__PURE__*/React.createElement("i", {
    className: "traffic-dot"
  })), /*#__PURE__*/React.createElement("div", {
    className: "brand"
  }, /*#__PURE__*/React.createElement("div", {
    className: "brand__mark"
  }, /*#__PURE__*/React.createElement("img", {
    src: asset("relay"),
    alt: ""
  })), /*#__PURE__*/React.createElement("div", null, /*#__PURE__*/React.createElement("div", {
    className: "brand__name"
  }, "JumpBoard"), /*#__PURE__*/React.createElement("div", {
    className: "brand__subtitle"
  }, "\u5185\u7F51\u7AEF\u53E3\u8F6C\u53D1"))), /*#__PURE__*/React.createElement("nav", {
    className: "navigation",
    "aria-label": "\u4E3B\u5BFC\u822A"
  }, navigationGroups.map(group => /*#__PURE__*/React.createElement("div", {
    className: "navigation__group",
    key: group.label
  }, /*#__PURE__*/React.createElement("div", {
    className: "navigation__label"
  }, group.label), group.items.map(item => /*#__PURE__*/React.createElement(NavItem, {
    key: item.id,
    icon: asset(item.icon),
    label: item.label,
    count: item.count,
    active: page === item.id,
    onClick: () => onPage(item.id)
  }))))), /*#__PURE__*/React.createElement("div", {
    className: "sidebar__bottom"
  }, /*#__PURE__*/React.createElement(NavItem, {
    icon: asset("settings"),
    label: "\u8BBE\u7F6E",
    active: page === "settings",
    onClick: () => onPage("settings")
  }), /*#__PURE__*/React.createElement("div", {
    className: "service-control"
  }, /*#__PURE__*/React.createElement("div", {
    className: "service-control__top"
  }, /*#__PURE__*/React.createElement("span", {
    className: "service-control__title"
  }, /*#__PURE__*/React.createElement("i", {
    className: `service-control__dot ${running ? "" : "is-off"}`
  }), serviceBusy ? "正在切换…" : running ? "服务运行中" : "服务已停止"), /*#__PURE__*/React.createElement(Switch, {
    checked: running,
    disabled: serviceBusy,
    onChange: onRunning,
    label: "\u8F6C\u53D1\u670D\u52A1\u5F00\u5173"
  })), /*#__PURE__*/React.createElement("div", {
    className: "service-control__note"
  }, "\u5185\u7F6E\u5F15\u64CE \xB7 ", running ? "4 条规则监听中" : "所有转发已停止"))));
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
  return /*#__PURE__*/React.createElement("header", {
    className: "window-bar"
  }, /*#__PURE__*/React.createElement("div", {
    className: "window-bar__title"
  }, /*#__PURE__*/React.createElement("h1", {
    className: "page-title"
  }, title), /*#__PURE__*/React.createElement("div", {
    className: "window-bar__subtitle"
  }, subtitle)), /*#__PURE__*/React.createElement("div", {
    className: "window-bar__actions"
  }, actionState === "checking" && /*#__PURE__*/React.createElement("span", {
    className: "action-status"
  }, "\u6B63\u5728\u68C0\u67E5\u7AEF\u53E3\u4E0E\u4E0A\u6E38"), actionState === "applying" && /*#__PURE__*/React.createElement("span", {
    className: "action-status"
  }, "\u6B63\u5728\u5E94\u7528\u914D\u7F6E"), actionState === "checked" && /*#__PURE__*/React.createElement("span", {
    className: "action-status action-status--success"
  }, "\u9884\u68C0\u901A\u8FC7"), page === "mappings" && /*#__PURE__*/React.createElement(React.Fragment, null, /*#__PURE__*/React.createElement(Button, {
    disabled: busy,
    onClick: () => onAction("pre-check")
  }, "\u9884\u68C0"), /*#__PURE__*/React.createElement(Button, {
    variant: "primary",
    disabled: busy,
    onClick: () => onAction("apply")
  }, actionState === "applying" ? "应用中…" : "应用配置")), page === "mappings" && /*#__PURE__*/React.createElement(Button, {
    variant: "secondary",
    disabled: busy,
    onClick: onNewMapping
  }, "\u65B0\u5EFA\u6620\u5C04"), page === "chat" && /*#__PURE__*/React.createElement(Button, {
    variant: "quiet",
    onClick: onClearChat
  }, "\u6E05\u7A7A\u5BF9\u8BDD")));
}
Object.assign(window, {
  AppSidebar,
  WindowBar
});
})(); } catch (e) { __ds_ns.__errors.push({ path: "ui_kits/workbench/chrome.jsx", error: String((e && e.message) || e) }); }

// ui_kits/workbench/data.jsx
try { (() => {
const DS = window.JumpboardRedesign_2bf643;
const {
  Button,
  IconButton,
  Panel,
  Badge,
  Status,
  Input,
  Switch,
  Segmented,
  NavItem,
  Notice,
  Dialog,
  DataTable
} = DS;
const iconPaths = {
  activity: "../assets/icons/activity.svg",
  alert: "../assets/icons/alert.svg",
  chat: "../assets/icons/chat.svg",
  check: "../assets/icons/check.svg",
  connector: "../assets/icons/connector.svg",
  delete: "../assets/icons/delete.svg",
  edit: "../assets/icons/edit.svg",
  history: "../assets/icons/history.svg",
  logs: "../assets/icons/logs.svg",
  relay: "../assets/icons/relay.svg",
  search: "../assets/icons/search.svg",
  settings: "../assets/icons/settings.svg",
  start: "../assets/icons/start.svg",
  stop: "../assets/icons/stop.svg",
  terminal: "../assets/icons/terminal.svg",
  unchecked: "../assets/icons/unchecked.svg"
};
const asset = name => iconPaths[name] || iconPaths.relay;
const initialMappings = [{
  id: 1,
  name: "MySQL 内网",
  protocol: "TCP",
  local: "127.0.0.1:9000",
  upstream: "192.168.1.10:3306",
  health: "健康",
  tone: "success",
  enabled: true,
  group: "数据库",
  traffic: "0 连接 · 0 B"
}, {
  id: 2,
  name: "GitLab Web",
  protocol: "HTTP",
  local: "git.example.com:8080",
  upstream: "192.168.1.20:80",
  health: "未检查",
  tone: "neutral",
  enabled: true,
  group: "Web 服务",
  traffic: "0 连接 · 0 B"
}, {
  id: 3,
  name: "Redis 缓存",
  protocol: "TCP",
  local: "127.0.0.1:9100",
  upstream: "192.168.1.30:6379",
  health: "异常",
  tone: "danger",
  enabled: false,
  group: "数据库",
  traffic: "—"
}, {
  id: 4,
  name: "MQTT Broker",
  protocol: "UDP",
  local: "127.0.0.1:1883",
  upstream: "192.168.1.40:1883",
  health: "健康",
  tone: "success",
  enabled: true,
  group: "物联网",
  traffic: "0 连接 · 0 B"
}, {
  id: 5,
  name: "日志收集",
  protocol: "UDP",
  local: "127.0.0.1:5140",
  upstream: "10.0.1.50:514",
  health: "未检查",
  tone: "neutral",
  enabled: true,
  group: "基础设施",
  traffic: "0 连接 · 0 B"
}];
const navigationGroups = [{
  label: "工作区",
  items: [{
    id: "mappings",
    label: "映射",
    icon: "relay",
    count: 5
  }, {
    id: "logs",
    label: "日志",
    icon: "logs"
  }, {
    id: "history",
    label: "配置历史",
    icon: "history"
  }]
}, {
  label: "工具",
  items: [{
    id: "terminal",
    label: "终端",
    icon: "terminal"
  }, {
    id: "chat",
    label: "AI 对话",
    icon: "chat"
  }]
}];
const pageMeta = {
  mappings: ["映射", "管理内网服务的转发规则"],
  logs: ["日志", "追踪配置操作与转发访问"],
  history: ["配置历史", "查看、比较和恢复每次应用的快照"],
  terminal: ["终端", "本地 shell、转发端口探测与串口"],
  chat: ["AI 对话", "描述需求，审阅草稿，再导入当前场景"],
  settings: ["设置", "应用偏好、AI 服务与数据管理"]
};
const settingSections = [["ai", "AI 配置", "chat"], ["general", "通用与外观", "settings"], ["access", "访问控制", "check"], ["backup", "备份与数据", "history"], ["workspace", "场景管理", "relay"], ["hotkeys", "快捷键", "terminal"], ["updates", "更新", "activity"], ["danger", "危险操作", "alert"]];
Object.assign(window, DS, {
  initialMappings,
  navigationGroups,
  pageMeta,
  settingSections,
  asset
});
})(); } catch (e) { __ds_ns.__errors.push({ path: "ui_kits/workbench/data.jsx", error: String((e && e.message) || e) }); }

// ui_kits/workbench/mappings.jsx
try { (() => {
const {
  Button,
  IconButton,
  Panel,
  Badge,
  Status,
  Input,
  Switch,
  Segmented,
  Notice,
  Dialog,
  DataTable
} = DS;
function MappingsPage({
  mappings,
  setMappings,
  dirty,
  setDirty,
  scene,
  scenes,
  onSceneSelect,
  onAction,
  actionState,
  onNewMapping,
  onEditMapping,
  onDeleteMapping,
  onChat,
  onSettings
}) {
  const [query, setQuery] = React.useState("");
  const [filter, setFilter] = React.useState("全部");
  const [group, setGroup] = React.useState("全部分组");
  const [detailId, setDetailId] = React.useState(null);
  const shown = mappings.filter(mapping => (filter === "全部" || (filter === "已启用" ? mapping.enabled : !mapping.enabled)) && (group === "全部分组" || mapping.group === group) && `${mapping.name} ${mapping.local} ${mapping.upstream}`.toLowerCase().includes(query.toLowerCase()));
  const detail = mappings.find(mapping => mapping.id === detailId);
  const toggleMapping = (id, enabled) => {
    setMappings(mappings.map(mapping => mapping.id === id ? {
      ...mapping,
      enabled
    } : mapping));
    setDirty(true);
  };
  return /*#__PURE__*/React.createElement("div", {
    className: "page__inner",
    "data-screen-label": "\u6620\u5C04\u5DE5\u4F5C\u53F0"
  }, /*#__PURE__*/React.createElement("div", {
    className: "toolbar"
  }, /*#__PURE__*/React.createElement(ScenePicker, {
    scene: scene,
    scenes: scenes,
    onSelect: onSceneSelect,
    onManage: onSettings
  }), /*#__PURE__*/React.createElement("div", {
    className: "toolbar__spacer"
  }), /*#__PURE__*/React.createElement(Button, {
    onClick: () => onAction("模板选择：MySQL、Redis、MongoDB、PostgreSQL、MQTT、RDP、SSH、WebSocket 等")
  }, "\u4ECE\u6A21\u677F\u65B0\u5EFA"), /*#__PURE__*/React.createElement(Button, {
    onClick: () => onAction("粘贴导入会预览变更，逐条校验后才会写入")
  }, "\u7C98\u8D34\u5BFC\u5165")), /*#__PURE__*/React.createElement("div", {
    className: "toolbar"
  }, /*#__PURE__*/React.createElement("div", {
    className: "search"
  }, /*#__PURE__*/React.createElement("img", {
    src: asset("search"),
    alt: ""
  }), /*#__PURE__*/React.createElement("input", {
    className: "jb-input",
    value: query,
    onChange: event => setQuery(event.target.value),
    placeholder: "\u641C\u7D22\u540D\u79F0\u3001\u7AEF\u53E3\u6216\u4E0A\u6E38\u5730\u5740",
    "aria-label": "\u641C\u7D22\u6620\u5C04"
  })), /*#__PURE__*/React.createElement(Segmented, {
    options: ["全部", "已启用", "已停用"],
    value: filter,
    onChange: setFilter,
    label: "\u7B5B\u9009\u6620\u5C04"
  }), /*#__PURE__*/React.createElement("div", {
    className: "toolbar__spacer"
  }), /*#__PURE__*/React.createElement(Button, {
    variant: "quiet",
    icon: asset("chat"),
    onClick: onChat
  }, "AI \u5EFA\u6620\u5C04"), /*#__PURE__*/React.createElement(Button, {
    variant: "quiet",
    onClick: () => onAction("AI 筛选只发送映射摘要，不包含密钥")
  }, "AI \u7B5B\u9009")), /*#__PURE__*/React.createElement("div", {
    className: "toolbar toolbar--chips"
  }, /*#__PURE__*/React.createElement("div", {
    className: "group-chips",
    role: "group",
    "aria-label": "\u6309\u5206\u7EC4\u7B5B\u9009"
  }, ["全部", ...new Set(mappings.map(mapping => mapping.group))].map(item => /*#__PURE__*/React.createElement("button", {
    key: item,
    type: "button",
    className: "group-chip",
    "aria-pressed": item === "全部" && group === "全部分组" || item === group,
    onClick: () => setGroup(item === "全部" ? "全部分组" : item)
  }, item)))), dirty && /*#__PURE__*/React.createElement("div", {
    className: "pending-notice"
  }, /*#__PURE__*/React.createElement(Notice, null, /*#__PURE__*/React.createElement("span", null, /*#__PURE__*/React.createElement("strong", null, "\u6709\u672A\u751F\u6548\u7684\u6539\u52A8\u3002"), " ", /*#__PURE__*/React.createElement("span", {
    className: "pending-notice__detail"
  }, "\u76D1\u542C\u7AEF\u53E3\u3001\u4E0A\u6E38\u5730\u5740\u6216\u542F\u7528\u72B6\u6001\u5DF2\u4FEE\u6539\uFF1B\u5F53\u524D\u8FD0\u884C\u914D\u7F6E\u4E0D\u53D8\u3002\u4F7F\u7528\u53F3\u4E0A\u89D2\u300C\u5E94\u7528\u914D\u7F6E\u300D\u751F\u6548\u3002")))), /*#__PURE__*/React.createElement(Panel, {
    className: "mapping-panel"
  }, /*#__PURE__*/React.createElement(DataTable, {
    label: "\u7AEF\u53E3\u8F6C\u53D1\u89C4\u5219",
    columns: [{
      key: "rule",
      label: "规则",
      width: "21%"
    }, {
      key: "protocol",
      label: "协议",
      width: "72px"
    }, {
      key: "forward",
      label: "转发关系"
    }, {
      key: "health",
      label: "健康",
      width: "92px"
    }, {
      key: "enabled",
      label: "启用",
      width: "62px"
    }, {
      key: "actions",
      label: "",
      width: "76px"
    }]
  }, shown.map(mapping => /*#__PURE__*/React.createElement("tr", {
    key: mapping.id,
    className: detailId === mapping.id ? "is-selected" : ""
  }, /*#__PURE__*/React.createElement("td", null, /*#__PURE__*/React.createElement("button", {
    className: "rule-name",
    onClick: () => setDetailId(detailId === mapping.id ? null : mapping.id)
  }, mapping.name), /*#__PURE__*/React.createElement("span", {
    className: "rule-meta"
  }, mapping.group)), /*#__PURE__*/React.createElement("td", null, /*#__PURE__*/React.createElement(Badge, {
    protocol: mapping.protocol
  }, mapping.protocol)), /*#__PURE__*/React.createElement("td", null, /*#__PURE__*/React.createElement("div", {
    className: "forward-relation"
  }, /*#__PURE__*/React.createElement("div", {
    className: "forward-relation__row mono",
    title: `${mapping.local} → ${mapping.upstream}`
  }, /*#__PURE__*/React.createElement("span", null, mapping.local), /*#__PURE__*/React.createElement("span", {
    className: "forward-relation__arrow"
  }, "\u2192"), /*#__PURE__*/React.createElement("span", null, mapping.upstream)), /*#__PURE__*/React.createElement("span", {
    className: "forward-relation__traffic"
  }, mapping.traffic))), /*#__PURE__*/React.createElement("td", null, /*#__PURE__*/React.createElement(Status, {
    tone: mapping.tone
  }, mapping.health)), /*#__PURE__*/React.createElement("td", null, /*#__PURE__*/React.createElement(Switch, {
    checked: mapping.enabled,
    onChange: enabled => toggleMapping(mapping.id, enabled),
    label: `启用-${mapping.name}`
  })), /*#__PURE__*/React.createElement("td", null, /*#__PURE__*/React.createElement("div", {
    className: "row-actions"
  }, /*#__PURE__*/React.createElement(IconButton, {
    icon: asset("edit"),
    label: `编辑-${mapping.name}`,
    onClick: () => onEditMapping(mapping)
  }), /*#__PURE__*/React.createElement(IconButton, {
    icon: asset("delete"),
    label: `删除-${mapping.name}`,
    danger: true,
    onClick: () => onDeleteMapping(mapping)
  })))))), shown.length === 0 && /*#__PURE__*/React.createElement("div", {
    className: "mapping-empty"
  }, "\u6CA1\u6709\u7B26\u5408\u5F53\u524D\u641C\u7D22\u548C\u7B5B\u9009\u6761\u4EF6\u7684\u6620\u5C04\u3002"), /*#__PURE__*/React.createElement("footer", {
    className: "panel-footer"
  }, /*#__PURE__*/React.createElement("span", null, "\u5F53\u524D\u573A\u666F\u300C", scene, "\u300D \xB7 \u663E\u793A ", shown.length, " / ", mappings.length, " \u6761\u89C4\u5219"), /*#__PURE__*/React.createElement("div", {
    className: "panel-footer__actions"
  }, /*#__PURE__*/React.createElement(Button, {
    variant: "quiet",
    onClick: () => {
      setMappings(mappings.map(mapping => ({
        ...mapping,
        enabled: true
      })));
      setDirty(true);
    }
  }, "\u5168\u90E8\u542F\u7528"), /*#__PURE__*/React.createElement(Button, {
    variant: "quiet",
    onClick: () => {
      setMappings(mappings.map(mapping => ({
        ...mapping,
        enabled: false
      })));
      setDirty(true);
    }
  }, "\u5168\u90E8\u505C\u7528")))), detail && /*#__PURE__*/React.createElement(Panel, {
    className: "detail-drawer"
  }, /*#__PURE__*/React.createElement("div", {
    className: "detail-drawer__header"
  }, /*#__PURE__*/React.createElement("h2", {
    className: "section-title"
  }, detail.name), /*#__PURE__*/React.createElement(Badge, {
    protocol: detail.protocol
  }, detail.protocol), /*#__PURE__*/React.createElement("span", {
    className: "toolbar__spacer"
  }), /*#__PURE__*/React.createElement(Status, {
    tone: detail.enabled ? "success" : "neutral"
  }, detail.enabled ? "已启用" : "已停用"), /*#__PURE__*/React.createElement(IconButton, {
    icon: asset("unchecked"),
    label: "\u5173\u95ED\u8BE6\u60C5",
    onClick: () => setDetailId(null)
  })), /*#__PURE__*/React.createElement("div", {
    className: "detail-drawer__body"
  }, /*#__PURE__*/React.createElement("div", {
    className: "detail-metric"
  }, /*#__PURE__*/React.createElement("small", null, "\u672C\u5730\u76D1\u542C"), /*#__PURE__*/React.createElement("strong", {
    className: "mono"
  }, detail.local)), /*#__PURE__*/React.createElement("div", {
    className: "detail-metric"
  }, /*#__PURE__*/React.createElement("small", null, "\u4E0A\u6E38\u76EE\u6807"), /*#__PURE__*/React.createElement("strong", {
    className: "mono"
  }, detail.upstream)), /*#__PURE__*/React.createElement("div", {
    className: "detail-metric"
  }, /*#__PURE__*/React.createElement("small", null, "\u8FDE\u63A5\u4E0E\u6D41\u91CF"), /*#__PURE__*/React.createElement("strong", null, detail.traffic))), /*#__PURE__*/React.createElement("div", {
    className: "detail-drawer__footer"
  }, /*#__PURE__*/React.createElement(Button, {
    onClick: () => onAction(`已复制 ${detail.local}`)
  }, "\u590D\u5236\u8F6C\u53D1\u5730\u5740"), /*#__PURE__*/React.createElement(Button, {
    onClick: () => onAction("已创建规则副本")
  }, "\u514B\u9686\u89C4\u5219"), /*#__PURE__*/React.createElement(Button, {
    onClick: () => onEditMapping(detail)
  }, "\u7F16\u8F91\u89C4\u5219"))));
}
function MappingDialog({
  mapping,
  onClose,
  onSave,
  onApply
}) {
  const [name, setName] = React.useState(mapping?.name || "");
  const [port, setPort] = React.useState(mapping?.local.split(":").pop() || "9000");
  const [host, setHost] = React.useState(mapping?.upstream.split(":").slice(0, -1).join(":") || "192.168.1.10");
  const [upstreamPort, setUpstreamPort] = React.useState(mapping?.upstream.split(":").pop() || "3306");
  const [protocol, setProtocol] = React.useState(mapping?.protocol || "TCP");
  const [error, setError] = React.useState("");
  const validate = () => {
    if (!name.trim()) return "请填写规则名称。";
    if (![port, upstreamPort].every(value => Number(value) >= 1 && Number(value) <= 65535)) return "端口需为 1–65535 之间的整数。";
    if (!host.trim()) return "请填写上游地址。";
    return "";
  };
  const save = apply => {
    const issue = validate();
    if (issue) {
      setError(issue);
      return;
    }
    onSave({
      id: mapping?.id || Date.now(),
      name,
      protocol,
      local: `127.0.0.1:${port}`,
      upstream: `${host}:${upstreamPort}`,
      health: "未检查",
      tone: "neutral",
      enabled: true,
      group: mapping?.group || "未分组",
      traffic: "0 连接 · 0 B"
    }, apply);
  };
  return /*#__PURE__*/React.createElement(Dialog, {
    title: mapping ? "编辑映射" : "新建映射",
    onClose: onClose,
    footer: /*#__PURE__*/React.createElement(React.Fragment, null, /*#__PURE__*/React.createElement("span", {
      className: "dialog-form-footer-note"
    }, "\u4FDD\u5B58\u53EA\u5165\u5E93\uFF1B\u5E94\u7528\u914D\u7F6E\u540E\u751F\u6548"), /*#__PURE__*/React.createElement(Button, {
      onClick: onClose
    }, "\u53D6\u6D88"), /*#__PURE__*/React.createElement(Button, {
      onClick: () => save(false)
    }, "\u4FDD\u5B58"), /*#__PURE__*/React.createElement(Button, {
      variant: "primary",
      onClick: () => save(true)
    }, "\u4FDD\u5B58\u5E76\u5E94\u7528"))
  }, /*#__PURE__*/React.createElement("div", {
    className: "jb-field-row"
  }, /*#__PURE__*/React.createElement(Input, {
    id: "mapping-name",
    label: "\u540D\u79F0",
    value: name,
    onChange: event => setName(event.target.value),
    placeholder: "\u4F8B\u5982\uFF1A\u5185\u7F51 MySQL"
  }), /*#__PURE__*/React.createElement(Input, {
    id: "mapping-group",
    label: "\u5206\u7EC4\uFF08\u53EF\u9009\uFF09",
    defaultValue: mapping?.group || "",
    placeholder: "\u4F8B\u5982\uFF1A\u6570\u636E\u5E93"
  })), /*#__PURE__*/React.createElement("div", {
    className: "dialog-protocol"
  }, /*#__PURE__*/React.createElement("span", {
    className: "text-secondary"
  }, "\u534F\u8BAE"), /*#__PURE__*/React.createElement(Segmented, {
    options: ["TCP", "UDP", "HTTP", "HTTPS"],
    value: protocol,
    onChange: setProtocol,
    label: "\u6620\u5C04\u534F\u8BAE"
  })), /*#__PURE__*/React.createElement("div", {
    className: "jb-section",
    style: {
      marginBottom: 0
    }
  }, /*#__PURE__*/React.createElement(Input, {
    id: "listen-port",
    label: "\u672C\u5730\u76D1\u542C\u7AEF\u53E3",
    value: port,
    onChange: event => setPort(event.target.value),
    inputMode: "numeric"
  })), /*#__PURE__*/React.createElement("div", {
    className: "dialog-form-grid"
  }, /*#__PURE__*/React.createElement(Input, {
    id: "upstream-host",
    label: "\u4E0A\u6E38\u5730\u5740",
    value: host,
    onChange: event => setHost(event.target.value)
  }), /*#__PURE__*/React.createElement(Input, {
    id: "upstream-port",
    label: "\u4E0A\u6E38\u7AEF\u53E3",
    value: upstreamPort,
    onChange: event => setUpstreamPort(event.target.value),
    inputMode: "numeric"
  })), /*#__PURE__*/React.createElement("div", {
    className: "dialog-route-preview mono"
  }, "127.0.0.1:", port || "—", " ", /*#__PURE__*/React.createElement("span", {
    className: "forward-relation__arrow"
  }, "\u2192"), " ", host || "—", ":", upstreamPort || "—"), /*#__PURE__*/React.createElement("div", {
    className: "jb-setting-row"
  }, /*#__PURE__*/React.createElement("div", {
    className: "jb-setting-copy"
  }, /*#__PURE__*/React.createElement("span", null, "\u5065\u5EB7\u68C0\u67E5"), /*#__PURE__*/React.createElement("small", null, "\u5B9A\u65F6\u63A2\u6D4B\u4E0A\u6E38\u53EF\u8FBE\u6027\uFF0C\u5728\u5217\u8868\u4E2D\u663E\u793A\u72B6\u6001")), /*#__PURE__*/React.createElement(Switch, {
    checked: true,
    onChange: () => {},
    label: "\u542F\u7528\u5065\u5EB7\u68C0\u67E5"
  })), error && /*#__PURE__*/React.createElement(Notice, {
    tone: "danger"
  }, error));
}
Object.assign(window, {
  MappingsPage,
  MappingDialog
});
})(); } catch (e) { __ds_ns.__errors.push({ path: "ui_kits/workbench/mappings.jsx", error: String((e && e.message) || e) }); }

// ui_kits/workbench/observability.jsx
try { (() => {
const {
  Button,
  Panel,
  Badge,
  Input,
  Switch,
  Segmented
} = DS;
const logRows = [["17:31:08", "info", "配置应用成功，4 条映射"], ["17:30:52", "info", "已保存映射「日志收集」"], ["17:29:16", "warn", "Redis 缓存健康检查失败：连接超时"], ["17:26:04", "info", "转发引擎已启动 · 内置引擎（Go）"], ["17:21:47", "error", "配置校验失败：监听端口 9100 已被占用"]];
function LogsPage({
  onAction
}) {
  const [tab, setTab] = React.useState("操作日志");
  const [level, setLevel] = React.useState("全部");
  const [query, setQuery] = React.useState("");
  const [readAccess, setReadAccess] = React.useState(true);
  const rows = logRows.filter(row => (level === "全部" || {
    错误: "error",
    警告: "warn",
    信息: "info"
  }[level] === row[1]) && row[2].includes(query));
  return /*#__PURE__*/React.createElement("div", {
    className: "page__inner",
    "data-screen-label": "\u65E5\u5FD7"
  }, /*#__PURE__*/React.createElement("div", {
    className: "toolbar"
  }, /*#__PURE__*/React.createElement("div", {
    className: "log-tabs"
  }, /*#__PURE__*/React.createElement(Segmented, {
    options: ["操作日志", "访问日志"],
    value: tab,
    onChange: setTab,
    label: "\u65E5\u5FD7\u7C7B\u578B"
  })), tab === "操作日志" && /*#__PURE__*/React.createElement(Segmented, {
    options: ["全部", "错误", "警告", "信息"],
    value: level,
    onChange: setLevel,
    label: "\u65E5\u5FD7\u7EA7\u522B"
  }), /*#__PURE__*/React.createElement("div", {
    className: "toolbar__spacer"
  }), /*#__PURE__*/React.createElement(Button, {
    onClick: () => onAction(`已导出${tab}`)
  }, "\u5BFC\u51FA"), /*#__PURE__*/React.createElement(Button, {
    variant: "danger-quiet",
    onClick: () => onAction(`清空${tab}前需要二次确认`)
  }, "\u6E05\u7A7A")), /*#__PURE__*/React.createElement("div", {
    className: "toolbar"
  }, /*#__PURE__*/React.createElement("div", {
    className: "search"
  }, /*#__PURE__*/React.createElement("img", {
    src: asset("search"),
    alt: ""
  }), /*#__PURE__*/React.createElement("input", {
    className: "jb-input",
    value: query,
    onChange: event => setQuery(event.target.value),
    placeholder: "\u641C\u7D22\u5F53\u524D\u65E5\u5FD7",
    "aria-label": "\u641C\u7D22\u65E5\u5FD7"
  })), tab === "访问日志" && /*#__PURE__*/React.createElement(React.Fragment, null, /*#__PURE__*/React.createElement("div", {
    className: "toolbar__spacer"
  }), /*#__PURE__*/React.createElement("span", {
    className: "toolbar-caption"
  }, "\u8BFB\u53D6\u8BBF\u95EE\u65E5\u5FD7"), /*#__PURE__*/React.createElement(Switch, {
    checked: readAccess,
    onChange: setReadAccess,
    label: "\u8BFB\u53D6\u8BBF\u95EE\u65E5\u5FD7"
  }), /*#__PURE__*/React.createElement(Button, {
    onClick: () => onAction("AI 将只读取日志末尾 200 行进行解读")
  }, "AI \u89E3\u8BFB"))), /*#__PURE__*/React.createElement(Panel, {
    className: "log-panel"
  }, /*#__PURE__*/React.createElement("div", {
    className: "log-list"
  }, tab === "操作日志" && rows.map((row, index) => /*#__PURE__*/React.createElement("div", {
    className: "log-line",
    key: index
  }, /*#__PURE__*/React.createElement("time", {
    className: "mono"
  }, row[0]), /*#__PURE__*/React.createElement(Badge, null, row[1]), /*#__PURE__*/React.createElement("span", null, row[2]))), tab === "访问日志" && readAccess && [1, 2, 3, 4, 5].map(index => /*#__PURE__*/React.createElement("div", {
    className: "log-line",
    key: index
  }, /*#__PURE__*/React.createElement("time", {
    className: "mono"
  }, "17:31:", 12 + index), /*#__PURE__*/React.createElement(Badge, {
    protocol: "HTTP"
  }, "GET"), /*#__PURE__*/React.createElement("span", {
    className: "mono"
  }, "127.0.0.1 \u2192 git.example.com:8080 \xB7 200 \xB7 ", index * 124, " B"))), tab === "访问日志" && !readAccess && /*#__PURE__*/React.createElement("div", {
    className: "mapping-empty"
  }, "\u8BBF\u95EE\u65E5\u5FD7\u8BFB\u53D6\u5DF2\u5173\u95ED\u3002\u5F00\u542F\u4E0A\u65B9\u5F00\u5173\u540E\u663E\u793A\u65E5\u5FD7\u5185\u5BB9\u3002")), /*#__PURE__*/React.createElement("footer", {
    className: "panel-footer"
  }, /*#__PURE__*/React.createElement("span", null, tab, " \xB7 \u6700\u65B0\u8BB0\u5F55\u5728\u524D"), /*#__PURE__*/React.createElement("span", {
    className: "toolbar__spacer"
  }), /*#__PURE__*/React.createElement("span", null, "\u5185\u5BB9\u53EF\u9009\u4E2D\u590D\u5236"))));
}
const snapshots = [{
  id: "current",
  time: "2026-10-29 17:31",
  note: "应用配置 · 5 条规则 · 当前版本",
  added: 1,
  removed: 0
}, {
  id: "previous",
  time: "2026-10-29 16:48",
  note: "修改 MySQL 上游 · 4 条规则",
  added: 2,
  removed: 2
}, {
  id: "first",
  time: "2026-10-28 09:12",
  note: "首次启动 · 4 条规则",
  added: 4,
  removed: 0
}];
function HistoryPage({
  onAction
}) {
  const [selected, setSelected] = React.useState("current");
  const snapshot = snapshots.find(item => item.id === selected);
  return /*#__PURE__*/React.createElement("div", {
    className: "page__inner",
    "data-screen-label": "\u914D\u7F6E\u5386\u53F2"
  }, /*#__PURE__*/React.createElement("div", {
    className: "toolbar"
  }, /*#__PURE__*/React.createElement("span", {
    className: "toolbar-caption"
  }, "\u6BCF\u6B21\u6210\u529F\u5E94\u7528\u81EA\u52A8\u4FDD\u5B58\uFF0C\u53EF\u56DE\u770B\u5DEE\u5F02\u6216\u56DE\u6EDA"), /*#__PURE__*/React.createElement("div", {
    className: "toolbar__spacer"
  }), /*#__PURE__*/React.createElement(Button, {
    onClick: () => onAction("已导出当前快照对应的 nginx.conf")
  }, "\u5BFC\u51FA\u914D\u7F6E")), /*#__PURE__*/React.createElement(Panel, {
    className: "history-layout"
  }, /*#__PURE__*/React.createElement("div", {
    className: "snapshot-list"
  }, snapshots.map(item => /*#__PURE__*/React.createElement("button", {
    className: "snapshot-item",
    "aria-pressed": selected === item.id,
    key: item.id,
    onClick: () => setSelected(item.id)
  }, /*#__PURE__*/React.createElement("span", {
    className: "snapshot-item__time"
  }, item.time), /*#__PURE__*/React.createElement("span", {
    className: "snapshot-item__note"
  }, item.note), item.id === "current" && /*#__PURE__*/React.createElement(Badge, null, "\u5F53\u524D\u751F\u6548")))), /*#__PURE__*/React.createElement("div", {
    className: "snapshot-detail"
  }, /*#__PURE__*/React.createElement("div", {
    className: "snapshot-detail__head"
  }, /*#__PURE__*/React.createElement("div", null, /*#__PURE__*/React.createElement("h2", {
    className: "section-title"
  }, "\u5FEB\u7167\u5185\u5BB9"), /*#__PURE__*/React.createElement("span", {
    className: "rule-meta"
  }, snapshot.time)), /*#__PURE__*/React.createElement("div", {
    className: "snapshot-detail__actions"
  }, /*#__PURE__*/React.createElement(Badge, null, "+", snapshot.added, " \u884C"), snapshot.removed > 0 && /*#__PURE__*/React.createElement(Badge, null, "\u2212", snapshot.removed, " \u884C"), /*#__PURE__*/React.createElement(Button, {
    variant: snapshot.id === "current" ? "secondary" : "danger-quiet",
    disabled: snapshot.id === "current",
    onClick: () => onAction("回滚会先校验配置，再替换当前生效版本")
  }, "\u56DE\u6EDA\u5230\u6B64\u7248\u672C"))), /*#__PURE__*/React.createElement("pre", {
    className: "config-code"
  }, `# JumpBoard · ${snapshot.time}\nworker_processes 1;\n\nstream {\n  upstream mysql_internal {\n    server 192.168.1.10:3306;\n  }\n\n  server {\n    listen 127.0.0.1:9000;\n    proxy_pass mysql_internal;\n  }\n`, /*#__PURE__*/React.createElement("span", {
    className: "code-add"
  }, `\n  server {\n    listen 127.0.0.1:5140 udp;\n    proxy_pass 10.0.1.50:514;\n  }`), `\n}`))));
}
Object.assign(window, {
  LogsPage,
  HistoryPage
});
})(); } catch (e) { __ds_ns.__errors.push({ path: "ui_kits/workbench/observability.jsx", error: String((e && e.message) || e) }); }

// ui_kits/workbench/scene.jsx
try { (() => {
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
  return /*#__PURE__*/React.createElement("div", {
    className: "scene-picker",
    ref: rootRef
  }, /*#__PURE__*/React.createElement("button", {
    className: "scene-trigger",
    type: "button",
    "aria-haspopup": "listbox",
    "aria-expanded": open,
    "aria-controls": listId,
    onClick: () => setOpen(!open)
  }, /*#__PURE__*/React.createElement("img", {
    src: asset("relay"),
    alt: ""
  }), /*#__PURE__*/React.createElement("span", {
    className: "scene-trigger__label"
  }, "\u573A\u666F"), /*#__PURE__*/React.createElement("strong", null, scene), /*#__PURE__*/React.createElement("span", {
    className: `scene-chevron ${open ? "is-open" : ""}`,
    "aria-hidden": "true"
  }, "\u2304")), /*#__PURE__*/React.createElement(Button, {
    variant: "quiet",
    onClick: onManage
  }, "\u7BA1\u7406\u573A\u666F"), open && /*#__PURE__*/React.createElement("div", {
    className: "scene-menu",
    id: listId,
    role: "listbox",
    "aria-label": "\u9009\u62E9\u573A\u666F"
  }, /*#__PURE__*/React.createElement("div", {
    className: "scene-menu__caption"
  }, "\u5207\u6362\u6574\u5957\u6620\u5C04\uFF0C\u5F53\u524D\u6539\u52A8\u4F1A\u4FDD\u7559"), scenes.map(item => /*#__PURE__*/React.createElement("button", {
    key: item.name,
    type: "button",
    role: "option",
    "aria-selected": scene === item.name,
    className: "scene-option",
    onClick: () => {
      setOpen(false);
      onSelect(item.name);
    }
  }, /*#__PURE__*/React.createElement("span", {
    className: "scene-option__check"
  }, scene === item.name ? "✓" : ""), /*#__PURE__*/React.createElement("span", {
    className: "scene-option__name"
  }, item.name), /*#__PURE__*/React.createElement("span", {
    className: "scene-option__count"
  }, item.count, " \u6761\u89C4\u5219"), item.name === scene && /*#__PURE__*/React.createElement(Badge, null, "\u5F53\u524D")))));
}
Object.assign(window, {
  ScenePicker
});
})(); } catch (e) { __ds_ns.__errors.push({ path: "ui_kits/workbench/scene.jsx", error: String((e && e.message) || e) }); }

// ui_kits/workbench/settings.jsx
try { (() => {
const {
  Button,
  Badge,
  Input,
  Switch,
  Segmented,
  Notice
} = DS;
function SettingsPage({
  theme,
  setTheme,
  onAction
}) {
  const [section, setSection] = React.useState("general");
  const [login, setLogin] = React.useState(false);
  const [traffic, setTraffic] = React.useState(true);
  const [whitelist, setWhitelist] = React.useState(false);
  const [logRead, setLogRead] = React.useState(true);
  const [connection, setConnection] = React.useState("idle");
  const testConnection = () => {
    setConnection("busy");
    setTimeout(() => {
      setConnection("success");
      onAction("连接成功：模型服务可用，AI 入口已启用");
    }, 900);
  };
  const title = settingSections.find(item => item[0] === section)?.[1];
  return /*#__PURE__*/React.createElement("div", {
    className: "page__inner settings-layout",
    "data-screen-label": "\u8BBE\u7F6E"
  }, /*#__PURE__*/React.createElement("nav", {
    className: "settings-nav",
    "aria-label": "\u8BBE\u7F6E\u5206\u7EC4"
  }, settingSections.map(([id, label, icon]) => /*#__PURE__*/React.createElement("button", {
    key: id,
    "aria-pressed": section === id,
    onClick: () => setSection(id)
  }, /*#__PURE__*/React.createElement("img", {
    src: asset(icon),
    alt: ""
  }), label))), /*#__PURE__*/React.createElement("div", {
    className: "settings-content"
  }, /*#__PURE__*/React.createElement("div", {
    className: "settings-content__intro"
  }, /*#__PURE__*/React.createElement("h2", null, title), /*#__PURE__*/React.createElement("p", null, {
    ai: "配置兼容 OpenAI 的模型服务。只有主动使用 AI 功能时才发送对应内容。",
    general: "应用名称、外观与显示偏好。",
    access: "限制可访问所有代理的客户端 IP。",
    backup: "备份、导入和查看实际生成的配置。",
    workspace: "管理相互独立的映射场景。",
    hotkeys: "设置全局显示/隐藏窗口的快捷键。",
    updates: "检查新版本，更新前保留当前配置。",
    danger: "会删除现有数据的操作集中在这里。"
  }[section])), section === "general" && /*#__PURE__*/React.createElement(React.Fragment, null, /*#__PURE__*/React.createElement("section", {
    className: "jb-section"
  }, /*#__PURE__*/React.createElement("h3", {
    className: "section-title"
  }, "\u901A\u7528"), /*#__PURE__*/React.createElement("div", {
    className: "jb-setting-row"
  }, /*#__PURE__*/React.createElement("div", {
    className: "jb-setting-copy"
  }, /*#__PURE__*/React.createElement("span", null, "\u7F51\u5173\u540D\u79F0"), /*#__PURE__*/React.createElement("small", null, "\u7528\u4E8E\u8BC6\u522B\u8FD9\u53F0 JumpBoard \u7F51\u5173")), /*#__PURE__*/React.createElement("input", {
    className: "jb-input",
    style: {
      width: 220
    },
    defaultValue: "\u672C\u673A\u7F51\u5173",
    "aria-label": "\u7F51\u5173\u540D\u79F0"
  })), /*#__PURE__*/React.createElement("div", {
    className: "jb-setting-row"
  }, /*#__PURE__*/React.createElement("div", {
    className: "jb-setting-copy"
  }, /*#__PURE__*/React.createElement("span", null, "\u5F00\u673A\u81EA\u542F"), /*#__PURE__*/React.createElement("small", null, "\u767B\u5F55\u7CFB\u7EDF\u540E\u81EA\u52A8\u542F\u52A8 JumpBoard")), /*#__PURE__*/React.createElement(Switch, {
    checked: login,
    onChange: setLogin,
    label: "\u5F00\u673A\u81EA\u542F\u52A8"
  }))), /*#__PURE__*/React.createElement("section", {
    className: "jb-section"
  }, /*#__PURE__*/React.createElement("h3", {
    className: "section-title"
  }, "\u5916\u89C2\u4E0E\u663E\u793A"), /*#__PURE__*/React.createElement("div", {
    className: "jb-setting-row"
  }, /*#__PURE__*/React.createElement("div", {
    className: "jb-setting-copy"
  }, /*#__PURE__*/React.createElement("span", null, "\u4E3B\u9898"), /*#__PURE__*/React.createElement("small", null, "\u8DDF\u968F\u7CFB\u7EDF\u6216\u56FA\u5B9A\u4EAE\u8272/\u6697\u8272\u5916\u89C2")), /*#__PURE__*/React.createElement(Segmented, {
    options: ["跟随系统", "亮色", "暗色"],
    value: theme,
    onChange: setTheme,
    label: "\u4E3B\u9898"
  })), /*#__PURE__*/React.createElement("div", {
    className: "jb-setting-row"
  }, /*#__PURE__*/React.createElement("div", {
    className: "jb-setting-copy"
  }, /*#__PURE__*/React.createElement("span", null, "\u6D41\u91CF\u663E\u793A"), /*#__PURE__*/React.createElement("small", null, "\u6620\u5C04\u5217\u8868\u663E\u793A\u8FDE\u63A5\u6570\u548C\u6D41\u91CF\uFF1B\u4E0D\u5F71\u54CD\u8F6C\u53D1")), /*#__PURE__*/React.createElement(Switch, {
    checked: traffic,
    onChange: setTraffic,
    label: "\u663E\u793A\u6D41\u91CF"
  })))), section === "ai" && /*#__PURE__*/React.createElement(React.Fragment, null, /*#__PURE__*/React.createElement("section", {
    className: "jb-section"
  }, /*#__PURE__*/React.createElement("h3", {
    className: "section-title"
  }, "\u670D\u52A1\u8FDE\u63A5"), /*#__PURE__*/React.createElement("div", {
    style: {
      display: "flex",
      flexDirection: "column",
      gap: 16,
      marginTop: 16
    }
  }, /*#__PURE__*/React.createElement(Input, {
    id: "ai-endpoint",
    label: "\u63A5\u53E3\u5730\u5740",
    placeholder: "https://api.openai.com \u6216\u672C\u5730\u7F51\u5173"
  }), /*#__PURE__*/React.createElement("div", {
    className: "jb-field-row"
  }, /*#__PURE__*/React.createElement(Input, {
    id: "ai-model",
    label: "\u6A21\u578B",
    placeholder: "gpt-4o-mini / deepseek-chat"
  }), /*#__PURE__*/React.createElement(Input, {
    id: "ai-diagnosis",
    label: "\u8BCA\u65AD/\u89E3\u8BFB\u6A21\u578B",
    placeholder: "\u7559\u7A7A = \u4E0E\u4E3B\u6A21\u578B\u76F8\u540C"
  })), /*#__PURE__*/React.createElement(Input, {
    id: "ai-key",
    label: "API Key",
    type: "password",
    placeholder: "sk-\u2026",
    hint: "\u5B58\u50A8\u5728\u7CFB\u7EDF\u51ED\u636E\u5E93\uFF0C\u4E0D\u5199\u8FDB\u6570\u636E\u5E93\u6216\u5BFC\u51FA\u6587\u4EF6"
  }))), /*#__PURE__*/React.createElement("section", {
    className: "jb-section"
  }, /*#__PURE__*/React.createElement("h3", {
    className: "section-title"
  }, "\u6570\u636E\u4E0E\u9690\u79C1"), /*#__PURE__*/React.createElement("ul", {
    className: "privacy-list"
  }, /*#__PURE__*/React.createElement("li", null, "\u751F\u6210\u8349\u7A3F\u53EA\u53D1\u9001\u5F53\u524D\u5BF9\u8BDD\uFF0C\u4E00\u6B21\u6700\u591A\u751F\u6210 5 \u6761\u3002"), /*#__PURE__*/React.createElement("li", null, "\u5931\u8D25\u8BCA\u65AD\u53D1\u9001\u9519\u8BEF\u3001\u81F3\u591A\u4E24\u6761\u6620\u5C04\u6458\u8981\u548C\u65E5\u5FD7\u5C3E\u90E8\u3002"), /*#__PURE__*/React.createElement("li", null, "\u6587\u4EF6\u89E3\u8BFB\u53D1\u9001\u6240\u9009\u6587\u4EF6\uFF1B\u8D85\u8FC7 256 KB \u4F1A\u88AB\u62D2\u7EDD\u3002"), /*#__PURE__*/React.createElement("li", null, "\u81EA\u7136\u8BED\u8A00\u7B5B\u9009\u4F1A\u53D1\u9001\u5168\u90E8\u6620\u5C04\u6458\u8981\uFF0C\u4E0D\u542B\u5BC6\u94A5\u3002"))), /*#__PURE__*/React.createElement("div", {
    className: "settings-actions"
  }, /*#__PURE__*/React.createElement(Button, {
    disabled: connection === "busy",
    onClick: () => onAction("正在从配置的网关拉取可用模型")
  }, "\u62C9\u53D6\u6A21\u578B"), /*#__PURE__*/React.createElement(Button, {
    disabled: connection === "busy",
    onClick: testConnection
  }, connection === "busy" ? "正在连接…" : "测试连接"), /*#__PURE__*/React.createElement(Button, {
    variant: "primary",
    disabled: connection === "busy",
    onClick: () => onAction("AI 配置已保存")
  }, "\u4FDD\u5B58\u914D\u7F6E"), connection === "busy" && /*#__PURE__*/React.createElement("span", {
    className: "connection-status connection-status--busy"
  }, "\u6B63\u5728\u9A8C\u8BC1\u63A5\u53E3\u4E0E\u6A21\u578B"), connection === "success" && /*#__PURE__*/React.createElement("span", {
    className: "connection-status connection-status--success"
  }, "\u2713 \u8FDE\u63A5\u6210\u529F\uFF0CAI \u5165\u53E3\u5DF2\u542F\u7528"))), section === "access" && /*#__PURE__*/React.createElement("section", {
    className: "jb-section"
  }, /*#__PURE__*/React.createElement("div", {
    className: "jb-setting-row"
  }, /*#__PURE__*/React.createElement("div", {
    className: "jb-setting-copy"
  }, /*#__PURE__*/React.createElement("span", null, "IP \u767D\u540D\u5355"), /*#__PURE__*/React.createElement("small", null, "\u5F00\u542F\u540E\u53EA\u5141\u8BB8\u5217\u8868\u4E2D\u7684 IP \u8BBF\u95EE\u6240\u6709\u4EE3\u7406")), /*#__PURE__*/React.createElement(Switch, {
    checked: whitelist,
    onChange: setWhitelist,
    label: "\u542F\u7528\u767D\u540D\u5355"
  })), whitelist && /*#__PURE__*/React.createElement(Notice, null, "\u8FD8\u6CA1\u6709 IP \u6761\u76EE\uFF1B\u6CA1\u6709\u6761\u76EE\u65F6\u5E94\u7528\u914D\u7F6E\u4F1A\u88AB\u62D2\u7EDD\uFF0C\u907F\u514D\u8BEF\u5F00\u653E\u8BBF\u95EE\u3002"), /*#__PURE__*/React.createElement("div", {
    className: "dialog-form-grid",
    style: {
      marginTop: 18
    }
  }, /*#__PURE__*/React.createElement(Input, {
    id: "allow-ip",
    label: "IP \u6216 CIDR",
    placeholder: "192.168.1.0/24"
  }), /*#__PURE__*/React.createElement(Input, {
    id: "allow-note",
    label: "\u5907\u6CE8",
    placeholder: "\u529E\u516C\u7F51"
  })), /*#__PURE__*/React.createElement(Button, {
    style: {
      marginTop: 12
    },
    onClick: () => onAction("条目会经 IP/CIDR 校验后添加")
  }, "\u6DFB\u52A0\u6761\u76EE")), section === "backup" && /*#__PURE__*/React.createElement(React.Fragment, null, /*#__PURE__*/React.createElement("section", {
    className: "jb-section"
  }, /*#__PURE__*/React.createElement("h3", {
    className: "section-title"
  }, "\u5907\u4EFD\u4E0E\u5FEB\u7167"), /*#__PURE__*/React.createElement("p", {
    className: "body-copy muted",
    style: {
      margin: "8px 0 16px"
    }
  }, "\u5BFC\u51FA\u4E0D\u542B\u5BC6\u7801\uFF1B\u5BFC\u5165\u4F1A\u5168\u91CF\u66FF\u6362\u73B0\u6709\u6620\u5C04\u4E0E\u767D\u540D\u5355\uFF0C\u64CD\u4F5C\u524D\u63D0\u4F9B\u9884\u89C8\u3002"), /*#__PURE__*/React.createElement("div", {
    className: "settings-actions"
  }, /*#__PURE__*/React.createElement(Button, {
    onClick: () => onAction("已导出完整备份（不含凭据）")
  }, "\u5BFC\u51FA\u5907\u4EFD"), /*#__PURE__*/React.createElement(Button, {
    onClick: () => onAction("选择备份后先预览，再确认导入")
  }, "\u5BFC\u5165\u5907\u4EFD"), /*#__PURE__*/React.createElement(Button, {
    onClick: () => onAction("备份映射将写入一个新场景，不影响当前场景")
  }, "\u5BFC\u5165\u4E3A\u65B0\u573A\u666F"))), /*#__PURE__*/React.createElement("section", {
    className: "jb-section"
  }, /*#__PURE__*/React.createElement("h3", {
    className: "section-title"
  }, "\u914D\u7F6E\u4E0E\u6570\u636E"), /*#__PURE__*/React.createElement("div", {
    className: "jb-setting-row"
  }, /*#__PURE__*/React.createElement("div", {
    className: "jb-setting-copy"
  }, /*#__PURE__*/React.createElement("span", null, "\u6807\u51C6 nginx \u914D\u7F6E"), /*#__PURE__*/React.createElement("small", null, "\u5BFC\u51FA\u6216\u67E5\u770B\u5F15\u64CE\u5B9E\u9645\u751F\u6210\u7684 nginx.conf")), /*#__PURE__*/React.createElement("div", {
    className: "settings-actions"
  }, /*#__PURE__*/React.createElement(Button, {
    onClick: () => onAction("已打开只读配置视图")
  }, "\u67E5\u770B"), /*#__PURE__*/React.createElement(Button, {
    onClick: () => onAction("已导出 nginx.conf")
  }, "\u5BFC\u51FA"))), /*#__PURE__*/React.createElement("div", {
    className: "jb-setting-row"
  }, /*#__PURE__*/React.createElement("div", {
    className: "jb-setting-copy"
  }, /*#__PURE__*/React.createElement("span", null, "\u6570\u636E\u76EE\u5F55"), /*#__PURE__*/React.createElement("small", {
    className: "mono"
  }, "./data")), /*#__PURE__*/React.createElement(Button, {
    onClick: () => onAction("数据目录已复制")
  }, "\u590D\u5236\u8DEF\u5F84")))), section === "workspace" && /*#__PURE__*/React.createElement("section", {
    className: "jb-section"
  }, /*#__PURE__*/React.createElement("div", {
    className: "jb-setting-row"
  }, /*#__PURE__*/React.createElement("div", {
    className: "jb-setting-copy"
  }, /*#__PURE__*/React.createElement("span", null, "\u9ED8\u8BA4"), /*#__PURE__*/React.createElement("small", null, "\u5F53\u524D\u573A\u666F \xB7 5 \u6761\u6620\u5C04")), /*#__PURE__*/React.createElement(Badge, null, "\u5F53\u524D")), /*#__PURE__*/React.createElement("div", {
    className: "jb-setting-row"
  }, /*#__PURE__*/React.createElement("div", {
    className: "jb-setting-copy"
  }, /*#__PURE__*/React.createElement("span", null, "\u5F00\u53D1\u73AF\u5883"), /*#__PURE__*/React.createElement("small", null, "\u72EC\u7ACB\u6620\u5C04\u3001\u767D\u540D\u5355\u4E0E\u914D\u7F6E\u5386\u53F2")), /*#__PURE__*/React.createElement(Button, {
    onClick: () => onAction("已切换到开发环境场景")
  }, "\u5207\u6362")), /*#__PURE__*/React.createElement(Button, {
    style: {
      marginTop: 16
    },
    onClick: () => onAction("新场景需要名称，创建后可独立导入映射")
  }, "\u65B0\u5EFA\u573A\u666F")), section === "hotkeys" && /*#__PURE__*/React.createElement(React.Fragment, null, /*#__PURE__*/React.createElement("section", {
    className: "jb-section"
  }, /*#__PURE__*/React.createElement("div", {
    className: "jb-setting-row"
  }, /*#__PURE__*/React.createElement("div", {
    className: "jb-setting-copy"
  }, /*#__PURE__*/React.createElement("span", null, "\u5168\u5C40\u663E\u793A/\u9690\u85CF\u4E3B\u7A97\u53E3"), /*#__PURE__*/React.createElement("small", null, "\u8F93\u5165\u52A0\u901F\u5668\u540E\u4FDD\u5B58\uFF0C\u6CE8\u518C\u5931\u8D25\u4F1A\u8BF4\u660E\u539F\u56E0")), /*#__PURE__*/React.createElement("input", {
    className: "jb-input mono",
    style: {
      width: 220
    },
    defaultValue: "CmdOrCtrl+Shift+Alt+K",
    "aria-label": "\u5168\u5C40\u5FEB\u6377\u952E"
  })), /*#__PURE__*/React.createElement(Button, {
    onClick: () => onAction("快捷键已保存")
  }, "\u4FDD\u5B58\u5FEB\u6377\u952E")), /*#__PURE__*/React.createElement("section", {
    className: "jb-section"
  }, /*#__PURE__*/React.createElement("h3", {
    className: "section-title"
  }, "\u5E94\u7528\u5185\u5FEB\u6377\u952E"), /*#__PURE__*/React.createElement("div", {
    className: "jb-setting-row"
  }, /*#__PURE__*/React.createElement("span", null, "\u65B0\u5EFA\u6620\u5C04"), /*#__PURE__*/React.createElement("kbd", {
    className: "mono"
  }, "\u2318 N")), /*#__PURE__*/React.createElement("div", {
    className: "jb-setting-row"
  }, /*#__PURE__*/React.createElement("span", null, "\u5E94\u7528\u914D\u7F6E"), /*#__PURE__*/React.createElement("kbd", {
    className: "mono"
  }, "\u2318 R")))), section === "updates" && /*#__PURE__*/React.createElement("section", {
    className: "jb-section"
  }, /*#__PURE__*/React.createElement("div", {
    className: "jb-setting-row"
  }, /*#__PURE__*/React.createElement("div", {
    className: "jb-setting-copy"
  }, /*#__PURE__*/React.createElement("span", null, "JumpBoard 1.2.0"), /*#__PURE__*/React.createElement("small", null, "\u5F53\u524D\u5DF2\u662F\u6700\u65B0\u7248\u672C\u3002\u672A\u7B7E\u540D\u7248\u672C\u7684\u5B89\u88C5\u8BF4\u660E\u4F1A\u968F\u4E0B\u8F7D\u63D0\u4F9B\u3002")), /*#__PURE__*/React.createElement(Button, {
    onClick: () => onAction("已是最新版本")
  }, "\u68C0\u67E5\u66F4\u65B0"))), section === "danger" && /*#__PURE__*/React.createElement("div", {
    className: "settings-danger"
  }, /*#__PURE__*/React.createElement("h3", {
    className: "section-title text-danger"
  }, "\u6E05\u7A7A\u6240\u6709\u6570\u636E"), /*#__PURE__*/React.createElement("p", null, "\u5220\u9664\u6620\u5C04\u3001\u65E5\u5FD7\u3001\u5FEB\u7167\u548C\u767D\u540D\u5355\uFF0C\u65E0\u6CD5\u64A4\u9500\u3002\u6B64\u64CD\u4F5C\u4E0D\u4F1A\u5F71\u54CD\u7CFB\u7EDF\u5176\u4ED6\u6587\u4EF6\u3002"), /*#__PURE__*/React.createElement(Button, {
    variant: "danger",
    onClick: () => onAction("危险操作必须确认后才会执行")
  }, "\u6E05\u7A7A\u6240\u6709\u6570\u636E"))));
}
Object.assign(window, {
  SettingsPage
});
})(); } catch (e) { __ds_ns.__errors.push({ path: "ui_kits/workbench/settings.jsx", error: String((e && e.message) || e) }); }

// ui_kits/workbench/tools.jsx
try { (() => {
const {
  Button,
  IconButton,
  Panel,
  Badge,
  Segmented
} = DS;
function TerminalPage({
  onAction
}) {
  const [newKind, setNewKind] = React.useState("本地终端");
  const [tabs, setTabs] = React.useState([{
    id: 1,
    name: "本地终端",
    kind: "本地终端"
  }]);
  const [active, setActive] = React.useState(1);
  const newTab = () => {
    const id = Date.now();
    setTabs([...tabs, {
      id,
      name: newKind,
      kind: newKind
    }]);
    setActive(id);
  };
  const activeTab = tabs.find(tab => tab.id === active);
  return /*#__PURE__*/React.createElement("div", {
    className: "page__inner",
    "data-screen-label": "\u7EC8\u7AEF"
  }, /*#__PURE__*/React.createElement("div", {
    className: "toolbar"
  }, /*#__PURE__*/React.createElement(Segmented, {
    options: ["本地终端", "端口探测", "串口"],
    value: newKind,
    onChange: setNewKind,
    label: "\u65B0\u4F1A\u8BDD\u7C7B\u578B"
  }), /*#__PURE__*/React.createElement(Button, {
    variant: "primary",
    onClick: newTab
  }, "\u65B0\u5EFA\u4F1A\u8BDD"), /*#__PURE__*/React.createElement("div", {
    className: "toolbar__spacer"
  }), tabs.length > 0 && /*#__PURE__*/React.createElement(Button, {
    variant: "quiet",
    onClick: () => setTabs([])
  }, "\u5168\u90E8\u5173\u95ED")), /*#__PURE__*/React.createElement(Panel, {
    className: "terminal-workspace"
  }, /*#__PURE__*/React.createElement("div", {
    className: "terminal-tabs"
  }, tabs.map(tab => /*#__PURE__*/React.createElement("div", {
    className: `terminal-tab ${tab.id === active ? "is-active" : ""}`,
    key: tab.id
  }, /*#__PURE__*/React.createElement("button", {
    className: "terminal-tab__select",
    "aria-pressed": tab.id === active,
    onClick: () => setActive(tab.id)
  }, tab.name), /*#__PURE__*/React.createElement("button", {
    className: "terminal-tab__close",
    "aria-label": `关闭-${tab.name}`,
    onClick: () => {
      const remaining = tabs.filter(item => item.id !== tab.id);
      setTabs(remaining);
      if (active === tab.id) setActive(remaining[0]?.id || null);
    }
  }, "\xD7"))), tabs.length > 0 && /*#__PURE__*/React.createElement("button", {
    className: "terminal-new-tab",
    "aria-label": "\u65B0\u5EFA\u4F1A\u8BDD",
    onClick: newTab
  }, "+")), /*#__PURE__*/React.createElement("div", {
    className: "terminal-content"
  }, !activeTab && /*#__PURE__*/React.createElement("div", {
    className: "terminal-empty"
  }, "\u9009\u62E9\u4E0A\u65B9\u4F1A\u8BDD\u7C7B\u578B\uFF0C\u70B9\u51FB\u300C\u65B0\u5EFA\u4F1A\u8BDD\u300D\u5F00\u59CB\u3002"), activeTab?.kind === "本地终端" && /*#__PURE__*/React.createElement(React.Fragment, null, /*#__PURE__*/React.createElement("p", {
    className: "terminal-muted"
  }, "JumpBoard \u672C\u5730\u7EC8\u7AEF \xB7 zsh"), /*#__PURE__*/React.createElement("p", {
    className: "terminal-muted"
  }, "\u8F93\u5165 exit \u7ED3\u675F\u5F53\u524D\u4F1A\u8BDD\u3002"), /*#__PURE__*/React.createElement("br", null), /*#__PURE__*/React.createElement("p", null, /*#__PURE__*/React.createElement("span", {
    className: "terminal-prompt"
  }, "lianxin@jumpboard"), " ", /*#__PURE__*/React.createElement("span", null, "~"), " % nc -vz 127.0.0.1 9000"), /*#__PURE__*/React.createElement("p", null, "Connection to 127.0.0.1 port 9000 [tcp/*] succeeded!"), /*#__PURE__*/React.createElement("p", null, /*#__PURE__*/React.createElement("span", {
    className: "terminal-prompt"
  }, "lianxin@jumpboard"), " ", /*#__PURE__*/React.createElement("span", null, "~"), " % ", /*#__PURE__*/React.createElement("span", {
    className: "terminal-cursor"
  }, "\u258B"))), activeTab?.kind === "端口探测" && /*#__PURE__*/React.createElement(React.Fragment, null, /*#__PURE__*/React.createElement("p", {
    className: "terminal-muted"
  }, "\u63A2\u6D4B\u8F6C\u53D1\u7AEF\u53E3 \xB7 \u4E0D\u542F\u52A8 SSH \u4F1A\u8BDD"), /*#__PURE__*/React.createElement("br", null), /*#__PURE__*/React.createElement("p", null, "\u76EE\u6807\uFF1AMySQL \u5185\u7F51 \xB7 127.0.0.1:9000"), /*#__PURE__*/React.createElement("p", {
    className: "terminal-prompt"
  }, "\u2713 \u8FDE\u63A5\u6210\u529F \xB7 12 ms"), /*#__PURE__*/React.createElement("br", null), /*#__PURE__*/React.createElement("p", null, "\u6700\u8FD1 5 \u6B21\u63A2\u6D4B\u5747\u6210\u529F\uFF0C\u4E0A\u6E38\u53EF\u8FBE\u3002")), activeTab?.kind === "串口" && /*#__PURE__*/React.createElement(React.Fragment, null, /*#__PURE__*/React.createElement("p", {
    className: "terminal-muted"
  }, "\u4E32\u53E3\u4F1A\u8BDD \xB7 /dev/cu.usbserial-0001 \xB7 115200"), /*#__PURE__*/React.createElement("br", null), /*#__PURE__*/React.createElement("p", null, "\u7B49\u5F85\u4E32\u53E3\u6570\u636E\u2026"))), /*#__PURE__*/React.createElement("footer", {
    className: "terminal-status"
  }, /*#__PURE__*/React.createElement("span", null, activeTab ? `${activeTab.kind} · 已连接` : "没有活动会话"), /*#__PURE__*/React.createElement("span", null, "UTF-8"), /*#__PURE__*/React.createElement("span", {
    className: "toolbar__spacer"
  }), /*#__PURE__*/React.createElement("span", null, "\u672C\u5730\u5904\u7406\uFF0C\u4E0D\u4E0A\u4F20\u7EC8\u7AEF\u5185\u5BB9"))));
}
function ChatPage({
  onAction,
  onImport,
  onClear
}) {
  const [input, setInput] = React.useState("");
  const [drafts, setDrafts] = React.useState([{
    id: "mysql",
    name: "内网 MySQL",
    protocol: "TCP",
    local: "127.0.0.1:9000",
    upstream: "192.168.1.10:3306",
    explanation: "把内网 MySQL 暴露到本机 9000 端口。"
  }, {
    id: "redis",
    name: "内网 Redis",
    protocol: "TCP",
    local: "127.0.0.1:9001",
    upstream: "192.168.1.11:6379",
    explanation: "把内网 Redis 暴露到本机 9001 端口。"
  }]);
  const send = () => {
    if (!input.trim()) return;
    setInput("");
    onAction("消息已加入对话；原型不会向外部服务发送内容");
  };
  return /*#__PURE__*/React.createElement("div", {
    className: "page__inner chat-layout",
    "data-screen-label": "AI \u5BF9\u8BDD\u4E0E\u8349\u7A3F\u5BA1\u9605"
  }, /*#__PURE__*/React.createElement(Panel, {
    className: "chat-panel"
  }, /*#__PURE__*/React.createElement("div", {
    className: "chat-panel__heading"
  }, /*#__PURE__*/React.createElement("h2", {
    className: "section-title"
  }, "\u5BF9\u8BDD"), /*#__PURE__*/React.createElement("p", null, "AI \u53EA\u751F\u6210\u8349\u7A3F\uFF0C\u4E0D\u4F1A\u76F4\u63A5\u4FEE\u6539\u6216\u5E94\u7528\u914D\u7F6E")), /*#__PURE__*/React.createElement("div", {
    className: "conversation"
  }, /*#__PURE__*/React.createElement("div", {
    className: "chat-message chat-message--user"
  }, /*#__PURE__*/React.createElement("div", {
    className: "chat-message__body"
  }, /*#__PURE__*/React.createElement("span", {
    className: "chat-message__role"
  }, "\u4F60"), /*#__PURE__*/React.createElement("p", null, "\u628A\u5185\u7F51 MySQL \u548C Redis \u5206\u522B\u66B4\u9732\u5230 9000/9001"))), /*#__PURE__*/React.createElement("div", {
    className: "chat-message"
  }, /*#__PURE__*/React.createElement("div", {
    className: "chat-message__avatar"
  }, /*#__PURE__*/React.createElement("img", {
    src: asset("chat"),
    alt: ""
  })), /*#__PURE__*/React.createElement("div", {
    className: "chat-message__body"
  }, /*#__PURE__*/React.createElement("span", {
    className: "chat-message__role"
  }, "JumpBoard AI"), /*#__PURE__*/React.createElement("p", null, "\u5DF2\u6839\u636E\u4F60\u7684\u63CF\u8FF0\u751F\u6210\u4E24\u6761 TCP \u6620\u5C04\u3002\u8BF7\u5728\u53F3\u4FA7\u68C0\u67E5\u76D1\u542C\u7AEF\u53E3\u548C\u4E0A\u6E38\u5730\u5740\uFF0C\u5355\u6761\u4FEE\u6539\u6216\u6574\u6279\u5BFC\u5165\u3002"), /*#__PURE__*/React.createElement("p", {
    style: {
      marginTop: 8
    }
  }, "\u5BFC\u5165\u540E\u8FD8\u9700\u8981\u70B9\u51FB\u300C\u5E94\u7528\u914D\u7F6E\u300D\u624D\u4F1A\u751F\u6548\u3002")))), /*#__PURE__*/React.createElement("div", {
    className: "chat-composer"
  }, /*#__PURE__*/React.createElement("div", {
    className: "chat-composer__row"
  }, /*#__PURE__*/React.createElement("textarea", {
    className: "jb-input",
    "aria-label": "\u6D88\u606F",
    placeholder: "\u63CF\u8FF0\u8981\u8F6C\u53D1\u7684\u670D\u52A1\uFF0C\u6216\u63A5\u7740\u4E0A\u4E00\u6761\u7EE7\u7EED\u804A",
    value: input,
    onChange: event => setInput(event.target.value)
  }), /*#__PURE__*/React.createElement(Button, {
    variant: "primary",
    disabled: !input.trim(),
    onClick: send
  }, "\u53D1\u9001")), /*#__PURE__*/React.createElement("p", {
    className: "chat-composer__hint"
  }, "\u4EC5\u53D1\u9001\u5F53\u524D\u5BF9\u8BDD\uFF1B\u6620\u5C04\u5217\u8868\u4E0D\u4F1A\u968F\u6D88\u606F\u4E0A\u4F20"))), /*#__PURE__*/React.createElement(Panel, {
    className: "draft-panel"
  }, /*#__PURE__*/React.createElement("div", {
    className: "draft-panel__heading"
  }, /*#__PURE__*/React.createElement("h2", {
    className: "section-title"
  }, "\u5F85\u5BA1\u8349\u7A3F ", /*#__PURE__*/React.createElement("span", {
    className: "text-tertiary"
  }, drafts.length)), /*#__PURE__*/React.createElement("p", null, "\u5BA1\u9605\u3001\u4FEE\u6539\u6216\u4E22\u5F03\uFF0C\u518D\u5BFC\u5165\u5F53\u524D\u573A\u666F")), /*#__PURE__*/React.createElement("div", {
    className: "draft-list"
  }, drafts.map(draft => /*#__PURE__*/React.createElement("div", {
    className: "draft-item",
    key: draft.id
  }, /*#__PURE__*/React.createElement("div", {
    className: "draft-item__top"
  }, /*#__PURE__*/React.createElement("strong", null, draft.name), /*#__PURE__*/React.createElement(Badge, {
    protocol: draft.protocol
  }, draft.protocol), /*#__PURE__*/React.createElement("div", {
    className: "draft-item__actions"
  }, /*#__PURE__*/React.createElement(IconButton, {
    icon: asset("edit"),
    label: `填入表单-${draft.name}`,
    onClick: () => onAction("草稿已填入普通映射表单，保存前可修改")
  }), /*#__PURE__*/React.createElement(IconButton, {
    icon: asset("delete"),
    label: `丢弃-${draft.name}`,
    onClick: () => setDrafts(drafts.filter(item => item.id !== draft.id))
  }))), /*#__PURE__*/React.createElement("div", {
    className: "draft-route mono"
  }, /*#__PURE__*/React.createElement("span", null, "\u76D1\u542C ", draft.local), /*#__PURE__*/React.createElement("span", null, "\u4E0A\u6E38 ", draft.upstream)), /*#__PURE__*/React.createElement("p", null, draft.explanation))), drafts.length === 0 && /*#__PURE__*/React.createElement("div", {
    className: "mapping-empty"
  }, "\u6682\u65E0\u5F85\u5BA1\u8349\u7A3F\u3002\u7EE7\u7EED\u5BF9\u8BDD\u53EF\u4EE5\u751F\u6210\u65B0\u7684\u6620\u5C04\u5EFA\u8BAE\u3002")), /*#__PURE__*/React.createElement("div", {
    className: "draft-panel__footer"
  }, /*#__PURE__*/React.createElement("span", {
    className: "toolbar-caption"
  }, "\u5BFC\u5165\u5230\u5F53\u524D\u573A\u666F\u300C\u9ED8\u8BA4\u300D"), /*#__PURE__*/React.createElement(Button, {
    variant: "primary",
    disabled: drafts.length === 0,
    onClick: () => {
      onImport(drafts);
      setDrafts([]);
    }
  }, "\u5BFC\u5165 ", drafts.length > 0 ? `${drafts.length} 条` : "", "\u8349\u7A3F"))));
}
Object.assign(window, {
  TerminalPage,
  ChatPage
});
})(); } catch (e) { __ds_ns.__errors.push({ path: "ui_kits/workbench/tools.jsx", error: String((e && e.message) || e) }); }

__ds_ns.Badge = __ds_scope.Badge;

__ds_ns.Button = __ds_scope.Button;

__ds_ns.IconButton = __ds_scope.IconButton;

__ds_ns.Panel = __ds_scope.Panel;

__ds_ns.Status = __ds_scope.Status;

__ds_ns.DataTable = __ds_scope.DataTable;

__ds_ns.Dialog = __ds_scope.Dialog;

__ds_ns.Notice = __ds_scope.Notice;

__ds_ns.Input = __ds_scope.Input;

__ds_ns.Segmented = __ds_scope.Segmented;

__ds_ns.Switch = __ds_scope.Switch;

__ds_ns.NavItem = __ds_scope.NavItem;

})();
