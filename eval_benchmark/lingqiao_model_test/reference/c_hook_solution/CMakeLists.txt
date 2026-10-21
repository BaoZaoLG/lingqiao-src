cmake_minimum_required(VERSION 3.16)
project(lingqiao_hook_test LANGUAGES C CXX)

set(CMAKE_C_STANDARD 11)
set(CMAKE_CXX_STANDARD 17)
set(CMAKE_CXX_STANDARD_REQUIRED ON)

# 编译选项
if(MSVC)
    add_compile_options(/W4 /WX)
else()
    add_compile_options(-Wall -Wextra -Werror -Wpedantic)
    # Sanitizer 支持
    option(ENABLE_ASAN "Enable AddressSanitizer" OFF)
    option(ENABLE_TSAN "Enable ThreadSanitizer" OFF)
    if(ENABLE_ASAN)
        add_compile_options(-fsanitize=address -fno-omit-frame-pointer)
        add_link_options(-fsanitize=address)
    endif()
    if(ENABLE_TSAN)
        add_compile_options(-fsanitize=thread)
        add_link_options(-fsanitize=thread)
    endif()
endif()

# Hook 引擎库
add_library(hook_engine STATIC
    hook_engine.c
    pattern_scan.cpp
    v8_intercept.cpp
)
target_include_directories(hook_engine PUBLIC ${CMAKE_CURRENT_SOURCE_DIR})

# 测试可执行文件
if(EXISTS "${CMAKE_SOURCE_DIR}/../tests/test_pattern_scan.cpp")
    add_executable(test_pattern_scan
        ../tests/test_pattern_scan.cpp
    )
    target_link_libraries(test_pattern_scan PRIVATE hook_engine)
    if(NOT MSVC)
        target_link_libraries(test_pattern_scan PRIVATE pthread)
    endif()
endif()

if(EXISTS "${CMAKE_SOURCE_DIR}/../tests/test_hook_engine.cpp")
    add_executable(test_hook_engine
        ../tests/test_hook_engine.cpp
    )
    target_link_libraries(test_hook_engine PRIVATE hook_engine)
    if(NOT MSVC)
        target_link_libraries(test_hook_engine PRIVATE pthread)
    endif()
endif()

if(EXISTS "${CMAKE_SOURCE_DIR}/../tests/test_bridge_protocol.cpp")
    add_executable(test_bridge_protocol
        ../tests/test_bridge_protocol.cpp
    )
    target_link_libraries(test_bridge_protocol PRIVATE hook_engine)
    if(NOT MSVC)
        target_link_libraries(test_bridge_protocol PRIVATE pthread)
    endif()
endif()
