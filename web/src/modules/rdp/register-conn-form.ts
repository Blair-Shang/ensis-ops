import { registerConnectionKind } from '@/modules/connection/registry'
import { registerConnectionFormAdapter } from '@/modules/ops/connection-form/registry'
import { rdpConnectionFormAdapter } from '@/modules/rdp/connection-form-adapter'
import RdpConnectionFields from '@/modules/rdp/components/RdpConnectionFields.vue'

let registered = false

/** 仅注册 RDP 连接表单。 */
export function registerForm(): void {
  if (registered) {
    return
  }
  registered = true
  registerConnectionFormAdapter('rdp', rdpConnectionFormAdapter)
  registerConnectionKind('rdp', {
    options: RdpConnectionFields,
    supportsProxy: false,
    supportsTunnel: false,
  })
}
