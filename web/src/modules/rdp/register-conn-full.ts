import { registerConnectionNavStrategy } from '@/modules/ops/connection-nav/registry'
import { rdpConnectionNavStrategy } from '@/modules/rdp/conn-nav-strategy'
import { registerForm } from '@/modules/rdp/register-conn-form'

let registered = false

/** RDP 完整自注册。 */
export function registerFull(): void {
  if (registered) {
    return
  }
  registered = true
  registerForm()
  registerConnectionNavStrategy('rdp', rdpConnectionNavStrategy)
}
