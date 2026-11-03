(function () {
const DS = window.JumpboardRedesign_2bf643;
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
  return React.createElement("div", {
    className: "page__inner",
    "data-screen-label": "终端"
  }, React.createElement("div", {
    className: "toolbar"
  }, React.createElement(Segmented, {
    options: ["本地终端", "端口探测", "串口"],
    value: newKind,
    onChange: setNewKind,
    label: "新会话类型"
  }), React.createElement(Button, {
    variant: "primary",
    onClick: newTab
  }, "新建会话"), React.createElement("div", {
    className: "toolbar__spacer"
  }), tabs.length > 0 && React.createElement(Button, {
    variant: "quiet",
    onClick: () => setTabs([])
  }, "全部关闭")), React.createElement(Panel, {
    className: "terminal-workspace"
  }, React.createElement("div", {
    className: "terminal-tabs"
  }, tabs.map(tab => React.createElement("div", {
    className: `terminal-tab ${tab.id === active ? "is-active" : ""}`,
    key: tab.id
  }, React.createElement("button", {
    className: "terminal-tab__select",
    "aria-pressed": tab.id === active,
    onClick: () => setActive(tab.id)
  }, tab.name), React.createElement("button", {
    className: "terminal-tab__close",
    "aria-label": `关闭-${tab.name}`,
    onClick: () => {
      const remaining = tabs.filter(item => item.id !== tab.id);
      setTabs(remaining);
      if (active === tab.id) setActive(remaining[0]?.id || null);
    }
  }, "×"))), tabs.length > 0 && React.createElement("button", {
    className: "terminal-new-tab",
    "aria-label": "新建会话",
    onClick: newTab
  }, "+")), React.createElement("div", {
    className: "terminal-content"
  }, !activeTab && React.createElement("div", {
    className: "terminal-empty"
  }, "选择上方会话类型，点击「新建会话」开始。"), activeTab?.kind === "本地终端" && React.createElement(React.Fragment, null, React.createElement("p", {
    className: "terminal-muted"
  }, "JumpBoard 本地终端 · zsh"), React.createElement("p", {
    className: "terminal-muted"
  }, "输入 exit 结束当前会话。"), React.createElement("br", null), React.createElement("p", null, React.createElement("span", {
    className: "terminal-prompt"
  }, "lianxin@jumpboard"), " ", React.createElement("span", null, "~"), " % nc -vz 127.0.0.1 9000"), React.createElement("p", null, "Connection to 127.0.0.1 port 9000 [tcp/*] succeeded!"), React.createElement("p", null, React.createElement("span", {
    className: "terminal-prompt"
  }, "lianxin@jumpboard"), " ", React.createElement("span", null, "~"), " % ", React.createElement("span", {
    className: "terminal-cursor"
  }, "▋"))), activeTab?.kind === "端口探测" && React.createElement(React.Fragment, null, React.createElement("p", {
    className: "terminal-muted"
  }, "探测转发端口 · 不启动 SSH 会话"), React.createElement("br", null), React.createElement("p", null, "目标：MySQL 内网 · 127.0.0.1:9000"), React.createElement("p", {
    className: "terminal-prompt"
  }, "✓ 连接成功 · 12 ms"), React.createElement("br", null), React.createElement("p", null, "最近 5 次探测均成功，上游可达。")), activeTab?.kind === "串口" && React.createElement(React.Fragment, null, React.createElement("p", {
    className: "terminal-muted"
  }, "串口会话 · /dev/cu.usbserial-0001 · 115200"), React.createElement("br", null), React.createElement("p", null, "等待串口数据…"))), React.createElement("footer", {
    className: "terminal-status"
  }, React.createElement("span", null, activeTab ? `${activeTab.kind} · 已连接` : "没有活动会话"), React.createElement("span", null, "UTF-8"), React.createElement("span", {
    className: "toolbar__spacer"
  }), React.createElement("span", null, "本地处理，不上传终端内容"))));
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
  return React.createElement("div", {
    className: "page__inner chat-layout",
    "data-screen-label": "AI 对话与草稿审阅"
  }, React.createElement(Panel, {
    className: "chat-panel"
  }, React.createElement("div", {
    className: "chat-panel__heading"
  }, React.createElement("h2", {
    className: "section-title"
  }, "对话"), React.createElement("p", null, "AI 只生成草稿，不会直接修改或应用配置")), React.createElement("div", {
    className: "conversation"
  }, React.createElement("div", {
    className: "chat-message chat-message--user"
  }, React.createElement("div", {
    className: "chat-message__body"
  }, React.createElement("span", {
    className: "chat-message__role"
  }, "你"), React.createElement("p", null, "把内网 MySQL 和 Redis 分别暴露到 9000/9001"))), React.createElement("div", {
    className: "chat-message"
  }, React.createElement("div", {
    className: "chat-message__avatar"
  }, React.createElement("img", {
    src: asset("chat"),
    alt: ""
  })), React.createElement("div", {
    className: "chat-message__body"
  }, React.createElement("span", {
    className: "chat-message__role"
  }, "JumpBoard AI"), React.createElement("p", null, "已根据你的描述生成两条 TCP 映射。请在右侧检查监听端口和上游地址，单条修改或整批导入。"), React.createElement("p", {
    style: {
      marginTop: 8
    }
  }, "导入后还需要点击「应用配置」才会生效。")))), React.createElement("div", {
    className: "chat-composer"
  }, React.createElement("div", {
    className: "chat-composer__row"
  }, React.createElement("textarea", {
    className: "jb-input",
    "aria-label": "消息",
    placeholder: "描述要转发的服务，或接着上一条继续聊",
    value: input,
    onChange: event => setInput(event.target.value)
  }), React.createElement(Button, {
    variant: "primary",
    disabled: !input.trim(),
    onClick: send
  }, "发送")), React.createElement("p", {
    className: "chat-composer__hint"
  }, "仅发送当前对话；映射列表不会随消息上传"))), React.createElement(Panel, {
    className: "draft-panel"
  }, React.createElement("div", {
    className: "draft-panel__heading"
  }, React.createElement("h2", {
    className: "section-title"
  }, "待审草稿 ", React.createElement("span", {
    className: "text-tertiary"
  }, drafts.length)), React.createElement("p", null, "审阅、修改或丢弃，再导入当前场景")), React.createElement("div", {
    className: "draft-list"
  }, drafts.map(draft => React.createElement("div", {
    className: "draft-item",
    key: draft.id
  }, React.createElement("div", {
    className: "draft-item__top"
  }, React.createElement("strong", null, draft.name), React.createElement(Badge, {
    protocol: draft.protocol
  }, draft.protocol), React.createElement("div", {
    className: "draft-item__actions"
  }, React.createElement(IconButton, {
    icon: asset("edit"),
    label: `填入表单-${draft.name}`,
    onClick: () => onAction("草稿已填入普通映射表单，保存前可修改")
  }), React.createElement(IconButton, {
    icon: asset("delete"),
    label: `丢弃-${draft.name}`,
    onClick: () => setDrafts(drafts.filter(item => item.id !== draft.id))
  }))), React.createElement("div", {
    className: "draft-route mono"
  }, React.createElement("span", null, "监听 ", draft.local), React.createElement("span", null, "上游 ", draft.upstream)), React.createElement("p", null, draft.explanation))), drafts.length === 0 && React.createElement("div", {
    className: "mapping-empty"
  }, "暂无待审草稿。继续对话可以生成新的映射建议。")), React.createElement("div", {
    className: "draft-panel__footer"
  }, React.createElement("span", {
    className: "toolbar-caption"
  }, "导入到当前场景「默认」"), React.createElement(Button, {
    variant: "primary",
    disabled: drafts.length === 0,
    onClick: () => {
      onImport(drafts);
      setDrafts([]);
    }
  }, "导入 ", drafts.length > 0 ? `${drafts.length} 条` : "", "草稿"))));
}
Object.assign(window, {
  TerminalPage,
  ChatPage
});
})();
