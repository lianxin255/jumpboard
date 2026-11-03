(function () {
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
  return React.createElement("div", {
    className: "page__inner",
    "data-screen-label": "映射工作台"
  }, React.createElement("div", {
    className: "toolbar"
  }, React.createElement(ScenePicker, {
    scene: scene,
    scenes: scenes,
    onSelect: onSceneSelect,
    onManage: onSettings
  }), React.createElement("div", {
    className: "toolbar__spacer"
  }), React.createElement(Button, {
    onClick: () => onAction("模板选择：MySQL、Redis、MongoDB、PostgreSQL、MQTT、RDP、SSH、WebSocket 等")
  }, "从模板新建"), React.createElement(Button, {
    onClick: () => onAction("粘贴导入会预览变更，逐条校验后才会写入")
  }, "粘贴导入")), React.createElement("div", {
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
    placeholder: "搜索名称、端口或上游地址",
    "aria-label": "搜索映射"
  })), React.createElement(Segmented, {
    options: ["全部", "已启用", "已停用"],
    value: filter,
    onChange: setFilter,
    label: "筛选映射"
  }), React.createElement("div", {
    className: "toolbar__spacer"
  }), React.createElement(Button, {
    variant: "quiet",
    icon: asset("chat"),
    onClick: onChat
  }, "AI 建映射"), React.createElement(Button, {
    variant: "quiet",
    onClick: () => onAction("AI 筛选只发送映射摘要，不包含密钥")
  }, "AI 筛选")), React.createElement("div", {
    className: "toolbar toolbar--chips"
  }, React.createElement("div", {
    className: "group-chips",
    role: "group",
    "aria-label": "按分组筛选"
  }, ["全部", ...new Set(mappings.map(mapping => mapping.group))].map(item => React.createElement("button", {
    key: item,
    type: "button",
    className: "group-chip",
    "aria-pressed": item === "全部" && group === "全部分组" || item === group,
    onClick: () => setGroup(item === "全部" ? "全部分组" : item)
  }, item)))), dirty && React.createElement("div", {
    className: "pending-notice"
  }, React.createElement(Notice, null, React.createElement("span", null, React.createElement("strong", null, "有未生效的改动。"), " ", React.createElement("span", {
    className: "pending-notice__detail"
  }, "监听端口、上游地址或启用状态已修改；当前运行配置不变。使用右上角「应用配置」生效。")))), React.createElement(Panel, {
    className: "mapping-panel"
  }, React.createElement(DataTable, {
    label: "端口转发规则",
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
  }, shown.map(mapping => React.createElement("tr", {
    key: mapping.id,
    className: detailId === mapping.id ? "is-selected" : ""
  }, React.createElement("td", null, React.createElement("button", {
    className: "rule-name",
    onClick: () => setDetailId(detailId === mapping.id ? null : mapping.id)
  }, mapping.name), React.createElement("span", {
    className: "rule-meta"
  }, mapping.group)), React.createElement("td", null, React.createElement(Badge, {
    protocol: mapping.protocol
  }, mapping.protocol)), React.createElement("td", null, React.createElement("div", {
    className: "forward-relation"
  }, React.createElement("div", {
    className: "forward-relation__row mono",
    title: `${mapping.local} → ${mapping.upstream}`
  }, React.createElement("span", null, mapping.local), React.createElement("span", {
    className: "forward-relation__arrow"
  }, "→"), React.createElement("span", null, mapping.upstream)), React.createElement("span", {
    className: "forward-relation__traffic"
  }, mapping.traffic))), React.createElement("td", null, React.createElement(Status, {
    tone: mapping.tone
  }, mapping.health)), React.createElement("td", null, React.createElement(Switch, {
    checked: mapping.enabled,
    onChange: enabled => toggleMapping(mapping.id, enabled),
    label: `启用-${mapping.name}`
  })), React.createElement("td", null, React.createElement("div", {
    className: "row-actions"
  }, React.createElement(IconButton, {
    icon: asset("edit"),
    label: `编辑-${mapping.name}`,
    onClick: () => onEditMapping(mapping)
  }), React.createElement(IconButton, {
    icon: asset("delete"),
    label: `删除-${mapping.name}`,
    danger: true,
    onClick: () => onDeleteMapping(mapping)
  })))))), shown.length === 0 && React.createElement("div", {
    className: "mapping-empty"
  }, "没有符合当前搜索和筛选条件的映射。"), React.createElement("footer", {
    className: "panel-footer"
  }, React.createElement("span", null, "当前场景「", scene, "」 · 显示 ", shown.length, " / ", mappings.length, " 条规则"), React.createElement("div", {
    className: "panel-footer__actions"
  }, React.createElement(Button, {
    variant: "quiet",
    onClick: () => {
      setMappings(mappings.map(mapping => ({
        ...mapping,
        enabled: true
      })));
      setDirty(true);
    }
  }, "全部启用"), React.createElement(Button, {
    variant: "quiet",
    onClick: () => {
      setMappings(mappings.map(mapping => ({
        ...mapping,
        enabled: false
      })));
      setDirty(true);
    }
  }, "全部停用")))), detail && React.createElement(Panel, {
    className: "detail-drawer"
  }, React.createElement("div", {
    className: "detail-drawer__header"
  }, React.createElement("h2", {
    className: "section-title"
  }, detail.name), React.createElement(Badge, {
    protocol: detail.protocol
  }, detail.protocol), React.createElement("span", {
    className: "toolbar__spacer"
  }), React.createElement(Status, {
    tone: detail.enabled ? "success" : "neutral"
  }, detail.enabled ? "已启用" : "已停用"), React.createElement(IconButton, {
    icon: asset("unchecked"),
    label: "关闭详情",
    onClick: () => setDetailId(null)
  })), React.createElement("div", {
    className: "detail-drawer__body"
  }, React.createElement("div", {
    className: "detail-metric"
  }, React.createElement("small", null, "本地监听"), React.createElement("strong", {
    className: "mono"
  }, detail.local)), React.createElement("div", {
    className: "detail-metric"
  }, React.createElement("small", null, "上游目标"), React.createElement("strong", {
    className: "mono"
  }, detail.upstream)), React.createElement("div", {
    className: "detail-metric"
  }, React.createElement("small", null, "连接与流量"), React.createElement("strong", null, detail.traffic))), React.createElement("div", {
    className: "detail-drawer__footer"
  }, React.createElement(Button, {
    onClick: () => onAction(`已复制 ${detail.local}`)
  }, "复制转发地址"), React.createElement(Button, {
    onClick: () => onAction("已创建规则副本")
  }, "克隆规则"), React.createElement(Button, {
    onClick: () => onEditMapping(detail)
  }, "编辑规则"))));
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
  return React.createElement(Dialog, {
    title: mapping ? "编辑映射" : "新建映射",
    onClose: onClose,
    footer: React.createElement(React.Fragment, null, React.createElement("span", {
      className: "dialog-form-footer-note"
    }, "保存只入库；应用配置后生效"), React.createElement(Button, {
      onClick: onClose
    }, "取消"), React.createElement(Button, {
      onClick: () => save(false)
    }, "保存"), React.createElement(Button, {
      variant: "primary",
      onClick: () => save(true)
    }, "保存并应用"))
  }, React.createElement("div", {
    className: "jb-field-row"
  }, React.createElement(Input, {
    id: "mapping-name",
    label: "名称",
    value: name,
    onChange: event => setName(event.target.value),
    placeholder: "例如：内网 MySQL"
  }), React.createElement(Input, {
    id: "mapping-group",
    label: "分组（可选）",
    defaultValue: mapping?.group || "",
    placeholder: "例如：数据库"
  })), React.createElement("div", {
    className: "dialog-protocol"
  }, React.createElement("span", {
    className: "text-secondary"
  }, "协议"), React.createElement(Segmented, {
    options: ["TCP", "UDP", "HTTP", "HTTPS"],
    value: protocol,
    onChange: setProtocol,
    label: "映射协议"
  })), React.createElement("div", {
    className: "jb-section",
    style: {
      marginBottom: 0
    }
  }, React.createElement(Input, {
    id: "listen-port",
    label: "本地监听端口",
    value: port,
    onChange: event => setPort(event.target.value),
    inputMode: "numeric"
  })), React.createElement("div", {
    className: "dialog-form-grid"
  }, React.createElement(Input, {
    id: "upstream-host",
    label: "上游地址",
    value: host,
    onChange: event => setHost(event.target.value)
  }), React.createElement(Input, {
    id: "upstream-port",
    label: "上游端口",
    value: upstreamPort,
    onChange: event => setUpstreamPort(event.target.value),
    inputMode: "numeric"
  })), React.createElement("div", {
    className: "dialog-route-preview mono"
  }, "127.0.0.1:", port || "—", " ", React.createElement("span", {
    className: "forward-relation__arrow"
  }, "→"), " ", host || "—", ":", upstreamPort || "—"), React.createElement("div", {
    className: "jb-setting-row"
  }, React.createElement("div", {
    className: "jb-setting-copy"
  }, React.createElement("span", null, "健康检查"), React.createElement("small", null, "定时探测上游可达性，在列表中显示状态")), React.createElement(Switch, {
    checked: true,
    onChange: () => {},
    label: "启用健康检查"
  })), error && React.createElement(Notice, {
    tone: "danger"
  }, error));
}
Object.assign(window, {
  MappingsPage,
  MappingDialog
});
})();
