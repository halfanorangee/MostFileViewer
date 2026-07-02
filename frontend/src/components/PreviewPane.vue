<template>
    <div class="preview-pane">
        <iframe
            v-if="isStandaloneWeb"
            class="preview-pane__frame"
            :srcdoc="content"
            :title="name ? `${name} 预览` : '网页预览'"
            sandbox="allow-scripts"
            referrerpolicy="no-referrer"
        ></iframe>
        <div
            v-else-if="isMarkdown"
            ref="markdownBody"
            class="preview-pane__md code-preview-md-body markdown-body"
            v-html="markdownHtml"
        ></div>
    </div>
</template>

<script setup>
import { computed, nextTick, onBeforeUnmount, ref, watch } from "vue";
import { renderMarkdown } from "../composables/useMarkdown.js";
import { renderMermaidDiagrams } from "../composables/useMermaid.js";
import { useTheme } from "../composables/useTheme.js";
import {
    isStandaloneWebFile,
    isMarkdownFile,
} from "../composables/useSyntaxLanguage.js";
import "./markdown-preview.css";

const props = defineProps({
    content: {
        type: String,
        default: "",
    },
    extension: {
        type: String,
        default: "",
    },
    name: {
        type: String,
        default: "",
    },
});

const markdownBody = ref(null);
const markdownHtml = ref("");
const { currentTheme } = useTheme();

const isStandaloneWeb = computed(() => isStandaloneWebFile(props.extension));
const isMarkdown = computed(() => isMarkdownFile(props.extension));

// 渲染预览区内的 mermaid 图表；需等 v-html 完成 DOM 更新后执行。
function renderDiagrams(force = false) {
    if (!isMarkdown.value) {
        return;
    }
    nextTick(() => {
        renderMermaidDiagrams(markdownBody.value, force);
    });
}

// 内容或文件切换时重新渲染 Markdown。
watch(
    [() => props.content, () => props.extension],
    () => {
        if (isMarkdown.value) {
            markdownHtml.value = renderMarkdown(props.content);
            renderDiagrams();
        } else {
            markdownHtml.value = "";
        }
    },
    { immediate: true },
);

// 明暗主题切换时重渲已完成的 mermaid 图表，使配色跟随主题。
watch(currentTheme, () => {
    renderDiagrams(true);
});

onBeforeUnmount(() => {
    markdownHtml.value = "";
});
</script>

<style scoped>
.preview-pane {
    display: flex;
    flex: 1;
    flex-direction: column;
    min-width: 0;
    height: 100%;
    min-height: 0;
    background: var(--bg-surface);
}

.preview-pane__frame {
    flex: 1;
    width: 100%;
    min-height: 0;
    border: 0;
    background: #ffffff;
}

.preview-pane__md {
    flex: 1;
    min-height: 0;
    overflow: auto;
    padding: 24px 28px;
    background: var(--bg-surface);
    color: var(--text-primary);
    font-size: 14px;
    line-height: 1.7;
    word-wrap: break-word;
}
</style>
