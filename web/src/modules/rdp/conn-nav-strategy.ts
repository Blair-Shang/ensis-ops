import type { ConnectionNavStrategy } from '@/modules/ops/connection-nav/types'
import { kindIcon } from '@/modules/ops/types'

/** RDP：每次从连接树打开都新建 Tab，每 Tab 一条桌面会话。 */
export const rdpConnectionNavStrategy: ConnectionNavStrategy = {
  kind: 'rdp',
  dedupFocus: false,

  buildTabSpec(item) {
    return {
      moduleId: 'rdp',
      title: item.profileName,
      icon: kindIcon('rdp'),
      props: { profileId: item.profileId },
    }
  },
}
