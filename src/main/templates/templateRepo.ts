import { readdirSync, readFileSync, writeFileSync, existsSync, mkdirSync, rmSync } from 'fs'
import { join, basename, extname } from 'path'
import { parse, stringify } from 'yaml'
import type { TemplateDefinition } from './types'
import { resolvePaths } from '../paths'

/**
 * 内置模板目录定位。
 * - 生产环境：electron-builder 把模板复制到 resources/templates/builtin，
 *   Electron 通过 process.resourcesPath 访问。
 * - vitest（源文件）：__dirname 就是 src/main/templates，内置模板在同级 builtin。
 * - electron-vite 编译后：__dirname 是 out/main，需回溯到 src 源文件。
 *
 * 先用 `__dirname/builtin` 这条最稳的路径探测；找不到再回退到编译模式的相对路径。
 */
function getBuiltinDir(): string {
  if (process.env.NODE_ENV === 'production') {
    return join(process.resourcesPath, 'templates', 'builtin')
  }
  // vitest 运行源文件时 __dirname 是 src/main/templates，内置模板就在 ./builtin
  const fromHere = join(__dirname, 'builtin')
  if (existsSync(fromHere)) return fromHere
  // electron-vite 编译后 __dirname 是 out/main，回溯到项目根再进 src
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
  if (!existsSync(dir)) mkdirSync(dir, { recursive: true })
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
  if (!existsSync(dir)) return []
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
  if (!existsSync(dir)) return []
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
  if (!existsSync(dir)) mkdirSync(dir, { recursive: true })
  const data = {
    id: tpl.id,
    name: tpl.name,
    icon: tpl.icon,
    category: tpl.category,
    description: tpl.description,
    mapping: tpl.mapping,
    extraDirectives: tpl.extraDirectives
  }
  writeFileSync(join(dir, `${tpl.id}.yml`), stringify(data), 'utf-8')
}

export function deleteCustomTemplate(id: string): void {
  const dir = getCustomDir()
  const file = join(dir, `${id}.yml`)
  if (existsSync(file)) rmSync(file)
}
