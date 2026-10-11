cmake_minimum_required(VERSION 3.21)
project(ensis_logutil VERSION 0.1.0 LANGUAGES CXX)

set(CMAKE_CXX_STANDARD 20)
set(CMAKE_CXX_STANDARD_REQUIRED ON)

# 父工程可在 add_subdirectory 前：
#   set(ENSIS_LOGUTIL_BUILD_TESTS OFF CACHE BOOL "" FORCE)
option(ENSIS_LOGUTIL_BUILD_TESTS "Build ensis_logutil unit tests" OFF)

add_library(ensis_logutil STATIC
  src/logutil.cpp
  src/crash.cpp
)

add_library(ensisops::logutil ALIAS ensis_logutil)

target_include_directories(ensis_logutil
  PUBLIC
    $<BUILD_INTERFACE:${CMAKE_CURRENT_SOURCE_DIR}/include>
    $<INSTALL_INTERFACE:include>
)

if(WIN32)
  target_compile_definitions(ensis_logutil PRIVATE _CRT_SECURE_NO_WARNINGS)
  target_link_libraries(ensis_logutil PUBLIC dbghelp)
  if(MSVC)
    target_compile_options(ensis_logutil PRIVATE /utf-8)
  endif()
else()
  target_link_libraries(ensis_logutil PUBLIC pthread)
endif()

if(ENSIS_LOGUTIL_BUILD_TESTS)
  enable_testing()
endif()
