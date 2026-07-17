import type { Mapping } from '@shared/types'
import type { NginxEngine } from './types'
import { generateFullConf, type ConfGenOptions } from './confGenerator'
import { createSnapshot, trimSnapshots } from '../db/snapshotRepo'
import { logger } from '../services/logger'

export interface ApplyOptions extends ConfGenOptions {
  /** 当前 host id，用于存配置快照。不传则不存快照。 */
  hostId?: string
}

export interface ApplyResult {
  ok: boolean
  /** 生成的完整 conf（无论成功失败，调试用） */
  conf: string
  /** nginx -t 或 reload 的输出/错误 */
  output: string
}

const SNAPSHOT_KEEP = 50

/**
 * 应用映射变更到 nginx：生成 → 校验 → 写入 → reload。
 * 校验失败则原配置不动（回滚旧 conf，不 reload）。
 * 成功后存配置快照（若提供 hostId）+ 写日志。
 */
export async function applyMappings(
  engine: NginxEngine,
  mappings: Mapping[],
  opts: ApplyOptions
): Promise<ApplyResult> {
  const conf = generateFullConf(mappings, opts)

  // 1. 备份旧 conf，写入新 conf
  const oldConf = await engine.readConf()
  await engine.writeConf(conf)

  // 2. nginx -t 校验磁盘上的新 conf
  const testResult = await engine.test()
  if (!testResult.ok) {
    await engine.writeConf(oldConf)
    logger.warn(opts.hostId ?? null, 'app', `配置校验失败: ${testResult.output}`)
    return { ok: false, conf, output: testResult.output }
  }

  // 3. 校验通过，reload 让新 conf 生效
  try {
    await engine.reload()
    // 成功：存快照 + 日志
    if (opts.hostId) {
      createSnapshot({ hostId: opts.hostId, content: conf, reason: 'reload' })
      trimSnapshots(SNAPSHOT_KEEP)
    }
    logger.info(opts.hostId ?? null, 'app', `配置应用成功，${mappings.length} 条映射`)
    return { ok: true, conf, output: testResult.output }
  } catch (e: any) {
    logger.error(opts.hostId ?? null, 'app', `reload 失败: ${e?.message ?? e}`)
    return { ok: false, conf, output: e?.message ?? String(e) }
  }
}
