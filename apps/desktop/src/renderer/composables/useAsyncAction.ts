import { toast } from '@renderer/feedback'
import { ref } from 'vue'

/**
 * 统一异步操作的 loading + 错误处理。
 *
 * 消除组件里反复出现的样板：
 *   const xxxing = ref(false)
 *   try { xxxing.value = true; await api.xxx() } catch (e:any) { toast('xxx失败: ' + e.message, 'err') } finally { xxxing.value = false }
 *
 * 用法：
 *   const { busy, run } = useAsyncAction()
 *   await run(async () => { await api.saveHost(input) }, '保存失败')
 *   <button :disabled="busy">  // busy 是 ref<boolean>，可多操作共享
 *
 * 行为约定：
 * - 成功：返回 true，busy 自动 false
 * - 失败：自动 toast(failMsg + ': ' + message)，返回 false，busy 自动 false
 * - failMsg 不传：失败时不 toast（用于静默操作），但仍返回 false
 * - message 统一用 e?.message ?? '未知错误'（修掉旧代码 e.message / err.message / '未知错误' 三种写法不一致）
 */
export function useAsyncAction() {
  const busy = ref(false)

  async function run(action: () => Promise<unknown>, failMsg?: string): Promise<boolean> {
    if (busy.value)
      return false // 防重复点击：正在执行时拒绝新调用
    busy.value = true
    try {
      await action()
      return true
    }
    catch (e: any) {
      const msg = e?.message ?? '未知错误'
      if (failMsg)
        toast(`${failMsg}: ${msg}`, 'err')
      return false
    }
    finally {
      busy.value = false
    }
  }

  return { busy, run }
}
