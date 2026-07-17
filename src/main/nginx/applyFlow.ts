import type { Mapping } from '@shared/types'
import type { NginxEngine } from './types'
import { generateFullConf, type ConfGenOptions } from './confGenerator'

export interface ApplyResult {
  ok: boolean
  /** 生成的完整 conf（无论成功失败，调试用） */
  conf: string
  /** nginx -t 或 reload 的输出/错误 */
  output: string
}

/**
 * 应用映射变更到 nginx：生成 → 校验 → 写入 → reload。
 * 校验失败则原配置不动（回滚旧 conf，不 reload）。
 *
 * 设计权衡：nginx -t 只能校验磁盘文件，所以必须先写新 conf 再 test。
 * test 失败时磁盘有短暂的新 conf，但 nginx 没 reload 所以运行时不受影响；
 * 失败后立即用 readConf 拿回的旧 conf 重写，恢复磁盘一致性。
 */
export async function applyMappings(
  engine: NginxEngine,
  mappings: Mapping[],
  opts: ConfGenOptions
): Promise<ApplyResult> {
  const conf = generateFullConf(mappings, opts)

  // 1. 备份旧 conf，写入新 conf
  const oldConf = await engine.readConf()
  await engine.writeConf(conf)

  // 2. nginx -t 校验磁盘上的新 conf
  const testResult = await engine.test()
  if (!testResult.ok) {
    // 回滚到旧 conf，恢复磁盘一致性
    await engine.writeConf(oldConf)
    return { ok: false, conf, output: testResult.output }
  }

  // 3. 校验通过，reload 让新 conf 生效
  try {
    await engine.reload()
    return { ok: true, conf, output: testResult.output }
  } catch (e: any) {
    // reload 失败：nginx 通常保留旧配置继续跑，但磁盘已是新 conf。
    // 这里返回错误让 UI 提示，不自动回滚（reload 失败多是 nginx 进程问题，回滚 conf 无意义）。
    return { ok: false, conf, output: e?.message ?? String(e) }
  }
}
