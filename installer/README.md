# Lingqiao Installer

This folder contains the WiX v4 installer skeleton used by the release system.

Build after the Release client is available:

```powershell
powershell -ExecutionPolicy Bypass -File installer\build_installer.ps1 -Version "3.0.0"
```

Optional signing environment:

```powershell
$env:SIGNTOOL_PATH="C:\Program Files (x86)\Windows Kits\10\bin\x64\signtool.exe"
$env:SIGN_CERT_SHA1="certificate-thumbprint"
$env:SIGN_TIMESTAMP_URL="http://timestamp.digicert.com"
```

Upload `dist\installer\LingqiaoSetup-<version>.exe` to a release as package kind `bundle`.
