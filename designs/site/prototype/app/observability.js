(function () {
const DS = window.JumpboardRedesign_2bf643;
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
  return React.createElement("div", {
    className: "page__inner",
    "data-screen-label": "日志"
  }, React.createElement("div", {
    className: "toolbar"
  }, React.createElement("div", {
    className: "log-tabs"
  }, React.createElement(Segmented, {
    options: ["操作日志", "访问日志"],
    value: tab,
    onChange: setTab,
    label: "日志类型"
  })), tab === "操作日志" && React.createElement(Segmented, {
    options: ["全部", "错误", "警告", "信息"],
    value: level,
    onChange: setLevel,
    label: "日志级别"
  }), React.createElement("div", {
    className: "toolbar__spacer"
  }), React.createElement(Button, {
    onClick: () => onAction(`已导出${tab}`)
  }, "导出"), React.createElement(Button, {
    variant: "danger-quiet",
    onClick: () => onAction(`清空${tab}前需要二次确认`)
  }, "清空")), React.createElement("div", {
    className: "toolbar"
  }, React.createElement("div", {
    className: "search"
  }, React.createElement("img", {
    src: asset("search"),
    alt: ""
  }), React.createElement("input", {
    className: "jb-input",
    value: query,
    onChange: event => setQuery(event.target.value),
    placeholder: "搜索当前日志",
    "aria-label": "搜索日志"
  })), tab === "访问日志" && React.createElement(React.Fragment, null, React.createElement("div", {
    className: "toolbar__spacer"
  }), React.createElement("span", {
    className: "toolbar-caption"
  }, "读取访问日志"), React.createElement(Switch, {
    checked: readAccess,
    onChange: setReadAccess,
    label: "读取访问日志"
  }), React.createElement(Button, {
    onClick: () => onAction("AI 将只读取日志末尾 200 行进行解读")
  }, "AI 解读"))), React.createElement(Panel, {
    className: "log-panel"
  }, React.createElement("div", {
    className: "log-list"
  }, tab === "操作日志" && rows.map((row, index) => React.createElement("div", {
    className: "log-line",
    key: index
  }, React.createElement("time", {
    className: "mono"
  }, row[0]), React.createElement(Badge, null, row[1]), React.createElement("span", null, row[2]))), tab === "访问日志" && readAccess && [1, 2, 3, 4, 5].map(index => React.createElement("div", {
    className: "log-line",
    key: index
  }, React.createElement("time", {
    className: "mono"
  }, "17:31:", 12 + index), React.createElement(Badge, {
    protocol: "HTTP"
  }, "GET"), React.createElement("span", {
    className: "mono"
  }, "127.0.0.1 → git.example.com:8080 · 200 · ", index * 124, " B"))), tab === "访问日志" && !readAccess && React.createElement("div", {
    className: "mapping-empty"
  }, "访问日志读取已关闭。开启上方开关后显示日志内容。")), React.createElement("footer", {
    className: "panel-footer"
  }, React.createElement("span", null, tab, " · 最新记录在前"), React.createElement("span", {
    className: "toolbar__spacer"
  }), React.createElement("span", null, "内容可选中复制"))));
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
  return React.createElement("div", {
    className: "page__inner",
    "data-screen-label": "配置历史"
  }, React.createElement("div", {
    className: "toolbar"
  }, React.createElement("span", {
    className: "toolbar-caption"
  }, "每次成功应用自动保存，可回看差异或回滚"), React.createElement("div", {
    className: "toolbar__spacer"
  }), React.createElement(Button, {
    onClick: () => onAction("已导出当前快照对应的 nginx.conf")
  }, "导出配置")), React.createElement(Panel, {
    className: "history-layout"
  }, React.createElement("div", {
    className: "snapshot-list"
  }, snapshots.map(item => React.createElement("button", {
    className: "snapshot-item",
    "aria-pressed": selected === item.id,
    key: item.id,
    onClick: () => setSelected(item.id)
  }, React.createElement("span", {
    className: "snapshot-item__time"
  }, item.time), React.createElement("span", {
    className: "snapshot-item__note"
  }, item.note), item.id === "current" && React.createElement(Badge, null, "当前生效")))), React.createElement("div", {
    className: "snapshot-detail"
  }, React.createElement("div", {
    className: "snapshot-detail__head"
  }, React.createElement("div", null, React.createElement("h2", {
    className: "section-title"
  }, "快照内容"), React.createElement("span", {
    className: "rule-meta"
  }, snapshot.time)), React.createElement("div", {
    className: "snapshot-detail__actions"
  }, React.createElement(Badge, null, "+", snapshot.added, " 行"), snapshot.removed > 0 && React.createElement(Badge, null, "−", snapshot.removed, " 行"), React.createElement(Button, {
    variant: snapshot.id === "current" ? "secondary" : "danger-quiet",
    disabled: snapshot.id === "current",
    onClick: () => onAction("回滚会先校验配置，再替换当前生效版本")
  }, "回滚到此版本"))), React.createElement("pre", {
    className: "config-code"
  }, `# JumpBoard · ${snapshot.time}\nworker_processes 1;\n\nstream {\n  upstream mysql_internal {\n    server 192.168.1.10:3306;\n  }\n\n  server {\n    listen 127.0.0.1:9000;\n    proxy_pass mysql_internal;\n  }\n`, React.createElement("span", {
    className: "code-add"
  }, `\n  server {\n    listen 127.0.0.1:5140 udp;\n    proxy_pass 10.0.1.50:514;\n  }`), `\n}`))));
}
Object.assign(window, {
  LogsPage,
  HistoryPage
});
})();
