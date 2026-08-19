/**
 * 协议/类别图标的 SVG path 数据。
 * 消除 TemplatePicker/MappingList/MappingForm 多处重复的内联 SVG path。
 *
 * 用法：<path :d="getProtocolIcon('tcp')" />
 * 或：import { protocolIconPaths } from '@web/utils/icons'
 */

/** 图标 path 集合（viewBox 0 0 24 24，stroke 风格） */
export const protocolIconPaths: Record<string, string> = {
  tcp: 'M5 8h14M5 12h14M5 16h14',
  udp: 'M12 9a3 3 0 1 0 0 6 3 3 0 0 0 0-6zM5 8h14M5 16h14',
  http: 'M3 12h18M12 3v18',
  https: 'M5 11h14v9H5zM8 11V7a4 4 0 0 1 8 0v4',
  network: 'M5 8h14M5 12h14M5 16h14',
  terminal: 'M7 9l3 3-3 3M13 15h4',
  monitor: 'M8 4h8v2M8 18h8v2M3 9h18M3 15h18',
  globe: 'M12 3a9 9 0 1 0 0 18 9 9 0 0 0 0-18zM3 12h18M12 3a14 14 0 0 1 0 18 14 14 0 0 1 0-18z',
  lock: 'M5 11h14v9H5zM8 11V7a4 4 0 0 1 8 0v4',
  code: 'M8 8l-4 4 4 4M16 8l4 4-4 4',
  bolt: 'M13 2L4 14h7l-2 8 9-12h-7z',
  database: 'M12 3c4.4 0 8 1.3 8 3v12c0 1.7-3.6 3-8 3s-8-1.3-8-3V6c0-1.7 3.6-3 8-3zM4 12c0 1.7 3.6 3 8 3s8-1.3 8-3',
  message: 'M4 18c4-8 12-8 16 0M4 14c4-6 12-6 16 0',
  api: 'M3 12h4l3-7 4 14 3-7h4',
  default: 'M12 5v14M5 12h14',
}

/** 取协议图标 path，未知协议回退到 default */
export function getProtocolIcon(protocol: string): string {
  return protocolIconPaths[protocol] || protocolIconPaths.default
}
