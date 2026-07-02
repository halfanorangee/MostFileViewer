// Markdown 源文（CodeMirror 编辑器）与右侧预览之间的双向同步滚动。
//
// 采用按比例联动：任一侧滚动时，取其滚动进度
//   ratio = scrollTop / (scrollHeight - clientHeight)
// 并按同一比例设置另一侧的 scrollTop。实现简单、对代码折行与图片高度差异
// 天然鲁棒；用 isSyncing 锁 + rAF 抑制回环抖动。

/**
 * 创建同步滚动控制器。
 * @param {() => import("@codemirror/view").EditorView | null} getEditor 获取当前 EditorView
 * @param {() => HTMLElement | null} getPreview 获取预览滚动容器
 * @returns {{ rebuild: () => void, dispose: () => void }}
 */
export function createScrollSync(getEditor, getPreview) {
    let isSyncing = false;
    let rafId = 0;
    let boundEditorScroller = null;
    let boundPreview = null;

    function getEditorScroller() {
        const editor = getEditor();
        return editor ? editor.scrollDOM : null;
    }

    function scrollRatio(el) {
        const max = el.scrollHeight - el.clientHeight;
        return max > 0 ? el.scrollTop / max : 0;
    }

    function applyRatio(el, ratio) {
        const max = el.scrollHeight - el.clientHeight;
        el.scrollTop = ratio * max;
    }

    function scheduleUnlock() {
        if (rafId) {
            cancelAnimationFrame(rafId);
        }
        rafId = requestAnimationFrame(() => {
            rafId = requestAnimationFrame(() => {
                isSyncing = false;
                rafId = 0;
            });
        });
    }

    function sync(fromEl, toEl) {
        if (isSyncing || !fromEl || !toEl) {
            return;
        }
        isSyncing = true;
        applyRatio(toEl, scrollRatio(fromEl));
        scheduleUnlock();
    }

    function handleEditorScroll() {
        sync(getEditorScroller(), getPreview());
    }

    function handlePreviewScroll() {
        sync(getPreview(), getEditorScroller());
    }

    // 绑定监听。预览容器可能在开启预览后才出现，因此每次 attach 都会重新
    // 绑定最新的 DOM 节点。
    function attach() {
        detach();
        boundEditorScroller = getEditorScroller();
        boundPreview = getPreview();
        boundEditorScroller?.addEventListener("scroll", handleEditorScroll, {
            passive: true,
        });
        boundPreview?.addEventListener("scroll", handlePreviewScroll, {
            passive: true,
        });
    }

    function detach() {
        boundEditorScroller?.removeEventListener("scroll", handleEditorScroll);
        boundPreview?.removeEventListener("scroll", handlePreviewScroll);
        boundEditorScroller = null;
        boundPreview = null;
    }

    function dispose() {
        if (rafId) {
            cancelAnimationFrame(rafId);
            rafId = 0;
        }
        detach();
        isSyncing = false;
    }

    // 比例同步无需采集锚点，rebuild 仅重新绑定最新的滚动容器监听。
    return { rebuild: attach, dispose };
}
