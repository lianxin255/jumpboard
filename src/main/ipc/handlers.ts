import { ipcMain } from 'electron'
import { IPC } from '@shared/types'
import type { MappingInput } from '@shared/types'
import { listHosts, createHost, updateHost, deleteHost, type CreateHostInput } from '../db/hostRepo'
import { listMappings, createMapping, updateMapping, deleteMapping } from '../db/mappingRepo'
import { setActiveEngine, getActiveEngine } from '../nginx/engineRegistry'

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
  ipcMain.handle(IPC.ENGINE_RELOAD, async () => { const eng = getActiveEngine(); if (eng) await eng.reload(); return true })
  ipcMain.handle(IPC.ENGINE_TEST, async () => { const eng = getActiveEngine(); return eng ? eng.test() : { ok: false, output: 'no engine' } })
  ipcMain.handle(IPC.ENGINE_STATUS, () => getActiveEngine()?.status() ?? 'unknown')
}
