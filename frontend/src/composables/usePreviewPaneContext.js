import { inject, provide } from "vue";

/**
 * 分屏预览树（PreviewArea → PreviewSplitNode → ...）共享的上下文注入键。
 * 递归组件层数不定，用 provide / inject 避免逐层透传大量 props 与事件。
 */
export const PREVIEW_PANE_CONTEXT = Symbol("most-file-viewer/preview-pane-context");

export function providePreviewPaneContext(context) {
    provide(PREVIEW_PANE_CONTEXT, context);
    return context;
}

export function usePreviewPaneContext() {
    return inject(PREVIEW_PANE_CONTEXT, null);
}
