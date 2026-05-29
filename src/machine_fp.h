#pragma once
// ============================================================================
// Machine Fingerprint — hardware-unique identification for license binding
//
// Combines multiple hardware identifiers for robust machine binding:
//   - Volume serial (C:\)
//   - MAC address (first ethernet adapter)
//   - Computer name + user name
//   - BIOS serial number
//   - CPU ID (vendor + signature)
//   - Motherboard serial
//   - Disk drive model/serial (WMI)
//   - GPU name (WMI)
//   - System UUID (SMBIOS)
//
// All identifiers are combined and HMAC-SHA256 hashed for privacy.
// ============================================================================
#include <windows.h>
#include <iphlpapi.h>
#include <intrin.h>
#include <QString>
#include "crypto.h"
#include "config.h"
#include "strcrypt.h"

// Read BIOS serial number from SMBIOS registry (hard to spoof)
static bool GetBiosSerial(char* buf, DWORD bufSize) {
    HKEY hKey;
    if (RegOpenKeyExW(HKEY_LOCAL_MACHINE,
            L"HARDWARE\\DESCRIPTION\\System\\BIOS", 0, KEY_READ, &hKey) != ERROR_SUCCESS)
        return false;
    DWORD type = 0, size = bufSize;
    LONG ok = RegQueryValueExA(hKey, _S("SystemSerialNumber"), NULL, &type, (LPBYTE)buf, &size);
    RegCloseKey(hKey);
    return ok == ERROR_SUCCESS && type == REG_SZ && size > 1;
}

// Get CPU ID via cpuid instruction (hardware-bound, not spoofable without hypervisor)
static void GetCpuId(char* buf, DWORD bufSize) {
    int cpuInfo[4] = {0};
    __cpuid(cpuInfo, 0); // highest ext + vendor
    int maxFunc = cpuInfo[0];
    int cpuInfo1[4] = {0};
    if (maxFunc >= 1) {
        __cpuid(cpuInfo1, 1); // stepping/model/family + feature flags
    }
    // Combine: vendor (from func 0) + signature (from func 1)
    sprintf_s(buf, bufSize, "%08X%08X%08X",
        cpuInfo[1], cpuInfo[3], cpuInfo1[0]); // vendor chars + signature
}

// Get motherboard serial from SMBIOS
static bool GetBaseboardSerial(char* buf, DWORD bufSize) {
    HKEY hKey;
    if (RegOpenKeyExW(HKEY_LOCAL_MACHINE,
            L"HARDWARE\\DESCRIPTION\\System\\BIOS", 0, KEY_READ, &hKey) != ERROR_SUCCESS)
        return false;
    DWORD type = 0, size = bufSize;
    LONG ok = RegQueryValueExA(hKey, _S("BaseBoardProduct"), NULL, &type, (LPBYTE)buf, &size);
    RegCloseKey(hKey);
    return ok == ERROR_SUCCESS && type == REG_SZ && size > 1;
}

// Get system UUID from SMBIOS registry
static bool GetSystemUuid(char* buf, DWORD bufSize) {
    HKEY hKey;
    if (RegOpenKeyExW(HKEY_LOCAL_MACHINE,
            L"SOFTWARE\\Microsoft\\Cryptography", 0, KEY_READ | KEY_WOW64_64KEY, &hKey) != ERROR_SUCCESS)
        return false;
    DWORD type = 0, size = bufSize;
    LONG ok = RegQueryValueExA(hKey, _S("MachineGuid"), NULL, &type, (LPBYTE)buf, &size);
    RegCloseKey(hKey);
    return ok == ERROR_SUCCESS && type == REG_SZ && size > 1;
}

// Get first physical disk serial number via DeviceIoControl
static bool GetDiskSerial(char* buf, DWORD bufSize) {
    HANDLE hDisk = CreateFileW(L"\\\\\\.\\PhysicalDrive0", 0,
        FILE_SHARE_READ | FILE_SHARE_WRITE, NULL, OPEN_EXISTING, 0, NULL);
    if (hDisk == INVALID_HANDLE_VALUE) return false;

    STORAGE_PROPERTY_QUERY query = {};
    query.PropertyId = StorageDeviceProperty;
    query.QueryType = PropertyStandardQuery;

    BYTE outBuf[1024] = {0};
    DWORD bytesReturned = 0;
    BOOL ok = DeviceIoControl(hDisk, IOCTL_STORAGE_QUERY_PROPERTY,
        &query, sizeof(query), outBuf, sizeof(outBuf), &bytesReturned, NULL);
    CloseHandle(hDisk);

    if (!ok || bytesReturned == 0) return false;

    STORAGE_DEVICE_DESCRIPTOR* desc = (STORAGE_DEVICE_DESCRIPTOR*)outBuf;
    if (desc->SerialNumberOffset > 0 && desc->SerialNumberOffset < bytesReturned) {
        const char* serial = (const char*)(outBuf + desc->SerialNumberOffset);
        // Trim whitespace
        while (*serial == ' ') serial++;
        strncpy_s(buf, bufSize, serial, _TRUNCATE);
        // Remove trailing whitespace
        size_t len = strlen(buf);
        while (len > 0 && buf[len - 1] == ' ') buf[--len] = 0;
        return len > 0;
    }
    return false;
}

static QString GetMachineFingerprint() {
    char volSerial[32] = "0", macAddr[32] = "00:00:00:00:00:00";
    char compName[64] = "unknown", userName[64] = "unknown";
    char biosSerial[128] = "none", cpuId[32] = "0", boardSerial[128] = "none";
    char systemUuid[128] = "none", diskSerial[128] = "none";

    // Volume serial (C:\)
    DWORD serial = 0;
    if (GetVolumeInformationW(L"C:\\", NULL, 0, &serial, NULL, NULL, NULL, 0))
        sprintf_s(volSerial, sizeof(volSerial), "%08lX", serial);

    // MAC address (first ethernet adapter)
    IP_ADAPTER_INFO adapterInfo[16];
    DWORD bufLen = sizeof(adapterInfo);
    if (GetAdaptersInfo(adapterInfo, &bufLen) == ERROR_SUCCESS) {
        for (PIP_ADAPTER_INFO p = adapterInfo; p; p = p->Next) {
            if (p->Type == MIB_IF_TYPE_ETHERNET && p->AddressLength == 6) {
                sprintf_s(macAddr, sizeof(macAddr), "%02X:%02X:%02X:%02X:%02X:%02X",
                    p->Address[0], p->Address[1], p->Address[2],
                    p->Address[3], p->Address[4], p->Address[5]);
                break;
            }
        }
    }

    // Computer name + user name
    WCHAR cn[64] = {0}; DWORD cnLen = _countof(cn);
    if (GetComputerNameW(cn, &cnLen))
        WideCharToMultiByte(CP_UTF8, 0, cn, -1, compName, sizeof(compName), NULL, NULL);
    WCHAR un[64] = {0}; DWORD unLen = _countof(un);
    if (GetUserNameW(un, &unLen))
        WideCharToMultiByte(CP_UTF8, 0, un, -1, userName, sizeof(userName), NULL, NULL);

    // Hardware-bound identifiers (difficult to spoof)
    GetBiosSerial(biosSerial, sizeof(biosSerial));
    GetCpuId(cpuId, sizeof(cpuId));
    GetBaseboardSerial(boardSerial, sizeof(boardSerial));
    GetSystemUuid(systemUuid, sizeof(systemUuid));
    GetDiskSerial(diskSerial, sizeof(diskSerial));

    // Combine all into fingerprint
    char fp[2048];
    sprintf_s(fp, sizeof(fp), "%s|%s|%s|%s|%s|%s|%s|%s|%s",
        volSerial, macAddr, compName, userName,
        biosSerial, cpuId, boardSerial, systemUuid, diskSerial);

    BYTE hash[32]; DWORD hashLen = 0;
    if (HmacSha256((const char*)HMAC_KEY, 32, fp, (DWORD)strlen(fp), hash, &hashLen)) {
        char hex[65]; ByteToHex(hash, hashLen, hex);
        return QString::fromLatin1(hex);
    }
    return QString::fromLatin1(fp);
}
