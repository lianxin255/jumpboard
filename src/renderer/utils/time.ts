/**
 * 统一时间格式化。消除 3 个组件各自实现的不一致版本。
 *
 * 语义统一：
 * - relTime(ts)：相对时间（"3 分钟前"），基于 dayjs fromNow + zh-cn
 * - formatTime(ts)：绝对时间（今天/昨天 HH:mm:ss，否则 MM-DD HH:mm:ss）
 *
 * dayjs + relativeTime 插件 + zh-cn locale 在此文件统一初始化，组件只需 import 函数。
 */
import dayjs from 'dayjs'
import relativeTime from 'dayjs/plugin/relativeTime'
import 'dayjs/locale/zh-cn'

dayjs.extend(relativeTime)
dayjs.locale('zh-cn')

/** 相对时间：null 返回 '—'，否则 dayjs.fromNow()（中文"3 分钟前"） */
export function relTime(ts: number | null): string {
  if (!ts) return '—'
  return dayjs(ts).fromNow()
}

/** 绝对时间：今天/昨天显示 HH:mm:ss，更早显示 MM-DD HH:mm:ss */
export function formatTime(ts: number): string {
  const d = dayjs(ts)
  const now = dayjs()
  if (d.isSame(now, 'day')) return '今天 ' + d.format('HH:mm:ss')
  if (d.isSame(now.subtract(1, 'day'), 'day')) return '昨天 ' + d.format('HH:mm:ss')
  return d.format('MM-DD HH:mm:ss')
}

/** 纯时分秒（用于日志行），让调用方不必自己 import dayjs */
export function absTime(ts: number): string {
  return dayjs(ts).format('HH:mm:ss')
}
