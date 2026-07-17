import { ipcMain } from 'electron'
import { IPC } from '@shared/types'
import type { MappingInput } from '@shared/types'
import { listHosts, createHost, updateHost, deleteHost, getDefaultHost, type CreateHostInput } from '../db/hostRepo'
import { listMappings, createMapping, updateMapping, deleteMapping, listEnabledMappings, getMapping } from '../db/mappingRepo'
import { setActiveEngine, getActiveEngine } from '../nginx/engineRegistry'
import { listAllTemplates, listBuiltinTemplates, saveCustomTemplate, deleteCustomTemplate } from '../templates/templateRepo'
import { applyMappings } from '../nginx/applyFlow'
import { resolvePaths } from '../paths'
import { listLogs } from '../db/logRepo'
import { listSnapshots, getSnapshot } from '../db/snapshotRepo'
import { isPortAvailable } from '../services/portCheck'
import { checkMapping } from '../services/healthCheck'

export { setActiveEngine }

export function registerIpcHandlers(): void {
  // Host
  ipcMain.handle(IPC.HOST_LIST, () => listHosts())
  ipcMain.handle(IPC.HOST_CREATE, (_e, input: CreateHostInput) => createHost(input))
  ipcMain.handle(IPC.HOST_UPDATE, (_e, id: string, patch: Partial<CreateHostInput>) => updateHost(id, patch))
  ipcMain.handle(IPC.HOST_DELETE, (_e, id: string) => { deleteHost(id); return true })

  // Mapping
  ipcMain.handle(IPC.MAPPING_LIST, (_e, hostId: string) => listMappings(hostId))
  ipcMain.handle(IPC.MAPPING_CREATE, (_e, input: MappingInput) => createMapping(input))
  ipcMain.handle(IPC.MAPPING_UPDATE, (_e, id: string, patch: Partial<MappingInput>) => updateMapping(id, patch))
  ipcMain.handle(IPC.MAPPING_DELETE, (_e, id: string) => { deleteMapping(id); return true })

  // Engine
  ipcMain.handle(IPC.ENGINE_START, async () => { const eng = getActiveEngine(); if (eng) await eng.start(); return eng?.status() ?? 'unknown' })
  ipcMain.handle(IPC.ENGINE_STOP, async () => { const eng = getActiveEngine(); if (eng) await eng.stop(); return eng?.status() ?? 'unknown' })
  ipcMain.handle(IPC.ENGINE_RELOAD, async () => {
    const eng = getActiveEngine()
    if (!eng) return { ok: false, output: 'no active engine' }
    const host = getDefaultHost()
    if (!host) return { ok: false, output: 'no default host' }
    const paths = resolvePaths()
    const mappings = listEnabledMappings(host.id)
    const result = await applyMappings(eng, mappings, {
      nginxLogDir: paths.nginxLogDir,
      nginxTempDir: paths.tempDir,
      hostId: host.id
    })
    return { ok: result.ok, output: result.output }
  })
  ipcMain.handle(IPC.ENGINE_TEST, async () => { const eng = getActiveEngine(); return eng ? eng.test() : { ok: false, output: 'no engine' } })
  ipcMain.handle(IPC.ENGINE_STATUS, () => getActiveEngine()?.status() ?? 'unknown')

  // Template
  ipcMain.handle(IPC.TEMPLATE_LIST, () => listAllTemplates())
  ipcMain.handle(IPC.TEMPLATE_LIST_BUILTIN, () => listBuiltinTemplates())
  ipcMain.handle(IPC.TEMPLATE_SAVE_CUSTOM, (_e, tpl: any) => { saveCustomTemplate(tpl); return true })
  ipcMain.handle(IPC.TEMPLATE_DELETE_CUSTOM, (_e, id: string) => { deleteCustomTemplate(id); return true })

  // Log
  ipcMain.handle(IPC.LOG_LIST, (_e, hostId: string | null, limit: number) => listLogs(hostId, limit))

  // Snapshot
  ipcMain.handle(IPC.SNAPSHOT_LIST, (_e, hostId: string) => listSnapshots(hostId, 50))
  ipcMain.handle(IPC.SNAPSHOT_GET, (_e, id: string) => getSnapshot(id))
  ipcMain.handle(IPC.SNAPSHOT_ROLLBACK, async (_e, snapshotId: string) => {
    // 回滚：用快照 content 覆盖 conf → nginx -t → reload
    const snap = getSnapshot(snapshotId)
    if (!snap) return { ok: false, output: 'snapshot not found' }
    const eng = getActiveEngine()
    if (!eng) return { ok: false, output: 'no active engine' }
    const oldConf = await eng.readConf()
    await eng.writeConf(snap.content)
    const testResult = await eng.test()
    if (!testResult.ok) {
      await eng.writeConf(oldConf)
      return { ok: false, output: testResult.output }
    }
    try {
      await eng.reload()
      return { ok: true, output: testResult.output }
    } catch (e: any) {
      return { ok: false, output: e?.message ?? String(e) }
    }
  })

  // Health check
  ipcMain.handle(IPC.HEALTH_CHECK_NOW, async (_e, mappingId: string) => {
    const m = getMapping(mappingId)
    if (!m) return 'unknown'
    return await checkMapping(m)
  })

  // Port check
  ipcMain.handle(IPC.PORT_CHECK, (_e, port: number) => isPortAvailable(port))
}
