#include "handler/dispatcher.hpp"
#include "util/paths.hpp"

#include <ensisops/logutil/logutil.hpp>
#include <ensisops/serviceipc/server.hpp>

#include <chrono>
#include <csignal>
#include <memory>

namespace {
std::unique_ptr<ensisops::serviceipc::Server> g_server;

void OnSignal(int) {
  if (g_server) {
    g_server->Stop();
  }
}
}  // namespace

int main(int argc, char** argv) {
  std::string addr = ensisops::oracle::util::DefaultIpcAddress();
  for (int i = 1; i < argc; ++i) {
    const std::string a = argv[i];
    if ((a == "--addr" || a == "-a") && i + 1 < argc) {
      addr = argv[++i];
    }
  }

  ensisops::logutil::Init("oracle-service");
  ensisops::logutil::Info("oracle-service starting",
                       {{"ipc", addr}, {"oracleClientLibDir", ensisops::oracle::util::OracleClientLibDir()}});

  ensisops::oracle::handler::Dispatcher dispatcher;
  g_server = std::make_unique<ensisops::serviceipc::Server>(
      addr,
      [&dispatcher](const std::string& req) {
        const auto t0 = std::chrono::steady_clock::now();
        const std::string resp = dispatcher.HandleFrame(req);
        const auto ms = std::chrono::duration_cast<std::chrono::milliseconds>(
                            std::chrono::steady_clock::now() - t0)
                            .count();
        ensisops::logutil::ObserveIPC("oracle-service", req, resp, ms);
        return resp;
      },
      "oracle-service");

  std::signal(SIGINT, OnSignal);
  std::signal(SIGTERM, OnSignal);

  ensisops::logutil::Info("serving", {{"addr", addr}});
  return g_server->Serve();
}
