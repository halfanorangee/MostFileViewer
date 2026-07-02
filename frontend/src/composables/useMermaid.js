// mermaid 图表渲染：懒加载 mermaid，将 useMarkdown 生成的占位容器
// (.mermaid-diagram[data-mermaid-source]) 渲染为 SVG，主题跟随 data-theme。

let mermaidModulePromise = null;
// 渲染出的 SVG id 需全局唯一且不能依赖随机数（构建环境禁用 Math.random），
// 用自增计数器保证唯一。
let renderSeq = 0;

// 懒加载 mermaid 主体，避免拖大首屏 bundle；仅在首次渲染图表时按需引入。
function loadMermaid() {
    if (!mermaidModulePromise) {
        mermaidModulePromise = import("mermaid").then((mod) => mod.default);
    }
    return mermaidModulePromise;
}

function decodeMermaidSource(encoded) {
    try {
        return decodeURIComponent(escape(atob(encoded)));
    } catch (e) {
        return "";
    }
}

function resolveMermaidTheme() {
    return document.documentElement.getAttribute("data-theme") === "dark"
        ? "dark"
        : "default";
}

/**
 * 渲染指定根元素下所有未处理的 mermaid 占位容器为 SVG。
 * 主题变化时可传入 force=true 强制重渲已完成的图表。
 * @param {HTMLElement | null} root 预览内容根元素
 * @param {boolean} [force] 是否强制重新渲染（用于主题切换）
 */
export async function renderMermaidDiagrams(root, force = false) {
    if (!root) {
        return;
    }

    const selector = force
        ? ".mermaid-diagram"
        : ".mermaid-diagram:not([data-mermaid-rendered])";
    const nodes = Array.from(root.querySelectorAll(selector));
    if (nodes.length === 0) {
        return;
    }

    const mermaid = await loadMermaid();
    mermaid.initialize({
        startOnLoad: false,
        theme: resolveMermaidTheme(),
        securityLevel: "strict",
    });

    for (const node of nodes) {
        const source = decodeMermaidSource(
            node.getAttribute("data-mermaid-source") || "",
        );
        if (!source) {
            continue;
        }

        const id = `mermaid-svg-${(renderSeq += 1)}`;
        try {
            const { svg } = await mermaid.render(id, source);
            node.innerHTML = svg;
            node.setAttribute("data-mermaid-rendered", "true");
            node.classList.remove("mermaid-diagram--error");
        } catch (e) {
            node.textContent = `Mermaid 渲染失败：${e?.message || e}`;
            node.setAttribute("data-mermaid-rendered", "true");
            node.classList.add("mermaid-diagram--error");
        }
    }
}
