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
})();
