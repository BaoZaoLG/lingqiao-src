# -- Common include paths for all targets --------------------------------
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

# -- MinHook library (built as part of CefHook) -------------------------
set(MINHOOK_DIR ${CMAKE_CURRENT_SOURCE_DIR}/vendor/minhook)

if(EXISTS "${MINHOOK_DIR}/src/hook.c")
    set(MINHOOK_SOURCES
        ${MINHOOK_DIR}/src/buffer.c
        ${MINHOOK_DIR}/src/hook.c
        ${MINHOOK_DIR}/src/trampoline.c
    )
    if(CMAKE_SIZEOF_VOID_P EQUAL 8)
        if(EXISTS "${MINHOOK_DIR}/src/hde/hde64.c")
            list(APPEND MINHOOK_SOURCES ${MINHOOK_DIR}/src/hde/hde64.c)
        endif()
    else()
        if(EXISTS "${MINHOOK_DIR}/src/hde/hde32.c")
            list(APPEND MINHOOK_SOURCES ${MINHOOK_DIR}/src/hde/hde32.c)
        endif()
    endif()
    set(HAS_MINHOOK TRUE)
    message(STATUS "MinHook found at ${MINHOOK_DIR}")
else()
    set(HAS_MINHOOK FALSE)
    message(WARNING "MinHook not found! Please check vendor/minhook directory.")
endif()

# ============================================================
# CefHook DLL (CEF injection hook)
# ============================================================
add_library(CefHook SHARED
    hook/dllmain.cpp
    hook/thread.cpp
    hook/pattern_scan.cpp
    hook/v8_signatures.cpp
    hook/v8_hooks.cpp
    hook/process_identity.cpp
    hook/hook_engine.c
    hook/script_loader.h
    hook/hook_safety.h
    core/pch.cpp
    core/pch.h
    core/framework.h
    ${MINHOOK_SOURCES}
)

set_target_properties(CefHook PROPERTIES LINKER_LANGUAGE CXX OUTPUT_NAME "CefHook" PREFIX "")
target_include_directories(CefHook PRIVATE ${COMMON_INCLUDES} ${MINHOOK_DIR}/include ${MINHOOK_DIR}/src)
target_link_libraries(CefHook PRIVATE shell32 comdlg32 advapi32 bcrypt winhttp crypt32)
target_compile_options(CefHook PRIVATE /utf-8 /GS /guard:cf /O2 /GL)
target_link_options(CefHook PRIVATE /DYNAMICBASE /NXCOMPAT /guard:cf /LTCG /OPT:REF /OPT:ICF /PDBALTPATH:%_PDB%)

# ============================================================
# Qt dependency (shared by Injector + ManualDllInjectorGui)
# ============================================================
if(NOT CMAKE_PREFIX_PATH)
    set(CMAKE_PREFIX_PATH "C:/Qt/5.15.2/msvc2019")
endif()
find_package(Qt5 REQUIRED COMPONENTS Widgets Network WinExtras Concurrent)



# ============================================================
# Standalone injector tools
# ============================================================
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

# ============================================================
# Injector EXE (Qt GUI — main product)
# ============================================================
set(CMAKE_AUTOMOC ON)
set(CMAKE_AUTORCC ON)
set(CMAKE_AUTOUIC ON)

configure_file(${CMAKE_CURRENT_SOURCE_DIR}/version.rc.in ${CMAKE_CURRENT_BINARY_DIR}/version.rc @ONLY)

set(INJECTOR_HEADERS
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
)

add_executable(Injector WIN32 injector_qt.cpp ${INJECTOR_HEADERS} ${CMAKE_CURRENT_BINARY_DIR}/version.rc)
set_target_properties(Injector PROPERTIES OUTPUT_NAME "CX_LingQiao")
target_include_directories(Injector PRIVATE ${COMMON_INCLUDES} ${CMAKE_CURRENT_BINARY_DIR})
target_link_libraries(Injector PRIVATE Qt5::Widgets Qt5::Network Qt5::WinExtras Qt5::Concurrent
    comctl32 shell32 advapi32 iphlpapi psapi crypt32 ws2_32 version)
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
