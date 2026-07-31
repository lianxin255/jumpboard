import { readdirSync, readFileSync, outputFileSync, pathExistsSync, ensureDirSync, removeSync } from 'fs-extra'
import { join, basename, extname } from 'path'
import { parse, stringify } from 'yaml'
import type { TemplateDefinition } from './types'
import { resolvePaths } from '../paths'

/**
 * 内置模板目录定位。
 * - 生产环境：electron-builder 把模板复制到 resources/bundled/templates，
 *   Electron 通过 process.resourcesPath 访问。
 * - vitest（源文件）：__dirname 就是 src/main/templates，内置模板在同级 builtin。
 * - electron-vite 编译后：__dirname 是 out/main，需回溯到 src 源文件。
 */
function getBuiltinDir(): string {
  // 打包环境：模板在 resources/bundled/templates（与 nginx 一起整合到 bundled/ 下）
  if (process.resourcesPath && process.resourcesPath !== 'None') {
    const packaged = join(process.resourcesPath, 'bundled', 'templates')
    if (pathExistsSync(packaged)) return packaged
  }
  // vitest 运行源文件时 __dirname 是 src/main/templates，内置模板就在 ./builtin
  const fromHere = join(__dirname, 'builtin')
  if (pathExistsSync(fromHere)) return fromHere
  // electron-vite 编译后（开发模式 npm run dev）__dirname 是 out/main，回溯到 src
  return join(__dirname, '..', '..', '..', 'src', 'main', 'templates', 'builtin')
}

let customDirForTest: string | null = null
export function __setCustomDirForTest(dir: string): void {
  customDirForTest = dir
}

function getCustomDir(): string {
  if (customDirForTest) return customDirForTest
  const paths = resolvePaths()
  const dir = join(paths.dataDir, 'templates', 'custom')
  ensureDirSync(dir)
  return dir
}

function loadYamlFile(filePath: string, id: string, builtin: boolean): TemplateDefinition | null {
  try {
    const content = readFileSync(filePath, 'utf-8')
    const raw = parse(content) as Partial<TemplateDefinition>
    if (!raw || !raw.mapping || !raw.mapping.protocol) return null
    return {
      id: raw.id ?? id,
      name: raw.name ?? id,
      icon: raw.icon ?? 'default',
      category: raw.category ?? '其他',
      description: raw.description ?? '',
      builtin,
      mapping: raw.mapping,
      extraDirectives: raw.extraDirectives
    }
  } catch {
    return null
  }
}

export function listBuiltinTemplates(): TemplateDefinition[] {
  const dir = getBuiltinDir()
  if (!pathExistsSync(dir)) return []
  const files = readdirSync(dir).filter(f => extname(f) === '.yml' || extname(f) === '.yaml')
  const list: TemplateDefinition[] = []
  for (const f of files) {
    const id = basename(f, extname(f))
    const tpl = loadYamlFile(join(dir, f), id, true)
    if (tpl) list.push(tpl)
  }
  return list.sort((a, b) => a.category.localeCompare(b.category, 'zh') || a.name.localeCompare(b.name, 'zh'))
}

export function listCustomTemplates(): TemplateDefinition[] {
  const dir = getCustomDir()
  if (!pathExistsSync(dir)) return []
  const files = readdirSync(dir).filter(f => extname(f) === '.yml' || extname(f) === '.yaml')
  const list: TemplateDefinition[] = []
  for (const f of files) {
    const id = basename(f, extname(f))
    const tpl = loadYamlFile(join(dir, f), id, false)
    if (tpl) list.push(tpl)
  }
  return list.sort((a, b) => a.name.localeCompare(b.name, 'zh'))
}

export function listAllTemplates(): TemplateDefinition[] {
  return [...listBuiltinTemplates(), ...listCustomTemplates()]
}

export function saveCustomTemplate(tpl: TemplateDefinition): void {
  const dir = getCustomDir()
  const data = {
    id: tpl.id,
    name: tpl.name,
    icon: tpl.icon,
    category: tpl.category,
    description: tpl.description,
    mapping: tpl.mapping,
    extraDirectives: tpl.extraDirectives
  }
  outputFileSync(join(dir, `${tpl.id}.yml`), stringify(data))
}

export function deleteCustomTemplate(id: string): void {
  const dir = getCustomDir()
  const file = join(dir, `${id}.yml`)
  if (pathExistsSync(file)) removeSync(file)
}
