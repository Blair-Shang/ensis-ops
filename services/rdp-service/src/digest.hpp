#pragma once

#include <cstddef>
#include <cstdint>
#include <string>
#include <string_view>

namespace ensisops::rdp {

/** SHA-1，供 WebSocket 握手。失败返回 false。 */
bool Sha1(const std::uint8_t* data, std::size_t size, std::uint8_t out[20]);

/** SHA-256，供证书 PEM 指纹。失败返回 false。 */
bool Sha256(const std::uint8_t* data, std::size_t size, std::uint8_t out[32]);

std::string Base64(const std::uint8_t* data, std::size_t size);
std::string HexLower(const std::uint8_t* data, std::size_t size);

}  // namespace ensisops::rdp
