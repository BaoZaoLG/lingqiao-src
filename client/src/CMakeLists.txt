set(CMAKE_AUTOMOC ON)
set(CMAKE_AUTORCC ON)
set(CMAKE_AUTOUIC ON)

set(COMMON_INCLUDES
    ${CMAKE_CURRENT_SOURCE_DIR}
    ${CMAKE_CURRENT_SOURCE_DIR}/core
    ${CMAKE_CURRENT_SOURCE_DIR}/security
    ${CMAKE_CURRENT_SOURCE_DIR}/network
    ${CMAKE_CURRENT_SOURCE_DIR}/hook
    ${CMAKE_CURRENT_SOURCE_DIR}/generated
    ${CMAKE_CURRENT_SOURCE_DIR}/ui
    ${CMAKE_CURRENT_SOURCE_DIR}/injector
)

set(MINHOOK_DIR ${CMAKE_CURRENT_SOURCE_DIR}/vendor/minhook)
if(NOT EXISTS "${MINHOOK_DIR}/src/hook.c")
    message(FATAL_ERROR "MinHook not found at ${MINHOOK_DIR}")
endif()

set(MINHOOK_SOURCES
    ${MINHOOK_DIR}/src/buffer.c
    ${MINHOOK_DIR}/src/hook.c
    ${MINHOOK_DIR}/src/trampoline.c
)
if(CMAKE_SIZEOF_VOID_P EQUAL 8)
    list(APPEND MINHOOK_SOURCES ${MINHOOK_DIR}/src/hde/hde64.c)
else()
    list(APPEND MINHOOK_SOURCES ${MINHOOK_DIR}/src/hde/hde32.c)
endif()

set(BRIDGE_SOURCES
    hook/auto_inject.cpp
    hook/gpu_cmdline.cpp
    hook/keyboard_devtools.cpp
    hook/page_inject.cpp
    hook/bridge_console_fallback.cpp
    hook/bridge_dispatcher.cpp
    hook/bridge_json.cpp
    hook/bridge_runtime.cpp
    hook/bridge_security.cpp
    hook/bridge_v8.cpp
    hook/capabilities/diagnostics_capability.cpp
    hook/capabilities/ai_capability.cpp
    hook/capabilities/exam_capability.cpp
    hook/capabilities/native_input_capability.cpp
    hook/capabilities/page_capability.cpp
    hook/capabilities/runtime_capability.cpp
    hook/capabilities/script_capability.cpp
)

set(V8_SOURCES
    hook/v8_compile_sampler.cpp
    hook/v8_fast_neutralize.cpp
    hook/v8_hooks.cpp
    hook/v8_intercept.cpp
    hook/v8_raw_source_probe.cpp
    hook/v8_signatures.cpp
    hook/v8_source_neutralizer.cpp
    hook/v8_source_probe.cpp
)

add_library(CefHook SHARED
    hook/dllmain.cpp
    hook/thread.cpp
    hook/pattern_scan.cpp
    hook/process_identity.cpp
    hook/hook_engine.c
    ${BRIDGE_SOURCES}
    ${V8_SOURCES}
    core/pch.cpp
    ${MINHOOK_SOURCES}
)
set_target_properties(CefHook PROPERTIES LINKER_LANGUAGE CXX OUTPUT_NAME "CefHook" PREFIX "")
target_include_directories(CefHook PRIVATE ${COMMON_INCLUDES} ${MINHOOK_DIR}/include ${MINHOOK_DIR}/src)
target_link_libraries(CefHook PRIVATE shell32 comdlg32 advapi32 bcrypt winhttp crypt32)
target_compile_options(CefHook PRIVATE /utf-8 /GS /guard:cf /O2 /GL)
target_link_options(CefHook PRIVATE /DYNAMICBASE /NXCOMPAT /guard:cf /LTCG /OPT:REF /OPT:ICF /PDBALTPATH:%_PDB%)

if(NOT CMAKE_PREFIX_PATH)
    set(CMAKE_PREFIX_PATH "C:/Qt/5.15.2/msvc2019")
endif()
find_package(Qt5 REQUIRED COMPONENTS Core Widgets Network WinExtras Concurrent)

add_executable(ManualDllInjector injector/manual_dll_injector.cpp injector/manual_injector_core.cpp)
target_include_directories(ManualDllInjector PRIVATE ${COMMON_INCLUDES})
target_compile_options(ManualDllInjector PRIVATE /utf-8)
target_link_options(ManualDllInjector PRIVATE /DYNAMICBASE /NXCOMPAT)

add_executable(ManualDllInjectorGui WIN32 injector/manual_dll_injector_gui.cpp injector/manual_injector_core.cpp)
target_include_directories(ManualDllInjectorGui PRIVATE ${COMMON_INCLUDES})
target_link_libraries(ManualDllInjectorGui PRIVATE Qt5::Widgets)
target_compile_options(ManualDllInjectorGui PRIVATE /utf-8)
target_link_options(ManualDllInjectorGui PRIVATE /DYNAMICBASE /NXCOMPAT)
add_custom_command(TARGET ManualDllInjectorGui POST_BUILD
    COMMAND "${CMAKE_PREFIX_PATH}/bin/windeployqt.exe" --release --force --no-translations "$<TARGET_FILE:ManualDllInjectorGui>"
    COMMENT "Deploying Qt runtime for ManualDllInjectorGui..."
)

configure_file(${CMAKE_CURRENT_SOURCE_DIR}/version.rc.in ${CMAKE_CURRENT_BINARY_DIR}/version.rc @ONLY)

set(INJECTOR_HEADERS
    controllers/session_controller.h
    controllers/update_controller.h
    services/api_client.h
    security/antidebug.h
    security/machine_fp.h
    security/dll_extractor.h
    core/strcrypt.h
    core/config.h
    core/crypto.h
    network/http_client.h
    network/workers.h
    ui/main_window.h
    ui/theme.h
    ui/title_bar.h
    ui/inline_status.h
    ui/animated_button.h
    ui/update_url.h
    ui/impl_session.h
    ui/impl_builder.h
    ui/impl_update.h
    ui/impl_network.h
    ui/impl_chat.h
    ui/impl_actions.h
    ui/ui_resources.qrc
)

add_executable(Injector WIN32
    injector_qt.cpp
    controllers/session_controller.cpp
    controllers/update_controller.cpp
    services/api_client.cpp
    services/network_service.cpp
    core/config.cpp
    injector/injection_runner.cpp
    ${INJECTOR_HEADERS}
    ${CMAKE_CURRENT_BINARY_DIR}/version.rc
)
set_target_properties(Injector PROPERTIES OUTPUT_NAME "CX_LingQiao")
target_include_directories(Injector PRIVATE ${COMMON_INCLUDES} ${CMAKE_CURRENT_BINARY_DIR})
target_link_libraries(Injector PRIVATE Qt5::Widgets Qt5::Network Qt5::WinExtras Qt5::Concurrent
    comctl32 shell32 advapi32 iphlpapi psapi crypt32 bcrypt ws2_32 version)
target_compile_options(Injector PRIVATE /utf-8 /bigobj /GS /guard:cf /O2 /GL)
target_link_options(Injector PRIVATE /DYNAMICBASE /NXCOMPAT /guard:cf /LTCG /OPT:REF /OPT:ICF /PDBALTPATH:%_PDB%)
target_compile_definitions(Injector PRIVATE
    QT_DISABLE_DEPRECATED_BEFORE=0x050F00
    _CRT_SECURE_NO_WARNINGS
    APP_VERSION="${APP_VERSION}"
    UPDATE_MANIFEST_PUBLIC_KEY_HEX="${UPDATE_MANIFEST_PUBLIC_KEY_HEX}"
)
add_custom_command(TARGET Injector POST_BUILD
    COMMAND "${CMAKE_PREFIX_PATH}/bin/windeployqt.exe" --release --force --no-translations "$<TARGET_FILE:Injector>"
    COMMENT "Deploying Qt runtime for Injector..."
)

add_executable(ClientTests test_client.cpp services/api_client.cpp core/config.cpp)
target_include_directories(ClientTests PRIVATE ${COMMON_INCLUDES} ${CMAKE_CURRENT_BINARY_DIR})
target_link_libraries(ClientTests PRIVATE Qt5::Core Qt5::Network winhttp bcrypt)
target_compile_options(ClientTests PRIVATE /utf-8 /EHsc)
target_compile_definitions(ClientTests PRIVATE _CRT_SECURE_NO_WARNINGS)

function(add_native_regression_test target)
    add_executable(${target} ${ARGN})
    target_include_directories(${target} PRIVATE ${COMMON_INCLUDES})
    target_compile_options(${target} PRIVATE /utf-8 /EHsc)
    add_test(NAME ${target} COMMAND ${target})
endfunction()

add_native_regression_test(BridgeSecurityTests
    test_bridge_security.cpp
    hook/bridge_json.cpp
    hook/bridge_security.cpp
)
add_native_regression_test(V8CompileSamplerTests
    test_v8_compile_sampler.cpp
    hook/v8_compile_sampler.cpp
    hook/v8_raw_source_probe.cpp
)
add_native_regression_test(V8FastNeutralizeTests
    test_v8_fast_neutralize.cpp
    hook/v8_fast_neutralize.cpp
    hook/v8_raw_source_probe.cpp
)
add_native_regression_test(V8RawSourceProbeTests
    test_v8_raw_source_probe.cpp
    hook/v8_raw_source_probe.cpp
)
add_native_regression_test(V8SourceNeutralizerTests
    test_v8_source_neutralizer.cpp
    hook/v8_source_neutralizer.cpp
)
