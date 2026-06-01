cmake_minimum_required (VERSION 3.15)
project(CefHook)
set(CMAKE_CXX_STANDARD 17)

# ============================================================
# Version — embedded in PE resource, authoritative at runtime
# ============================================================
if(NOT APP_VERSION)
    set(APP_VERSION "2.1.13")
endif()
string(REPLACE "." "," APP_VERSION_COMMA "${APP_VERSION}")
set(CMAKE_CXX_STANDARD_REQUIRED ON)

# Static link CRT to avoid VC++ redist dependency on target machine
if(MSVC)
    cmake_policy(SET CMP0091 NEW)
    set(CMAKE_MSVC_RUNTIME_LIBRARY "MultiThreaded$<$<CONFIG:Debug>:Debug>")
endif()

add_definitions(-DUNICODE -D_UNICODE)
include_directories(${CMAKE_SOURCE_DIR}/cef)
include_directories(${CMAKE_SOURCE_DIR}/chromium)

# ============================================================
# Security hardening flags (applied to all targets)
# ============================================================
if(MSVC)
    # Stack buffer overflow protection
    add_compile_options(/GS)
    # Control Flow Guard
    add_compile_options(/guard:cf)
    # Address Space Layout Randomization
    add_link_options(/DYNAMICBASE)
    # Data Execution Prevention
    add_link_options(/NXCOMPAT)
    # Remove unused data
    add_link_options(/OPT:REF /OPT:ICF)
endif()

add_subdirectory(src)
