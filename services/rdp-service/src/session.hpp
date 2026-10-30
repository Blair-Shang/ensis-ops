#pragma once

#include "json_mini.hpp"

#include <string>
#include <vector>

namespace ensisops::rdp {

struct SessionInfo {
  std::string session_id;
  int width = 0;
  int height = 0;
  int frame_port = 0;
  std::string frame_token;
  /** 映射到远端的本机目录。远端「此电脑」里的磁盘名见 drive_name。 */
  std::string local_path;
  std::string drive_name;
};

/** 进程内 RDP 会话表。FreeRDP 调用都留在会话线程。 */
class Manager {
 public:
  Manager();
  ~Manager();

  Manager(const Manager&) = delete;
  Manager& operator=(const Manager&) = delete;

  bool Open(const Json& params, SessionInfo& info, std::string& error);
  bool Test(const Json& params, std::string& message, std::string& error);
  bool Info(const std::string& session_id, SessionInfo& info, std::string& error);
  bool Close(const std::string& session_id, std::string& error);
  bool Mouse(const std::string& session_id, const std::string& action, int x, int y, int button,
             int delta, std::string& error);
  bool Key(const std::string& session_id, const std::string& code, bool down, bool repeat,
           std::string& error);
  /** 列出映射目录里的文件。json 为对象，含 localPath、driveName、files。 */
  bool ListFiles(const std::string& session_id, std::string& json, std::string& error);
  /** 把本机文件复制进映射目录，远端即可在 Ensis 磁盘里看到。 */
  bool ImportFiles(const std::string& session_id, const std::vector<std::string>& paths,
                   std::string& json, std::string& error);
  /** 用系统文件管理器打开映射目录。 */
  bool OpenShare(const std::string& session_id, std::string& error);

 private:
  struct Impl;
  Impl* impl_;
};

}  // namespace ensisops::rdp
