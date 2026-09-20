import docxWasmUrl from "../../node_modules/@silurus/ooxml/dist/docx_parser_bg.wasm?url";
import pptxWasmUrl from "../../node_modules/@silurus/ooxml/dist/pptx_parser_bg.wasm?url";
import xlsxWasmUrl from "../../node_modules/@silurus/ooxml/dist/xlsx_parser_bg.wasm?url";

export const OFFICE_WORKER_TIMEOUT_MS = 15000;

const OFFICE_WASM_URLS = {
    word: docxWasmUrl,
    excel: xlsxWasmUrl,
    ppt: pptxWasmUrl,
};

export function getOfficeWasmUrl(type) {
    return OFFICE_WASM_URLS[type] ?? "";
}

/**
 * 复制一份 ArrayBuffer 副本交给 Office 预览库。
 *
 * @silurus/ooxml 的 viewer.load() 会把传入的 ArrayBuffer 作为 transferable
 * 发给解析 worker，返回后原缓冲区即被 detach。而 tab.source 是共享的单一
 * 数据源：分屏 / 拖拽移动 tab 会带着同一 buffer 在新 pane 重新挂载预览组件，
 * 主题变化与会话恢复也会用同一 source 触发二次渲染——第二次 load 直接报
 * 「Cannot perform Construct on a detached ArrayBuffer」。因此每次渲染前
 * 复制副本，只把副本交给库，tab.source 本身永远不被消费。
 */
export function cloneOfficeSource(source) {
    if (source instanceof ArrayBuffer) {
        return source.slice(0);
    }
    if (ArrayBuffer.isView(source)) {
        return source.buffer.slice(
            source.byteOffset,
            source.byteOffset + source.byteLength,
        );
    }
    return source;
}

export function normalizeOfficePreviewError(error, fallback = "Office 预览失败") {
    if (error instanceof Error) {
        return error.message || fallback;
    }
    const message = String(error ?? "").trim();
    return message || fallback;
}

export function waitForRenderableHost(element, timeoutMs = 1000) {
    if (!element) {
        return Promise.resolve(false);
    }
    if (hasRenderableSize(element)) {
        return Promise.resolve(true);
    }

    return new Promise((resolve) => {
        let done = false;
        let observer = null;
        let rafId = 0;

        const finish = (ready) => {
            if (done) return;
            done = true;
            observer?.disconnect();
            if (rafId) {
                cancelAnimationFrame(rafId);
            }
            resolve(ready);
        };

        const check = () => {
            if (hasRenderableSize(element)) {
                finish(true);
            }
        };

        rafId = requestAnimationFrame(check);

        if (typeof ResizeObserver !== "undefined") {
            observer = new ResizeObserver(check);
            observer.observe(element);
        }

        setTimeout(() => finish(hasRenderableSize(element)), timeoutMs);
    });
}

function hasRenderableSize(element) {
    const rect = element.getBoundingClientRect();
    return rect.width > 0 && rect.height > 0;
}
