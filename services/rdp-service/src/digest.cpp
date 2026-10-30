#include "digest.hpp"

#include <openssl/evp.h>

namespace ensisops::rdp {
namespace {

bool Digest(const EVP_MD* md, const std::uint8_t* data, std::size_t size, std::uint8_t* out,
            unsigned int expect) {
  unsigned int len = 0;
  if (EVP_Digest(data, size, out, &len, md, nullptr) != 1) {
    return false;
  }
  return len == expect;
}

}  // namespace

bool Sha1(const std::uint8_t* data, std::size_t size, std::uint8_t out[20]) {
  return Digest(EVP_sha1(), data, size, out, 20);
}

bool Sha256(const std::uint8_t* data, std::size_t size, std::uint8_t out[32]) {
  return Digest(EVP_sha256(), data, size, out, 32);
}

std::string Base64(const std::uint8_t* data, std::size_t size) {
  static constexpr char kTable[] =
      "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";
  std::string out;
  out.reserve(((size + 2) / 3) * 4);
  std::size_t i = 0;
  while (i + 3 <= size) {
    const unsigned n = (data[i] << 16) | (data[i + 1] << 8) | data[i + 2];
    out.push_back(kTable[(n >> 18) & 63]);
    out.push_back(kTable[(n >> 12) & 63]);
    out.push_back(kTable[(n >> 6) & 63]);
    out.push_back(kTable[n & 63]);
    i += 3;
  }
  if (i < size) {
    unsigned n = data[i] << 16;
    out.push_back(kTable[(n >> 18) & 63]);
    if (i + 1 < size) {
      n |= data[i + 1] << 8;
      out.push_back(kTable[(n >> 12) & 63]);
      out.push_back(kTable[(n >> 6) & 63]);
      out.push_back('=');
    } else {
      out.push_back(kTable[(n >> 12) & 63]);
      out.push_back('=');
      out.push_back('=');
    }
  }
  return out;
}

std::string HexLower(const std::uint8_t* data, std::size_t size) {
  static constexpr char kHex[] = "0123456789abcdef";
  std::string out;
  out.resize(size * 2);
  for (std::size_t i = 0; i < size; ++i) {
    out[i * 2] = kHex[data[i] >> 4];
    out[i * 2 + 1] = kHex[data[i] & 0x0F];
  }
  return out;
}

}  // namespace ensisops::rdp
