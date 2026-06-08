@echo off
setlocal
cd /d "%~dp0"

if not defined BUILD_DIR set "BUILD_DIR=build"
if not defined QT_PREFIX_PATH set "QT_PREFIX_PATH=C:\Qt\5.15.2\msvc2019"
if not defined CMAKE_GENERATOR set "CMAKE_GENERATOR=Visual Studio 17 2022"
if not defined CMAKE_PLATFORM set "CMAKE_PLATFORM=Win32"

if not defined VCVARSALL (
    set "VSWHERE=%ProgramFiles(x86)%\Microsoft Visual Studio\Installer\vswhere.exe"
    if exist "%VSWHERE%" (
        for /f "usebackq delims=" %%I in (`"%VSWHERE%" -latest -products * -requires Microsoft.VisualStudio.Component.VC.Tools.x86.x64 -property installationPath`) do (
            set "VCVARSALL=%%I\VC\Auxiliary\Build\vcvarsall.bat"
        )
    )
)

if not defined VCVARSALL set "VCVARSALL=E:\Visual studio 2022\VC\Auxiliary\Build\vcvarsall.bat"
if not exist "%VCVARSALL%" (
    echo ERROR: vcvarsall.bat not found. Set VCVARSALL or install Visual Studio C++ tools.
    exit /b 1
)

call "%VCVARSALL%" x86
if errorlevel 1 (
    echo ERROR: vcvarsall.bat failed
    exit /b 1
)
echo [1/3] CMake configure...
cmake -S . -B "%BUILD_DIR%" -G "%CMAKE_GENERATOR%" -A "%CMAKE_PLATFORM%" -DCMAKE_PREFIX_PATH="%QT_PREFIX_PATH%"
if errorlevel 1 (
    echo ERROR: CMake configure failed
    exit /b 1
)
echo [2/3] Building CefHook DLL...
cmake --build "%BUILD_DIR%" --config Release --target CefHook
if errorlevel 1 (
    echo ERROR: CefHook build failed
    exit /b 1
)
echo [3/3] Building Injector EXE...
cmake --build "%BUILD_DIR%" --config Release --target Injector
if errorlevel 1 (
    echo ERROR: Injector build failed
    exit /b 1
)
echo.
echo BUILD SUCCESSFUL
echo DLL: %BUILD_DIR%\src\Release\CefHook.dll
echo EXE: %BUILD_DIR%\src\Release\CX_LingQiao.exe
