(function () {
const DS = window.JumpboardRedesign_2bf643;
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
  return React.createElement("div", {
    className: "prototype-shell"
  }, React.createElement("div", {
    className: "prototype-meta"
  }, React.createElement("strong", null, "JumpBoard / 界面重设计"), React.createElement("span", {
    className: "prototype-meta__rule"
  }), React.createElement("span", null, "克制专业 · 原生桌面工作台"), React.createElement("span", {
    className: "prototype-meta__spacer"
  }), React.createElement("span", null, "方案外观"), React.createElement("div", {
    className: "prototype-theme"
  }, ["亮色", "暗色"].map(choice => React.createElement("button", {
    key: choice,
    "aria-pressed": theme === choice,
    onClick: () => setTheme(choice)
  }, choice)))), React.createElement("div", {
    className: "desktop-window"
  }, React.createElement(AppSidebar, {
    page: page,
    onPage: setPage,
    running: running,
    onRunning: switchService,
    serviceBusy: serviceBusy
  }), React.createElement("main", {
    className: "main"
  }, React.createElement(WindowBar, {
    page: page,
    onAction: runAction,
    actionState: actionState,
    onNewMapping: () => setMappingDialog({
      mapping: null
    }),
    onClearChat: () => notify("清空前需要确认，避免误删当前对话")
  }), React.createElement("div", {
    className: "page"
  }, page === "mappings" && React.createElement(MappingsPage, {
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
  }), page === "logs" && React.createElement(LogsPage, {
    onAction: notify
  }), page === "history" && React.createElement(HistoryPage, {
    onAction: notify
  }), page === "terminal" && React.createElement(TerminalPage, {
    onAction: notify
  }), page === "chat" && React.createElement(ChatPage, {
    onAction: notify,
    onClear: () => notify("清空前需要确认"),
    onImport: setConfirmImport
  }), page === "settings" && React.createElement(SettingsPage, {
    theme: theme,
    setTheme: setTheme,
    onAction: notify
  })))), React.createElement("div", {
    className: "prototype-notes"
  }, React.createElement("span", null, "交互提案 · 非生产界面 · 不调用外部服务"), React.createElement("span", null, "六页导航、新建/编辑映射、草稿导入与设置分组均可体验")), sceneCandidate && React.createElement(Dialog, {
    title: "切换场景",
    onClose: () => setSceneCandidate(null),
    footer: React.createElement(React.Fragment, null, React.createElement(Button, {
      onClick: () => setSceneCandidate(null)
    }, "取消"), React.createElement(Button, {
      variant: "primary",
      onClick: () => {
        setScene(sceneCandidate);
        setSceneCandidate(null);
        notify(`已切换到「${sceneCandidate}」，未应用改动已保留`);
      }
    }, "保留改动并切换"))
  }, React.createElement("p", {
    className: "body-copy"
  }, "当前场景有未应用改动。切换后这些改动会保留在「", scene, "」，不会混入「", sceneCandidate, "」。")), mappingDialog && React.createElement(MappingDialog, {
    mapping: mappingDialog.mapping,
    onClose: () => setMappingDialog(null),
    onSave: saveMapping
  }), confirmStop && React.createElement(Dialog, {
    title: "停止转发服务",
    onClose: () => setConfirmStop(false),
    footer: React.createElement(React.Fragment, null, React.createElement(Button, {
      onClick: () => setConfirmStop(false)
    }, "取消"), React.createElement(Button, {
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
    }, "停止服务"))
  }, React.createElement("p", {
    className: "body-copy"
  }, "停止后所有转发规则立即失效，外部无法访问。已有配置会保留，重新启动后可恢复。")), deleteCandidate && React.createElement(Dialog, {
    title: "删除映射",
    onClose: () => setDeleteCandidate(null),
    footer: React.createElement(React.Fragment, null, React.createElement(Button, {
      onClick: () => setDeleteCandidate(null)
    }, "取消"), React.createElement(Button, {
      variant: "danger",
      onClick: () => {
        setMappings(mappings.filter(item => item.id !== deleteCandidate.id));
        setDirty(true);
        notify(`已删除「${deleteCandidate.name}」`);
        setDeleteCandidate(null);
      }
    }, "确认删除"))
  }, React.createElement("p", {
    className: "body-copy"
  }, "「", deleteCandidate.name, "」将被删除，需要重新创建。正在运行的配置要点击「应用配置」后才会同步删除。")), confirmImport && React.createElement(Dialog, {
    title: "导入映射草稿",
    onClose: () => setConfirmImport(null),
    footer: React.createElement(React.Fragment, null, React.createElement(Button, {
      onClick: () => setConfirmImport(null)
    }, "取消"), React.createElement(Button, {
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
    }, "导入"))
  }, React.createElement("p", {
    className: "body-copy"
  }, "把 ", confirmImport.length, " 条草稿写入当前场景，使用与手动新建相同的校验。导入后点击「应用配置」才会生效。")), toast && React.createElement("div", {
    className: "prototype-toast",
    role: "status"
  }, toast));
}
ReactDOM.createRoot(document.getElementById("root")).render(React.createElement(App, null));
})();
