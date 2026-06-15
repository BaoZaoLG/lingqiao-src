cmake_minimum_required (VERSION 3.15)
if(POLICY CMP0091)
    cmake_policy(SET CMP0091 NEW)
endif()
project(CefHook)
set(CMAKE_CXX_STANDARD 17)

# ============================================================
# Version — embedded in PE resource, authoritative at runtime
# ============================================================
if(NOT APP_VERSION)
    set(APP_VERSION "2.3.0")
endif()
set(UPDATE_MANIFEST_PUBLIC_KEY_HEX "" CACHE STRING "Pinned release manifest public key hex")
string(REPLACE "." "," APP_VERSION_COMMA "${APP_VERSION}")
set(CMAKE_CXX_STANDARD_REQUIRED ON)

# Static link CRT to avoid VC++ redist dependency on target machine
if(MSVC)
    set(CMAKE_MSVC_RUNTIME_LIBRARY "MultiThreaded$<$<CONFIG:Debug>:Debug>")
endif()

add_definitions(-DUNICODE -D_UNICODE)
include_directories(${CMAKE_SOURCE_DIR}/cef)
include_directories(${CMAKE_SOURCE_DIR}/chromium)

# ============================================================
# Security hardening flags (applied to all targets)
# ============================================================
if(MSVC)
    add_compile_options(/GS)
    add_compile_options(/guard:cf)
    add_link_options(/DYNAMICBASE)
    add_link_options(/NXCOMPAT)
    add_link_options(/OPT:REF /OPT:ICF)
endif()

add_subdirectory(src)
