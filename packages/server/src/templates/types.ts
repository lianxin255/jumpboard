import type { MappingInput, Protocol } from '@ngj/shared/types'

/**
 * 模板定义：预填的 MappingInput 草稿 + 附带的 nginx 指令。
 * 用户选模板 → 跳到预填表单 → 改几个值 → 保存。
 */
export interface TemplateDefinition {
  /** 文件名 stem（不含扩展名），作为 id */
  id: string
  name: string
  /** 图标标识，前端映射到 SVG/emoji。内置模板用固定集合。 */
  icon: string
  /** 分类，用于模板选择器分组：网络层 / Web / 数据库 / 消息 */
  category: string
  description: string
  /** true = 随包发布的内置模板；false = 用户自建 */
  builtin: boolean
  /** 预填的映射字段（部分字段为 null 表示用户必填） */
  mapping: Partial<MappingInput> & {
    protocol: Protocol
  }
  /** 生成 nginx 配置时附加到 server 块的指令 */
  extraDirectives?: string
}
