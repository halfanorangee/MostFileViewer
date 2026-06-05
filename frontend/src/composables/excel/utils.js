export function toArrayBuffer(src) {
    if (src instanceof ArrayBuffer) {
        return src;
    }

    if (ArrayBuffer.isView(src)) {
        return src.buffer.slice(
            src.byteOffset,
            src.byteOffset + src.byteLength,
        );
    }

    if (src?.buffer instanceof ArrayBuffer) {
        const byteOffset = src.byteOffset ?? 0;
        const byteLength = src.byteLength ?? src.buffer.byteLength;
        return src.buffer.slice(byteOffset, byteOffset + byteLength);
    }

    throw new Error("无法读取文件二进制数据");
}

export function normalizeTextEncoding(encoding) {
    const normalized = String(encoding || "").toLowerCase();
    if (["gbk", "gb2312", "gb18030"].includes(normalized)) {
        return "gb18030";
    }
    if (normalized === "utf-16le" || normalized === "utf-16be") {
        return normalized;
    }
    return "utf-8";
}

export function columnLabel(n) {
    let label = "";
    while (n > 0) {
        n--;
        label = String.fromCharCode(65 + (n % 26)) + label;
        n = Math.floor(n / 26);
    }
    return label;
}

export function cellAddressToColumnNumber(address) {
    const letters = String(address ?? "").match(/^[A-Z]+/i)?.[0] ?? "";
    let column = 0;

    for (const letter of letters.toUpperCase()) {
        column = column * 26 + letter.charCodeAt(0) - 64;
    }

    return column;
}
