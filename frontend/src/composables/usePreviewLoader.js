import { App } from "../../bindings/MostFileViewer";
import {
    getMediaCapability,
    mediaErrorMessage,
} from "./useMediaPlayer";

export function usePreviewLoader({ getPreviewType, nextMediaSourceVersion }) {
    async function loadTabPreview(
        path,
        fallbackExtension = "",
        sourceVersion = 0,
    ) {
        const content = await App.ReadFile(path);
        const previewType = getPreviewType(content.extension || fallbackExtension);
        const isCodePreview = previewType === "code";
        const isMediaPreview = previewType === "audio" || previewType === "video";
        const loadedSource = isCodePreview
            ? null
            : await loadBinarySource(
                  content,
                  previewType,
                  isMediaPreview
                      ? sourceVersion || nextMediaSourceVersion()
                      : sourceVersion,
              );

        return {
            content,
            previewType,
            isCodePreview,
            isMediaPreview,
            source: isMediaPreview ? loadedSource?.source || null : loadedSource,
            media: isMediaPreview ? loadedSource?.media : undefined,
        };
    }

    async function loadBinarySource(content, previewType, sourceVersion = 1) {
        if (!content) {
            return null;
        }

        if (previewType === "audio" || previewType === "video") {
            const capability = getMediaCapability(previewType, content.extension);
            if (capability === "unsupported") {
                return {
                    source: null,
                    media: {
                        token: "",
                        sourceVersion,
                        capability,
                        loadState: "failed",
                        errorCode: "unsupported",
                        errorMessage: mediaErrorMessage("unsupported"),
                    },
                };
            }
            const tokenInfo = await App.RegisterMediaToken(content.path);
            return {
                source: tokenInfo.url,
                media: {
                    token: tokenInfo.token,
                    sourceVersion,
                    capability,
                    loadState: "idle",
                    errorCode: "",
                    errorMessage: "",
                    size: Number(tokenInfo.size || content.size || 0),
                    modifiedAt: tokenInfo.modifiedAt || null,
                },
            };
        }

        if (
            ["word", "excel", "ppt", "image", "pdf", "ebook"].includes(
                previewType,
            )
        ) {
            return readFileInChunks(content.path, Number(content.size || 0));
        }
        return base64ToArrayBuffer(content.base64);
    }

    async function readFileInChunks(path, totalSize) {
        if (!path || !Number.isFinite(totalSize) || totalSize <= 0) {
            return new ArrayBuffer(0);
        }

        const chunkSize = 2 * 1024 * 1024;
        const bytes = new Uint8Array(totalSize);
        let offset = 0;

        while (offset < totalSize) {
            const nextSize = Math.min(chunkSize, totalSize - offset);
            const chunk = await App.ReadFileChunk(path, offset, nextSize);
            const actualSize = Number(chunk?.size || 0);
            if (actualSize <= 0) {
                break;
            }

            writeBase64Chunk(bytes, offset, chunk.base64, actualSize);
            offset += actualSize;
        }

        return bytes.buffer;
    }

    function writeBase64Chunk(target, offset, base64, expectedSize) {
        const binary = window.atob(base64 || "");
        const size = Math.min(binary.length, expectedSize, target.length - offset);
        for (let index = 0; index < size; index += 1) {
            target[offset + index] = binary.charCodeAt(index);
        }
    }

    function base64ToArrayBuffer(base64) {
        if (!base64) {
            return null;
        }
        const binary = window.atob(base64);
        const bytes = new Uint8Array(binary.length);
        for (let index = 0; index < binary.length; index += 1) {
            bytes[index] = binary.charCodeAt(index);
        }
        return bytes.buffer;
    }

    async function revokeMediaToken(token) {
        if (!token) {
            return;
        }
        try {
            await App.RevokeMediaToken(token);
        } catch (error) {
            // Window close and server TTL provide a fallback for cleanup.
        }
    }

    async function revokeTabMedia(tab) {
        await revokeMediaToken(tab?.media?.token);
    }

    return {
        loadTabPreview,
        revokeMediaToken,
        revokeTabMedia,
    };
}
