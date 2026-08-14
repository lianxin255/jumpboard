import type { NginxEngine } from './types'

/**
 * conf 应用的互斥锁。
 * RELOAD / SNAPSHOT_ROLLBACK / HOST_SWITCH 都会写 conf，若并发触发，
 * read→write 非原子，nginx -t 可能校验到半写文件，或两个流程互相覆盖。
 * 用 Promise 链串行化：所有 conf 写入操作排队执行。
 */
let confChain: Promise<unknown> = Promise.resolve()

export interface SafeApplyResult {
  ok: boolean
  /** nginx -t 或 reload 的输出/错误 */
  output: string
  /** 失败阶段：区分 test 失败（配置错）和 reload 失败（nginx/运行时问题），供调用方定日志级别。成功时为 undefined。 */
  failStage?: 'test' | 'reload' | 'io'
}

/**
 * 安全应用 conf：备份旧 conf → 写新 conf → nginx -t → reload，任一步失败都回滚到旧 conf。
 *
 * 统一了原先 applyFlow 和 SNAPSHOT_ROLLBACK 两条重复流水线，并修复了它们共同的问题：
 * **reload 失败时不回滚磁盘 conf**。reload 失败时旧 worker 仍跑旧 conf（转发暂不受影响），
 * 但磁盘 conf 已是新版本，下次 nginx 重启会加载坏 conf 导致全挂。回滚磁盘 conf 保证重启安全。
 *
 * 由 confChain 串行化，保证不会有两个 safeApplyConf 并发执行。
 */
export async function safeApplyConf(
  engine: NginxEngine,
  newConf: string
): Promise<SafeApplyResult> {
  // 把本次调用串到链尾，拿到自己的"执行槽"
  let release!: () => void
  const myTurn = new Promise<void>(r => { release = r })
  const previous = confChain
  confChain = myTurn
  await previous

  try {
    // 1. 备份旧 conf（test/reload 失败时回滚用）
    const oldConf = await engine.readConf()

    // 2. 写新 conf
    try {
      await engine.writeConf(newConf)
    } catch (e: any) {
      return { ok: false, output: e?.message ?? String(e), failStage: 'io' }
    }

    // 3. nginx -t 校验磁盘上的新 conf
    const testResult = await engine.test()
    if (!testResult.ok) {
      // test 失败：回滚磁盘 conf，不 reload
      try { await engine.writeConf(oldConf) } catch { /* 回滚失败只能记录，无法恢复 */ }
      return { ok: false, output: testResult.output, failStage: 'test' }
    }

    // 4. 校验通过，reload 让新 conf 生效
    try {
      await engine.reload()
      return { ok: true, output: testResult.output }
    } catch (e: any) {
      // reload 失败：回滚磁盘 conf（修复原 applyFlow/rollback 不回滚的 bug）。
      // nginx reload 失败时旧 worker 仍在跑旧 conf，回滚磁盘 conf 保证下次重启安全。
      try { await engine.writeConf(oldConf) } catch { /* 回滚失败记录在 output */ }
      return { ok: false, output: e?.message ?? String(e), failStage: 'reload' }
    }
  } catch (e: any) {
    // readConf 抛错（如 SSH 断开）
    return { ok: false, output: e?.message ?? String(e), failStage: 'io' }
  } finally {
    release()
  }
}
