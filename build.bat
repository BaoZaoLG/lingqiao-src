@echo off
cd /d "%~dp0"
call "E:\Visual studio 2022\VC\Auxiliary\Build\vcvarsall.bat" x86
if errorlevel 1 (
    echo ERROR: vcvarsall.bat failed
    exit /b 1
)
echo [1/3] CMake configure...
cmake . -G "Visual Studio 17 2022" -A Win32 -DCMAKE_PREFIX_PATH="C:/Qt/5.15.2/msvc2019"
if errorlevel 1 (
    echo ERROR: CMake configure failed
    exit /b 1
)
echo [2/3] Building CefHook DLL...
cmake --build . --config Release --target CefHook
if errorlevel 1 (
    echo ERROR: CefHook build failed
    exit /b 1
)
echo [3/3] Building Injector EXE...
cmake --build . --config Release --target Injector
if errorlevel 1 (
    echo ERROR: Injector build failed
    exit /b 1
)
echo.
echo BUILD SUCCESSFUL
echo DLL: src\Release\CefHook.dll
echo EXE: src\Release\CX鐏垫ˉ.exe