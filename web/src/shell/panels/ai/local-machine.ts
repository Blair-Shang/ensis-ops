/** 「这台电脑」是用户在 @ 里点明的目标，不会在空白页签时自动带上。 */

export const LOCAL_MACHINE_ID = 'machine:local'

export function isLocalMachineAttachment(item: { id?: string; kind?: string }): boolean {
  return item.kind === 'machine' || item.id === LOCAL_MACHINE_ID
}

export function localMachineAttachment(
  label: string,
  detail = 'local',
): {
  id: string
  kind: 'machine'
  label: string
  detail: string
  payload: { moduleId: 'local' }
} {
  return {
    id: LOCAL_MACHINE_ID,
    kind: 'machine',
    label,
    detail,
    payload: { moduleId: 'local' },
  }
}
