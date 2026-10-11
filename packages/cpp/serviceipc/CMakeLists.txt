cmake_minimum_required(VERSION 3.21)
project(ensis_serviceipc VERSION 0.1.0 LANGUAGES CXX)

set(CMAKE_CXX_STANDARD 20)
set(CMAKE_CXX_STANDARD_REQUIRED ON)

# 父工程可在 add_subdirectory 前：
#   set(ENSIS_SERVICEIPC_BUILD_TESTS OFF CACHE BOOL "" FORCE)
option(ENSIS_SERVICEIPC_BUILD_TESTS "Build ensis_serviceipc unit tests" ON)

add_library(ensis_serviceipc STATIC
  src/frame.cpp
  src/server.cpp
  src/message.cpp
  src/event.cpp
)

add_library(ensisops::serviceipc ALIAS ensis_serviceipc)

target_include_directories(ensis_serviceipc
  PUBLIC
    $<BUILD_INTERFACE:${CMAKE_CURRENT_SOURCE_DIR}/include>
    $<INSTALL_INTERFACE:include>
)

if(WIN32)
  target_compile_definitions(ensis_serviceipc PUBLIC ENSIS_SERVICEIPC_WIN=1)
  target_compile_definitions(ensis_serviceipc PRIVATE _CRT_SECURE_NO_WARNINGS)
  if(MSVC)
    target_compile_options(ensis_serviceipc PRIVATE /utf-8)
  endif()
else()
  target_compile_definitions(ensis_serviceipc PUBLIC ENSIS_SERVICEIPC_UNIX=1)
  target_link_libraries(ensis_serviceipc PUBLIC pthread)
endif()

if(ENSIS_SERVICEIPC_BUILD_TESTS)
  enable_testing()
  add_executable(ensis-serviceipc-frame-test tests/frame_test.cpp)
  target_link_libraries(ensis-serviceipc-frame-test PRIVATE ensis_serviceipc)
  if(MSVC)
    target_compile_options(ensis-serviceipc-frame-test PRIVATE /utf-8)
  endif()
  add_test(NAME ensis-serviceipc-frame-test COMMAND ensis-serviceipc-frame-test)

  add_executable(ensis-serviceipc-message-test tests/message_test.cpp)
  target_link_libraries(ensis-serviceipc-message-test PRIVATE ensis_serviceipc)
  file(TO_CMAKE_PATH
       "${CMAKE_CURRENT_SOURCE_DIR}/../../go/serviceipc/envelope/golden"
       _ensis_ipc_golden_dir)
  target_compile_definitions(ensis-serviceipc-message-test PRIVATE
                             "ENSIS_IPC_GOLDEN_DIR=\"${_ensis_ipc_golden_dir}\"")
  if(MSVC)
    target_compile_options(ensis-serviceipc-message-test PRIVATE /utf-8)
  endif()
  add_test(NAME ensis-serviceipc-message-test COMMAND ensis-serviceipc-message-test)
endif()
