package com.lingqiao.server.client;

import com.lingqiao.server.common.errors.ErrorCode;
import com.lingqiao.server.common.errors.SidecarException;
import com.lingqiao.server.payloads.PayloadCrypto;
import com.lingqiao.server.payloads.PayloadDocument;
import com.lingqiao.server.payloads.PayloadStateReader;
import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.security.MessageDigest;
import java.util.HexFormat;

/**
 * Resolves the DLL bytes to serve, mirroring Go APIHandler.loadDLLSource:
 * prefer the active packed payload, fall back to the legacy CefHook.dll file.
 */
public class DllSourceResolver {
    private final Path dataDir;
    private final PayloadStateReader stateReader;

    public DllSourceResolver(Path dataDir, PayloadStateReader stateReader) {
        this.dataDir = dataDir.toAbsolutePath().normalize();
        this.stateReader = stateReader;
    }

    public DllSource resolve() {
        var state = stateReader.read();
        String activeId = state.getActiveId();
        if (!activeId.isEmpty()) {
            PayloadDocument active = state.getPayloads().get(activeId);
            if (active != null) {
                try {
                    byte[] plain = decryptActivePayload(active);
                    return new DllSource(plain, "active-payload", active.getPayloadId(), sha256Hex(plain));
                } catch (SidecarException ignored) {
                    // fall through to legacy file, matching Go behavior
                }
            }
        }

        Path legacy = dataDir.resolve("CefHook.dll").toAbsolutePath().normalize();
        if (!legacy.startsWith(dataDir) || !Files.exists(legacy)) {
            throw new SidecarException(ErrorCode.NOT_FOUND, "DLL not found on server");
        }
        try {
            byte[] bytes = Files.readAllBytes(legacy);
            return new DllSource(bytes, "legacy-file", "", sha256Hex(bytes));
        } catch (IOException ex) {
            throw new SidecarException(ErrorCode.NOT_FOUND, "DLL not found on server");
        }
    }

    private byte[] decryptActivePayload(PayloadDocument active) {
        byte[] aesKey = PayloadCrypto.parseHex(active.getAesKey(), 32, "aes_key");
        byte[] iv = PayloadCrypto.parseHex(active.getIv(), 16, "iv");

        Path base = dataDir.resolve("payloads").resolve(active.getPayloadId())
            .toAbsolutePath().normalize();
        if (!base.startsWith(dataDir.resolve("payloads"))) {
            throw new SidecarException(ErrorCode.FORBIDDEN, "invalid payload path");
        }

        byte[] ciphertext = new byte[active.getTotalSize()];
        int offset = 0;
        for (int i = 0; i < active.getChunkCount(); i++) {
            Path chunk = base.resolve(String.format("chunk_%04d.bin", i));
            try {
                byte[] data = Files.readAllBytes(chunk);
                if (offset + data.length > ciphertext.length) {
                    throw new SidecarException(ErrorCode.INTERNAL_ERROR, "payload chunk overflow");
                }
                System.arraycopy(data, 0, ciphertext, offset, data.length);
                offset += data.length;
            } catch (IOException ex) {
                throw new SidecarException(ErrorCode.NOT_FOUND, "payload chunk missing");
            }
        }
        if (offset != ciphertext.length) {
            throw new SidecarException(ErrorCode.INTERNAL_ERROR, "payload size mismatch");
        }
        return PayloadCrypto.encryptPayload(aesKey, iv, ciphertext);
    }

    static String sha256Hex(byte[] data) {
        try {
            return HexFormat.of().formatHex(MessageDigest.getInstance("SHA-256").digest(data));
        } catch (Exception ex) {
            throw new IllegalStateException("sha256 unavailable", ex);
        }
    }

    public record DllSource(byte[] bytes, String kind, String payloadId, String sha256) {
    }
}
