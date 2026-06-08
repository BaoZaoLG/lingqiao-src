# -- MinHook library (built as part of CefHook) -------------------------
set(MINHOOK_DIR ${CMAKE_CURRENT_SOURCE_DIR}/vendor/minhook)

# Check if MinHook source exists
if(EXISTS "${MINHOOK_DIR}/src/hook.c")
    set(MINHOOK_SOURCES
        ${MINHOOK_DIR}/src/buffer.c
        ${MINHOOK_DIR}/src/hook.c
        ${MINHOOK_DIR}/src/trampoline.c
    )

    # HDE (Hacker Disassembler Engine) — platform-specific
    if(CMAKE_SIZEOF_VOID_P EQUAL 8)
        # x64
        if(EXISTS "${MINHOOK_DIR}/src/hde/hde64.c")
            list(APPEND MINHOOK_SOURCES ${MINHOOK_DIR}/src/hde/hde64.c)
        endif()
    else()
        # x86
        if(EXISTS "${MINHOOK_DIR}/src/hde/hde32.c")
            list(APPEND MINHOOK_SOURCES ${MINHOOK_DIR}/src/hde/hde32.c)
        endif()
    endif()

    set(HAS_MINHOOK TRUE)
    message(STATUS "MinHook found at ${MINHOOK_DIR}")
else()
    set(HAS_MINHOOK FALSE)
    message(WARNING "MinHook not found! Run: powershell -File scripts/download_minhook.ps1")
endif()

# -- Hook DLL ----------------------------------------------------------
add_library(CefHook SHARED
    dllmain.cpp
    thread.cpp
    pattern_scan.cpp
    v8_signatures.cpp
    v8_hooks.cpp
    process_identity.cpp
    hook_engine.c
    pch.cpp
    pch.h
    hook_engine.h
    pattern_scan.h
    v8_signatures.h
    v8_hooks.h
    process_identity.h
    script_loader.h
    hook_safety.h
    framework.h
    ${MINHOOK_SOURCES}
)

set_target_properties(CefHook PROPERTIES
    LINKER_LANGUAGE CXX
    OUTPUT_NAME "CefHook"
    PREFIX ""
)

target_include_directories(CefHook PRIVATE
    ${CMAKE_CURRENT_SOURCE_DIR}
    ${MINHOOK_DIR}/include
    ${MINHOOK_DIR}/src       # For HDE headers
)

target_link_libraries(CefHook PRIVATE shell32 comdlg32 advapi32 bcrypt winhttp crypt32)
target_compile_options(CefHook PRIVATE /utf-8 /GS /guard:cf /O2 /GL)
target_link_options(CefHook PRIVATE /DYNAMICBASE /NXCOMPAT /guard:cf /LTCG /OPT:REF /OPT:ICF /PDBALTPATH:%_PDB%)

if(NOT CMAKE_PREFIX_PATH)
    set(CMAKE_PREFIX_PATH "C:/Qt/5.15.2/msvc2019")
endif()
find_package(Qt5 REQUIRED COMPONENTS Widgets Network WinExtras Concurrent)

# -- Small native test utilities --------------------------------------
add_executable(PatternScanTest
    pattern_scan_test.cpp
    pattern_scan.cpp
)
target_include_directories(PatternScanTest PRIVATE ${CMAKE_CURRENT_SOURCE_DIR})
target_compile_options(PatternScanTest PRIVATE /utf-8)

add_executable(ManualDllInjector
    manual_dll_injector.cpp
    manual_injector_core.cpp
    manual_injector_core.h
)
target_compile_options(ManualDllInjector PRIVATE /utf-8)
target_link_options(ManualDllInjector PRIVATE /DYNAMICBASE /NXCOMPAT)

add_executable(ManualInjectorCoreTest
    manual_injector_core_test.cpp
    manual_injector_core.cpp
    manual_injector_core.h
)
target_include_directories(ManualInjectorCoreTest PRIVATE ${CMAKE_CURRENT_SOURCE_DIR})
target_compile_options(ManualInjectorCoreTest PRIVATE /utf-8)

add_executable(ProcessIdentityTest
    process_identity_test.cpp
    process_identity.cpp
    process_identity.h
)
target_include_directories(ProcessIdentityTest PRIVATE ${CMAKE_CURRENT_SOURCE_DIR})
target_compile_options(ProcessIdentityTest PRIVATE /utf-8)

add_executable(SecurityRegressionTest
    security_regression_test.cpp
)
target_include_directories(SecurityRegressionTest PRIVATE ${CMAKE_CURRENT_SOURCE_DIR})
target_compile_options(SecurityRegressionTest PRIVATE /utf-8)

add_executable(ManualDllInjectorGui WIN32
    manual_dll_injector_gui.cpp
    manual_injector_core.cpp
    manual_injector_core.h
)
target_include_directories(ManualDllInjectorGui PRIVATE ${CMAKE_CURRENT_SOURCE_DIR})
target_link_libraries(ManualDllInjectorGui PRIVATE Qt5::Widgets)
target_compile_options(ManualDllInjectorGui PRIVATE /utf-8)
target_link_options(ManualDllInjectorGui PRIVATE /DYNAMICBASE /NXCOMPAT)
add_custom_command(TARGET ManualDllInjectorGui POST_BUILD
    COMMAND "${CMAKE_PREFIX_PATH}/bin/windeployqt.exe" --release --force
        --no-translations
        "$<TARGET_FILE:ManualDllInjectorGui>"
    COMMENT "Deploying Qt runtime dependencies for ManualDllInjectorGui..."
)

# -- Injector EXE (Qt) -------------------------------------------------
set(CMAKE_AUTOMOC ON)
set(CMAKE_AUTORCC ON)
set(CMAKE_AUTOUIC ON)

# Generate version resource from CMake APP_VERSION
configure_file(${CMAKE_CURRENT_SOURCE_DIR}/version.rc.in
               ${CMAKE_CURRENT_BINARY_DIR}/version.rc @ONLY)

set(INJECTOR_HEADERS
    antidebug.h
    strcrypt.h
    config.h
    crypto.h
    http_client.h
    machine_fp.h
    dll_extractor.h
    workers.h
    ui/theme.h
    ui/title_bar.h
    ui/inline_status.h
    ui/animated_button.h
    ui/main_window.h
    ui/impl_session.h
    ui/impl_builder.h
    ui/impl_update.h
)

add_executable(Injector WIN32
    injector_qt.cpp
    ${INJECTOR_HEADERS}
    ${CMAKE_CURRENT_BINARY_DIR}/version.rc
)
set_target_properties(Injector PROPERTIES OUTPUT_NAME "CX_LingQiao")
target_include_directories(Injector PRIVATE ${CMAKE_CURRENT_BINARY_DIR} ${CMAKE_CURRENT_SOURCE_DIR})
target_link_libraries(Injector PRIVATE
    Qt5::Widgets
    Qt5::Network
    Qt5::WinExtras
    Qt5::Concurrent
    comctl32
    shell32
    advapi32
    iphlpapi
    psapi
    crypt32
    ws2_32
    version
)
target_compile_options(Injector PRIVATE /utf-8 /bigobj /GS /guard:cf /O2 /GL)
target_link_options(Injector PRIVATE /DYNAMICBASE /NXCOMPAT /guard:cf /LTCG /OPT:REF /OPT:ICF /PDBALTPATH:%_PDB%)
target_compile_definitions(Injector PRIVATE
    QT_DISABLE_DEPRECATED_BEFORE=0x050F00
    _CRT_SECURE_NO_WARNINGS
    APP_VERSION="${APP_VERSION}"
    UPDATE_MANIFEST_PUBLIC_KEY_HEX="${UPDATE_MANIFEST_PUBLIC_KEY_HEX}"
)

# Auto-deploy Qt DLLs after building Injector.exe
add_custom_command(TARGET Injector POST_BUILD
    COMMAND "${CMAKE_PREFIX_PATH}/bin/windeployqt.exe" --release --force
        --no-translations
        "$<TARGET_FILE:Injector>"
    COMMENT "Deploying Qt runtime dependencies..."
)
