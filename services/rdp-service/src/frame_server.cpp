#include "frame_server.hpp"

#include "digest.hpp"

#include <atomic>
#include <cerrno>
#include <cstring>
#include <mutex>
#include <thread>
#include <vector>

#if defined(_WIN32)
#ifndef NOMINMAX
#define NOMINMAX
#endif
#ifndef WIN32_LEAN_AND_MEAN
#define WIN32_LEAN_AND_MEAN
#endif
#include <winsock2.h>
#include <ws2tcpip.h>
#else
#include <arpa/inet.h>
#include <netinet/in.h>
#include <netinet/tcp.h>
#include <sys/socket.h>
#include <sys/time.h>
#include <unistd.h>
#endif

namespace ensisops::rdp {
namespace {

#if defined(_WIN32)
using Socket = SOCKET;
constexpr Socket kInvalid = INVALID_SOCKET;
int LastSockErr() { return WSAGetLastError(); }
void CloseSock(Socket s) {
  if (s != kInvalid) {
    closesocket(s);
  }
}
bool Startup() {
  static std::once_flag once;
  static bool ok = false;
  std::call_once(once, [] {
    WSADATA data;
    ok = WSAStartup(MAKEWORD(2, 2), &data) == 0;
  });
  return ok;
}
#else
using Socket = int;
constexpr Socket kInvalid = -1;
int LastSockErr() { return errno; }
void CloseSock(Socket s) {
  if (s != kInvalid) {
    ::close(s);
  }
}
bool Startup() { return true; }
#endif

bool SendAll(Socket sock, const std::uint8_t* data, std::size_t size) {
  std::size_t sent = 0;
  while (sent < size) {
#if defined(_WIN32)
    const int n = ::send(sock, reinterpret_cast<const char*>(data + sent),
                         static_cast<int>(size - sent), 0);
#else
#if defined(MSG_NOSIGNAL)
    const ssize_t n = ::send(sock, data + sent, size - sent, MSG_NOSIGNAL);
#else
    const ssize_t n = ::send(sock, data + sent, size - sent, 0);
#endif
#endif
    if (n <= 0) {
      return false;
    }
    sent += static_cast<std::size_t>(n);
  }
  return true;
}

bool ReadExact(Socket sock, std::uint8_t* data, std::size_t size) {
  std::size_t got = 0;
  while (got < size) {
#if defined(_WIN32)
    const int n = ::recv(sock, reinterpret_cast<char*>(data + got), static_cast<int>(size - got), 0);
#else
    const ssize_t n = ::recv(sock, data + got, size - got, 0);
#endif
    if (n <= 0) {
      return false;
    }
    got += static_cast<std::size_t>(n);
  }
  return true;
}

std::vector<std::uint8_t> WsFrameOpcode(std::uint8_t opcode, const std::uint8_t* data, std::size_t size) {
  std::vector<std::uint8_t> out;
  out.reserve(size + 14);
  out.push_back(static_cast<std::uint8_t>(0x80 | opcode));
  if (size < 126) {
    out.push_back(static_cast<std::uint8_t>(size));
  } else if (size <= 0xFFFF) {
    out.push_back(126);
    out.push_back(static_cast<std::uint8_t>((size >> 8) & 0xFF));
    out.push_back(static_cast<std::uint8_t>(size & 0xFF));
  } else {
    out.push_back(127);
    for (int shift = 56; shift >= 0; shift -= 8) {
      out.push_back(static_cast<std::uint8_t>((size >> shift) & 0xFF));
    }
  }
  if (data != nullptr && size > 0) {
    out.insert(out.end(), data, data + size);
  }
  return out;
}

std::vector<std::uint8_t> WsFrame(const std::uint8_t* data, std::size_t size) {
  return WsFrameOpcode(0x2, data, size);
}

/** 读掉浏览器发来的帧，并回答 Ping，避免接收缓冲塞满。 */
bool DrainClient(Socket sock, std::mutex* send_mu) {
  while (true) {
    std::uint8_t header[2];
    if (!ReadExact(sock, header, 2)) {
      return false;
    }
    const std::uint8_t opcode = header[0] & 0x0F;
    std::uint64_t length = header[1] & 0x7F;
    const bool masked = (header[1] & 0x80) != 0;
    if (length == 126) {
      std::uint8_t ext[2];
      if (!ReadExact(sock, ext, 2)) {
        return false;
      }
      length = (static_cast<std::uint64_t>(ext[0]) << 8) | ext[1];
    } else if (length == 127) {
      std::uint8_t ext[8];
      if (!ReadExact(sock, ext, 8)) {
        return false;
      }
      length = 0;
      for (int i = 0; i < 8; ++i) {
        length = (length << 8) | ext[i];
      }
    }
    if (length > 1024 * 1024) {
      return false;
    }
    std::uint8_t mask[4] = {};
    if (masked && !ReadExact(sock, mask, 4)) {
      return false;
    }
    std::vector<std::uint8_t> payload(static_cast<std::size_t>(length));
    if (length > 0 && !ReadExact(sock, payload.data(), payload.size())) {
      return false;
    }
    if (masked) {
      for (std::size_t i = 0; i < payload.size(); ++i) {
        payload[i] = static_cast<std::uint8_t>(payload[i] ^ mask[i % 4]);
      }
    }
    if (opcode == 0x8) {
      return false;
    }
    if (opcode == 0x9) {
      const std::vector<std::uint8_t> pong = WsFrameOpcode(0xA, payload.data(), payload.size());
      std::lock_guard<std::mutex> lock(*send_mu);
      if (!SendAll(sock, pong.data(), pong.size())) {
        return false;
      }
    }
  }
}

bool ReadHeaders(Socket sock, std::string& headers) {
  char buf[1024];
  while (headers.find("\r\n\r\n") == std::string::npos) {
    if (headers.size() > 8192) {
      return false;
    }
#if defined(_WIN32)
    const int n = ::recv(sock, buf, sizeof(buf), 0);
#else
    const ssize_t n = ::recv(sock, buf, sizeof(buf), 0);
#endif
    if (n <= 0) {
      return false;
    }
    headers.append(buf, buf + n);
  }
  return true;
}

std::string QueryToken(const std::string& headers) {
  const auto line_end = headers.find("\r\n");
  const std::string line = headers.substr(0, line_end);
  const auto q = line.find("token=");
  if (q == std::string::npos) {
    return "";
  }
  std::size_t begin = q + 6;
  std::size_t end = begin;
  while (end < line.size() && line[end] != ' ' && line[end] != '&') {
    ++end;
  }
  return line.substr(begin, end - begin);
}

std::string WsKey(const std::string& headers) {
  const auto pos = headers.find("Sec-WebSocket-Key:");
  if (pos == std::string::npos) {
    return "";
  }
  std::size_t begin = pos + 18;
  while (begin < headers.size() && (headers[begin] == ' ' || headers[begin] == '\t')) {
    ++begin;
  }
  std::size_t end = begin;
  while (end < headers.size() && headers[end] != '\r' && headers[end] != '\n') {
    ++end;
  }
  while (end > begin && headers[end - 1] == ' ') {
    --end;
  }
  return headers.substr(begin, end - begin);
}

bool Handshake(Socket sock, const std::string& token) {
  std::string headers;
  if (!ReadHeaders(sock, headers)) {
    return false;
  }
  if (QueryToken(headers) != token) {
    return false;
  }
  const std::string key = WsKey(headers);
  if (key.empty()) {
    return false;
  }
  const std::string material = key + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11";
  std::uint8_t digest[20];
  if (!Sha1(reinterpret_cast<const std::uint8_t*>(material.data()), material.size(), digest)) {
    return false;
  }
  const std::string accept = Base64(digest, 20);
  const std::string response =
      "HTTP/1.1 101 Switching Protocols\r\n"
      "Upgrade: websocket\r\n"
      "Connection: Upgrade\r\n"
      "Sec-WebSocket-Accept: " +
      accept + "\r\n\r\n";
  return SendAll(sock, reinterpret_cast<const std::uint8_t*>(response.data()), response.size());
}

}  // namespace

struct FrameServer::Impl {
  Socket listen = kInvalid;
  Socket client = kInvalid;
  int port = 0;
  std::string token;
  std::atomic<bool> stop{false};
  std::uint64_t generation = 0;
  std::thread accept_thread;
  std::mutex mu;
  std::mutex send_mu;

  void AcceptLoop() {
    while (!stop.load()) {
      Socket incoming = ::accept(listen, nullptr, nullptr);
      if (incoming == kInvalid) {
        break;
      }
#if defined(_WIN32)
      DWORD timeout_ms = 2000;
      setsockopt(incoming, SOL_SOCKET, SO_SNDTIMEO, reinterpret_cast<const char*>(&timeout_ms),
                 sizeof(timeout_ms));
      BOOL nodelay = TRUE;
      setsockopt(incoming, IPPROTO_TCP, TCP_NODELAY, reinterpret_cast<const char*>(&nodelay),
                 sizeof(nodelay));
#else
      timeval timeout{};
      timeout.tv_sec = 2;
      setsockopt(incoming, SOL_SOCKET, SO_SNDTIMEO, &timeout, sizeof(timeout));
      int nodelay = 1;
      setsockopt(incoming, IPPROTO_TCP, TCP_NODELAY, &nodelay, sizeof(nodelay));
#endif
      if (!Handshake(incoming, token)) {
        CloseSock(incoming);
        continue;
      }
      {
        std::lock_guard<std::mutex> lock(mu);
        if (client != kInvalid) {
          CloseSock(client);
        }
        client = incoming;
        ++generation;
      }
      DrainClient(incoming, &send_mu);
      std::lock_guard<std::mutex> lock(mu);
      if (client == incoming) {
        CloseSock(client);
        client = kInvalid;
      }
    }
  }
};

FrameServer::~FrameServer() { Close(); }

bool FrameServer::Listen(const std::string& token, std::string& error) {
  Close();
  if (!Startup()) {
    error = "rdp: winsock startup failed";
    return false;
  }
  impl_ = new Impl();
  impl_->token = token;
  impl_->listen = ::socket(AF_INET, SOCK_STREAM, IPPROTO_TCP);
  if (impl_->listen == kInvalid) {
    error = "rdp: frame socket failed";
    Close();
    return false;
  }
  sockaddr_in addr{};
  addr.sin_family = AF_INET;
  addr.sin_addr.s_addr = htonl(INADDR_LOOPBACK);
  addr.sin_port = 0;
  if (::bind(impl_->listen, reinterpret_cast<sockaddr*>(&addr), sizeof(addr)) != 0 ||
      ::listen(impl_->listen, 2) != 0) {
    error = "rdp: frame listen failed (" + std::to_string(LastSockErr()) + ")";
    Close();
    return false;
  }
  sockaddr_in bound{};
#if defined(_WIN32)
  int len = sizeof(bound);
#else
  socklen_t len = sizeof(bound);
#endif
  if (::getsockname(impl_->listen, reinterpret_cast<sockaddr*>(&bound), &len) != 0) {
    error = "rdp: frame port failed";
    Close();
    return false;
  }
  impl_->port = ntohs(bound.sin_port);
  impl_->accept_thread = std::thread([this] { impl_->AcceptLoop(); });
  return true;
}

int FrameServer::Port() const { return impl_ == nullptr ? 0 : impl_->port; }

std::uint64_t FrameServer::Generation() const {
  if (impl_ == nullptr) {
    return 0;
  }
  std::lock_guard<std::mutex> lock(impl_->mu);
  return impl_->generation;
}

bool FrameServer::HasClient() const {
  if (impl_ == nullptr) {
    return false;
  }
  std::lock_guard<std::mutex> lock(impl_->mu);
  return impl_->client != kInvalid;
}

bool FrameServer::Send(const std::uint8_t* data, std::size_t size) {
  if (impl_ == nullptr) {
    return false;
  }
  const std::vector<std::uint8_t> frame = WsFrame(data, size);
  Socket sock = kInvalid;
  {
    std::lock_guard<std::mutex> lock(impl_->mu);
    sock = impl_->client;
  }
  if (sock == kInvalid) {
    return false;
  }
  std::lock_guard<std::mutex> sending(impl_->send_mu);
  if (!SendAll(sock, frame.data(), frame.size())) {
    std::lock_guard<std::mutex> lock(impl_->mu);
    if (impl_->client == sock) {
      CloseSock(impl_->client);
      impl_->client = kInvalid;
    }
    return false;
  }
  return true;
}

void FrameServer::Close() {
  if (impl_ == nullptr) {
    return;
  }
  impl_->stop.store(true);
  CloseSock(impl_->listen);
  impl_->listen = kInvalid;
  {
    std::lock_guard<std::mutex> lock(impl_->mu);
    CloseSock(impl_->client);
    impl_->client = kInvalid;
  }
  if (impl_->accept_thread.joinable()) {
    impl_->accept_thread.join();
  }
  delete impl_;
  impl_ = nullptr;
}

}  // namespace ensisops::rdp
