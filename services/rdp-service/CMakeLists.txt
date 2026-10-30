cmake_minimum_required(VERSION 3.21)
project(ensis-rdp-service VERSION 0.1.0 LANGUAGES C CXX)

set(CMAKE_CXX_STANDARD 20)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
set(CMAKE_C_STANDARD 11)

# 公共 IPC。单测由本服务不重复跑。
set(ENSIS_SERVICEIPC_DIR "${CMAKE_CURRENT_SOURCE_DIR}/../../packages/cpp/serviceipc")
if(NOT EXISTS "${ENSIS_SERVICEIPC_DIR}/CMakeLists.txt")
  message(FATAL_ERROR "packages/cpp/serviceipc not found at ${ENSIS_SERVICEIPC_DIR}")
endif()
set(ENSIS_SERVICEIPC_BUILD_TESTS OFF CACHE BOOL "" FORCE)
add_subdirectory("${ENSIS_SERVICEIPC_DIR}" "${CMAKE_BINARY_DIR}/ensis_serviceipc")

set(ENSIS_LOGUTIL_DIR "${CMAKE_CURRENT_SOURCE_DIR}/../../packages/cpp/logutil")
if(NOT EXISTS "${ENSIS_LOGUTIL_DIR}/CMakeLists.txt")
  message(FATAL_ERROR "packages/cpp/logutil not found at ${ENSIS_LOGUTIL_DIR}")
endif()
set(ENSIS_LOGUTIL_BUILD_TESTS OFF CACHE BOOL "" FORCE)
add_subdirectory("${ENSIS_LOGUTIL_DIR}" "${CMAKE_BINARY_DIR}/ensis_logutil")

# 脚本会把 OpenSSL 3.5 装到 third_party/openssl。系统里已有时也可传 OPENSSL_ROOT_DIR。
if(NOT OPENSSL_ROOT_DIR)
  set(_ensis_openssl "${CMAKE_CURRENT_SOURCE_DIR}/third_party/openssl")
  if(EXISTS "${_ensis_openssl}/include/openssl/ssl.h")
    set(OPENSSL_ROOT_DIR "${_ensis_openssl}" CACHE PATH "" FORCE)
    set(OPENSSL_USE_STATIC_LIBS TRUE CACHE BOOL "" FORCE)
  endif()
endif()
find_package(OpenSSL REQUIRED)

# 只链协议库和静态通道加载器。不编官方窗口客户端，也不带 FFmpeg。
set(BUILD_SHARED_LIBS OFF CACHE BOOL "" FORCE)
set(BUILD_TESTING OFF CACHE BOOL "" FORCE)
set(WITH_SERVER OFF CACHE BOOL "" FORCE)
set(WITH_CLIENT OFF CACHE BOOL "" FORCE)
set(WITH_CLIENT_COMMON ON CACHE BOOL "" FORCE)
set(WITH_CLIENT_CHANNELS ON CACHE BOOL "" FORCE)
set(WITH_SAMPLE OFF CACHE BOOL "" FORCE)
set(WITH_PROXY OFF CACHE BOOL "" FORCE)
set(WITH_SHADOW OFF CACHE BOOL "" FORCE)
set(WITH_PLATFORM_SERVER OFF CACHE BOOL "" FORCE)
set(WITH_FFMPEG OFF CACHE BOOL "" FORCE)
set(WITH_SWSCALE OFF CACHE BOOL "" FORCE)
set(WITH_DSP_FFMPEG OFF CACHE BOOL "" FORCE)
set(WITH_OPENH264 OFF CACHE BOOL "" FORCE)
set(WITH_OPENCL OFF CACHE BOOL "" FORCE)
set(WITH_X11 OFF CACHE BOOL "" FORCE)
set(WITH_WAYLAND OFF CACHE BOOL "" FORCE)
set(WITH_SDL_LINK_SHARED OFF CACHE BOOL "" FORCE)
set(WITH_CLIENT_SDL OFF CACHE BOOL "" FORCE)
set(WITH_CLIENT_SDL2 OFF CACHE BOOL "" FORCE)
set(WITH_CLIENT_SDL3 OFF CACHE BOOL "" FORCE)
set(WITH_MANPAGES OFF CACHE BOOL "" FORCE)
set(WITH_CAIRO OFF CACHE BOOL "" FORCE)
set(WITH_CUPS OFF CACHE BOOL "" FORCE)
set(WITH_PCSC OFF CACHE BOOL "" FORCE)
set(WITH_PKCS11 OFF CACHE BOOL "" FORCE)
set(WITH_FUSE OFF CACHE BOOL "" FORCE)
set(WITH_ALSA OFF CACHE BOOL "" FORCE)
set(WITH_PULSE OFF CACHE BOOL "" FORCE)
set(WITH_OSS OFF CACHE BOOL "" FORCE)
set(WITH_LIBSYSTEMD OFF CACHE BOOL "" FORCE)
set(WITH_VAAPI OFF CACHE BOOL "" FORCE)
set(WITH_KRB5 OFF CACHE BOOL "" FORCE)
set(WITH_SMARTCARD_EMULATE OFF CACHE BOOL "" FORCE)
set(CHANNEL_URBDRC OFF CACHE BOOL "" FORCE)

include(FetchContent)
FetchContent_Declare(freerdp
  URL https://github.com/FreeRDP/FreeRDP/releases/download/3.32.2/freerdp-3.32.2.tar.gz
  DOWNLOAD_EXTRACT_TIMESTAMP TRUE
)
FetchContent_MakeAvailable(freerdp)

add_executable(ensis-rdp-service
  src/main.cpp
  src/json_mini.cpp
  src/dispatcher.cpp
  src/session.cpp
  src/frame_server.cpp
  src/scancode.cpp
  src/digest.cpp
)

target_include_directories(ensis-rdp-service PRIVATE
  src
  "${freerdp_SOURCE_DIR}/include"
  "${freerdp_BINARY_DIR}/include"
  "${freerdp_SOURCE_DIR}/winpr/include"
  "${freerdp_BINARY_DIR}/winpr/include"
  "${freerdp_BINARY_DIR}"
)

target_link_libraries(ensis-rdp-service PRIVATE
  freerdp
  freerdp-client
  winpr
  ensisops::serviceipc
  ensisops::logutil
  OpenSSL::Crypto
  OpenSSL::SSL
)

if(WIN32)
  target_link_libraries(ensis-rdp-service PRIVATE ws2_32 shell32 ole32 uuid)
  target_compile_definitions(ensis-rdp-service PRIVATE NOMINMAX WIN32_LEAN_AND_MEAN FREERDP_EXPORTS)
  if(MSVC)
    target_compile_options(ensis-rdp-service PRIVATE /utf-8)
  endif()
endif()

