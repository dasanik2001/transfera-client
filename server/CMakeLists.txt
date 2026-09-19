cmake_minimum_required(VERSION 3.20)
project(server LANGUAGES CXX)

set(CMAKE_CXX_STANDARD 20)
set(CMAKE_CXX_STANDARD_REQUIRED ON)

add_executable(server
  src/main_runner.cpp
  src/app.cpp
  src/logging.cpp
  src/services/fileSharer.cpp
  src/services/roomManager.cpp
  src/services/fileController.cpp
  src/utils/uploadUtils.cpp
)

target_include_directories(server PRIVATE
  ${CMAKE_CURRENT_SOURCE_DIR}/include
)
target_compile_features(server PRIVATE cxx_std_20)