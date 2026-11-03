(function () {
const DS = window.JumpboardRedesign_2bf643;
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
  return React.createElement("div", {
    className: "page__inner settings-layout",
    "data-screen-label": "设置"
  }, React.createElement("nav", {
    className: "settings-nav",
    "aria-label": "设置分组"
  }, settingSections.map(([id, label, icon]) => React.createElement("button", {
    key: id,
    "aria-pressed": section === id,
    onClick: () => setSection(id)
  }, React.createElement("img", {
    src: asset(icon),
    alt: ""
  }), label))), React.createElement("div", {
    className: "settings-content"
  }, React.createElement("div", {
    className: "settings-content__intro"
  }, React.createElement("h2", null, title), React.createElement("p", null, {
    ai: "配置兼容 OpenAI 的模型服务。只有主动使用 AI 功能时才发送对应内容。",
    general: "应用名称、外观与显示偏好。",
    access: "限制可访问所有代理的客户端 IP。",
    backup: "备份、导入和查看实际生成的配置。",
    workspace: "管理相互独立的映射场景。",
    hotkeys: "设置全局显示/隐藏窗口的快捷键。",
    updates: "检查新版本，更新前保留当前配置。",
    danger: "会删除现有数据的操作集中在这里。"
  }[section])), section === "general" && React.createElement(React.Fragment, null, React.createElement("section", {
    className: "jb-section"
  }, React.createElement("h3", {
    className: "section-title"
  }, "通用"), React.createElement("div", {
    className: "jb-setting-row"
  }, React.createElement("div", {
    className: "jb-setting-copy"
  }, React.createElement("span", null, "网关名称"), React.createElement("small", null, "用于识别这台 JumpBoard 网关")), React.createElement("input", {
    className: "jb-input",
    style: {
      width: 220
    },
    defaultValue: "本机网关",
    "aria-label": "网关名称"
  })), React.createElement("div", {
    className: "jb-setting-row"
  }, React.createElement("div", {
    className: "jb-setting-copy"
  }, React.createElement("span", null, "开机自启"), React.createElement("small", null, "登录系统后自动启动 JumpBoard")), React.createElement(Switch, {
    checked: login,
    onChange: setLogin,
    label: "开机自启动"
  }))), React.createElement("section", {
    className: "jb-section"
  }, React.createElement("h3", {
    className: "section-title"
  }, "外观与显示"), React.createElement("div", {
    className: "jb-setting-row"
  }, React.createElement("div", {
    className: "jb-setting-copy"
  }, React.createElement("span", null, "主题"), React.createElement("small", null, "跟随系统或固定亮色/暗色外观")), React.createElement(Segmented, {
    options: ["跟随系统", "亮色", "暗色"],
    value: theme,
    onChange: setTheme,
    label: "主题"
  })), React.createElement("div", {
    className: "jb-setting-row"
  }, React.createElement("div", {
    className: "jb-setting-copy"
  }, React.createElement("span", null, "流量显示"), React.createElement("small", null, "映射列表显示连接数和流量；不影响转发")), React.createElement(Switch, {
    checked: traffic,
    onChange: setTraffic,
    label: "显示流量"
  })))), section === "ai" && React.createElement(React.Fragment, null, React.createElement("section", {
    className: "jb-section"
  }, React.createElement("h3", {
    className: "section-title"
  }, "服务连接"), React.createElement("div", {
    style: {
      display: "flex",
      flexDirection: "column",
      gap: 16,
      marginTop: 16
    }
  }, React.createElement(Input, {
    id: "ai-endpoint",
    label: "接口地址",
    placeholder: "https://api.openai.com 或本地网关"
  }), React.createElement("div", {
    className: "jb-field-row"
  }, React.createElement(Input, {
    id: "ai-model",
    label: "模型",
    placeholder: "gpt-4o-mini / deepseek-chat"
  }), React.createElement(Input, {
    id: "ai-diagnosis",
    label: "诊断/解读模型",
    placeholder: "留空 = 与主模型相同"
  })), React.createElement(Input, {
    id: "ai-key",
    label: "API Key",
    type: "password",
    placeholder: "sk-…",
    hint: "存储在系统凭据库，不写进数据库或导出文件"
  }))), React.createElement("section", {
    className: "jb-section"
  }, React.createElement("h3", {
    className: "section-title"
  }, "数据与隐私"), React.createElement("ul", {
    className: "privacy-list"
  }, React.createElement("li", null, "生成草稿只发送当前对话，一次最多生成 5 条。"), React.createElement("li", null, "失败诊断发送错误、至多两条映射摘要和日志尾部。"), React.createElement("li", null, "文件解读发送所选文件；超过 256 KB 会被拒绝。"), React.createElement("li", null, "自然语言筛选会发送全部映射摘要，不含密钥。"))), React.createElement("div", {
    className: "settings-actions"
  }, React.createElement(Button, {
    disabled: connection === "busy",
    onClick: () => onAction("正在从配置的网关拉取可用模型")
  }, "拉取模型"), React.createElement(Button, {
    disabled: connection === "busy",
    onClick: testConnection
  }, connection === "busy" ? "正在连接…" : "测试连接"), React.createElement(Button, {
    variant: "primary",
    disabled: connection === "busy",
    onClick: () => onAction("AI 配置已保存")
  }, "保存配置"), connection === "busy" && React.createElement("span", {
    className: "connection-status connection-status--busy"
  }, "正在验证接口与模型"), connection === "success" && React.createElement("span", {
    className: "connection-status connection-status--success"
  }, "✓ 连接成功，AI 入口已启用"))), section === "access" && React.createElement("section", {
    className: "jb-section"
  }, React.createElement("div", {
    className: "jb-setting-row"
  }, React.createElement("div", {
    className: "jb-setting-copy"
  }, React.createElement("span", null, "IP 白名单"), React.createElement("small", null, "开启后只允许列表中的 IP 访问所有代理")), React.createElement(Switch, {
    checked: whitelist,
    onChange: setWhitelist,
    label: "启用白名单"
  })), whitelist && React.createElement(Notice, null, "还没有 IP 条目；没有条目时应用配置会被拒绝，避免误开放访问。"), React.createElement("div", {
    className: "dialog-form-grid",
    style: {
      marginTop: 18
    }
  }, React.createElement(Input, {
    id: "allow-ip",
    label: "IP 或 CIDR",
    placeholder: "192.168.1.0/24"
  }), React.createElement(Input, {
    id: "allow-note",
    label: "备注",
    placeholder: "办公网"
  })), React.createElement(Button, {
    style: {
      marginTop: 12
    },
    onClick: () => onAction("条目会经 IP/CIDR 校验后添加")
  }, "添加条目")), section === "backup" && React.createElement(React.Fragment, null, React.createElement("section", {
    className: "jb-section"
  }, React.createElement("h3", {
    className: "section-title"
  }, "备份与快照"), React.createElement("p", {
    className: "body-copy muted",
    style: {
      margin: "8px 0 16px"
    }
  }, "导出不含密码；导入会全量替换现有映射与白名单，操作前提供预览。"), React.createElement("div", {
    className: "settings-actions"
  }, React.createElement(Button, {
    onClick: () => onAction("已导出完整备份（不含凭据）")
  }, "导出备份"), React.createElement(Button, {
    onClick: () => onAction("选择备份后先预览，再确认导入")
  }, "导入备份"), React.createElement(Button, {
    onClick: () => onAction("备份映射将写入一个新场景，不影响当前场景")
  }, "导入为新场景"))), React.createElement("section", {
    className: "jb-section"
  }, React.createElement("h3", {
    className: "section-title"
  }, "配置与数据"), React.createElement("div", {
    className: "jb-setting-row"
  }, React.createElement("div", {
    className: "jb-setting-copy"
  }, React.createElement("span", null, "标准 nginx 配置"), React.createElement("small", null, "导出或查看引擎实际生成的 nginx.conf")), React.createElement("div", {
    className: "settings-actions"
  }, React.createElement(Button, {
    onClick: () => onAction("已打开只读配置视图")
  }, "查看"), React.createElement(Button, {
    onClick: () => onAction("已导出 nginx.conf")
  }, "导出"))), React.createElement("div", {
    className: "jb-setting-row"
  }, React.createElement("div", {
    className: "jb-setting-copy"
  }, React.createElement("span", null, "数据目录"), React.createElement("small", {
    className: "mono"
  }, "./data")), React.createElement(Button, {
    onClick: () => onAction("数据目录已复制")
  }, "复制路径")))), section === "workspace" && React.createElement("section", {
    className: "jb-section"
  }, React.createElement("div", {
    className: "jb-setting-row"
  }, React.createElement("div", {
    className: "jb-setting-copy"
  }, React.createElement("span", null, "默认"), React.createElement("small", null, "当前场景 · 5 条映射")), React.createElement(Badge, null, "当前")), React.createElement("div", {
    className: "jb-setting-row"
  }, React.createElement("div", {
    className: "jb-setting-copy"
  }, React.createElement("span", null, "开发环境"), React.createElement("small", null, "独立映射、白名单与配置历史")), React.createElement(Button, {
    onClick: () => onAction("已切换到开发环境场景")
  }, "切换")), React.createElement(Button, {
    style: {
      marginTop: 16
    },
    onClick: () => onAction("新场景需要名称，创建后可独立导入映射")
  }, "新建场景")), section === "hotkeys" && React.createElement(React.Fragment, null, React.createElement("section", {
    className: "jb-section"
  }, React.createElement("div", {
    className: "jb-setting-row"
  }, React.createElement("div", {
    className: "jb-setting-copy"
  }, React.createElement("span", null, "全局显示/隐藏主窗口"), React.createElement("small", null, "输入加速器后保存，注册失败会说明原因")), React.createElement("input", {
    className: "jb-input mono",
    style: {
      width: 220
    },
    defaultValue: "CmdOrCtrl+Shift+Alt+K",
    "aria-label": "全局快捷键"
  })), React.createElement(Button, {
    onClick: () => onAction("快捷键已保存")
  }, "保存快捷键")), React.createElement("section", {
    className: "jb-section"
  }, React.createElement("h3", {
    className: "section-title"
  }, "应用内快捷键"), React.createElement("div", {
    className: "jb-setting-row"
  }, React.createElement("span", null, "新建映射"), React.createElement("kbd", {
    className: "mono"
  }, "⌘ N")), React.createElement("div", {
    className: "jb-setting-row"
  }, React.createElement("span", null, "应用配置"), React.createElement("kbd", {
    className: "mono"
  }, "⌘ R")))), section === "updates" && React.createElement("section", {
    className: "jb-section"
  }, React.createElement("div", {
    className: "jb-setting-row"
  }, React.createElement("div", {
    className: "jb-setting-copy"
  }, React.createElement("span", null, "JumpBoard 1.2.0"), React.createElement("small", null, "当前已是最新版本。未签名版本的安装说明会随下载提供。")), React.createElement(Button, {
    onClick: () => onAction("已是最新版本")
  }, "检查更新"))), section === "danger" && React.createElement("div", {
    className: "settings-danger"
  }, React.createElement("h3", {
    className: "section-title text-danger"
  }, "清空所有数据"), React.createElement("p", null, "删除映射、日志、快照和白名单，无法撤销。此操作不会影响系统其他文件。"), React.createElement(Button, {
    variant: "danger",
    onClick: () => onAction("危险操作必须确认后才会执行")
  }, "清空所有数据"))));
}
Object.assign(window, {
  SettingsPage
});
})();
