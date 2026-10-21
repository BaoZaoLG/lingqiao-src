cmake_minimum_required(VERSION 3.15)

if(POLICY CMP0091)
    cmake_policy(SET CMP0091 NEW)
endif()

project(CefHook LANGUAGES C CXX)

set(CMAKE_CXX_STANDARD 17)
set(CMAKE_CXX_STANDARD_REQUIRED ON)

if(NOT APP_VERSION)
    set(APP_VERSION "2.4.0")
endif()
set(UPDATE_MANIFEST_PUBLIC_KEY_HEX "" CACHE STRING "Pinned release manifest public key hex")
string(REPLACE "." "," APP_VERSION_COMMA "${APP_VERSION}")

if(MSVC)
    set(CMAKE_MSVC_RUNTIME_LIBRARY "MultiThreaded$<$<CONFIG:Debug>:Debug>")
    add_compile_options(/GS /guard:cf)
    add_link_options(/DYNAMICBASE /NXCOMPAT /OPT:REF /OPT:ICF)
endif()

add_definitions(-DUNICODE -D_UNICODE)
include_directories(${CMAKE_SOURCE_DIR}/../cef)
include_directories(${CMAKE_SOURCE_DIR}/../chromium)

include(CTest)
add_subdirectory(src)
