import type { Mapping } from '@ngj/shared/types'
import type { ConfGenOptions } from './confGenerator';
import type { NginxEngine } from './types'
import { createSnapshot, trimSnapshots } from '../db/snapshotRepo'
import { logger } from '../services/logger'
import { generateFullConf } from './confGenerator'
import { safeApplyConf } from './safeApply'

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
 * 应用映射变更到 nginx：生成 conf → safeApplyConf（备份→写入→校验→reload，失败回滚）。
 * 成功后存配置快照（若提供 hostId）+ 写日志。
 * conf 写入的原子性和并发互斥由 safeApplyConf 保证。
 */
export async function applyMappings(
  engine: NginxEngine,
  mappings: Mapping[],
  opts: ApplyOptions,
): Promise<ApplyResult> {
  const conf = generateFullConf(mappings, opts)

  const result = await safeApplyConf(engine, conf)

  if (result.ok) {
    // 成功：存快照 + 日志
    if (opts.hostId) {
      createSnapshot({ hostId: opts.hostId, content: conf, reason: 'reload' })
      trimSnapshots(SNAPSHOT_KEEP)
    }
    logger.info(opts.hostId ?? null, 'app', `配置应用成功，${mappings.length} 条映射`)
  }
  else {
    // 失败日志级别按阶段区分：test 失败=用户配置错(warn)，reload/io 失败=运行时问题(error)。
    // LogViewer 按 level 过滤，reload 失败若降级成 warn 会在"只看 error"下被漏掉。
    if (result.failStage === 'test') {
      logger.warn(opts.hostId ?? null, 'app', `配置校验失败: ${result.output}`)
    }
    else {
      logger.error(opts.hostId ?? null, 'app', `配置应用失败(${result.failStage ?? 'unknown'}): ${result.output}`)
    }
  }
  return { ok: result.ok, conf, output: result.output }
}
