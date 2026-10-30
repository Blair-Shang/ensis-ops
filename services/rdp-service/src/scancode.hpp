#pragma once

#include <cstdint>
#include <string_view>

namespace ensisops::rdp {

/**
 * 把浏览器 KeyboardEvent.code 映成 RDP scancode。
 * 高位 0x100 表示扩展键。未识别返回 false。
 */
bool LookupScancode(std::string_view code, std::uint32_t& scancode);

}  // namespace ensisops::rdp
