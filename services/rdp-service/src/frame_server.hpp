#pragma once

#include <cstddef>
#include <cstdint>
#include <string>

namespace ensisops::rdp {

/** 127.0.0.1 上的单连接 WebSocket，只推画面帧。 */
class FrameServer {
 public:
  FrameServer() = default;
  ~FrameServer();

  FrameServer(const FrameServer&) = delete;
  FrameServer& operator=(const FrameServer&) = delete;

  bool Listen(const std::string& token, std::string& error);
  int Port() const;
  std::uint64_t Generation() const;
  bool HasClient() const;
  bool Send(const std::uint8_t* data, std::size_t size);
  void Close();

 private:
  struct Impl;
  Impl* impl_ = nullptr;
};

}  // namespace ensisops::rdp
