<template>
    <aside class="workspace__sidebar">
        <div class="pane-container">
            <div
                class="pane-card__header"
                :class="{ 'pane-card__header--search': treeSearchActive }"
            >
                <template v-if="treeSearchActive">
                    <svg
                        xmlns="http://www.w3.org/2000/svg"
                        fill="none"
                        viewBox="0 0 24 24"
                        stroke-width="1.5"
                        stroke="currentColor"
                        class="pane-card__search-icon pane-card__search-icon--input"
                    >
                        <path
                            stroke-linecap="round"
                            stroke-linejoin="round"
                            d="m21 21-5.197-5.197m0 0A7.5 7.5 0 1 0 5.196 5.196a7.5 7.5 0 0 0 10.607 10.607Z"
                        />
                    </svg>
                    <input
                        ref="treeSearchInput"
                        v-model="treeSearchQuery"
                        type="text"
                        class="pane-card__search-input"
                        placeholder="过滤文件…"
                        @keydown.esc="closeTreeSearch"
                    />
                    <button
                        type="button"
                        class="pane-card__search-btn pane-card__search-btn--close"
                        title="关闭搜索"
                        @click="closeTreeSearch"
                    >
                        <svg
                            xmlns="http://www.w3.org/2000/svg"
                            fill="none"
                            viewBox="0 0 24 24"
                            stroke-width="1.5"
                            stroke="currentColor"
                            class="pane-card__search-icon"
                        >
                            <path
                                stroke-linecap="round"
                                stroke-linejoin="round"
                                d="M6 18 18 6M6 6l12 12"
                            />
                        </svg>
                    </button>
                </template>
                <template v-else>
                    <span class="pane-card__title">{{ folderName || "文件树" }}</span>
                    <div class="pane-card__header-actions">
                        <button
                            type="button"
                            class="pane-card__search-btn"
                            :class="{
                                'pane-card__search-btn--busy': treeRefreshing,
                            }"
                            title="刷新"
                            aria-label="刷新文件列表"
                            :disabled="treeRefreshing"
                            @click="emit('refresh')"
                        >
                            <svg
                                xmlns="http://www.w3.org/2000/svg"
                                fill="none"
                                viewBox="0 0 24 24"
                                stroke-width="1.5"
                                stroke="currentColor"
                                class="pane-card__search-icon"
                                :class="{
                                    'pane-card__search-icon--spinning':
                                        treeRefreshing,
                                }"
                            >
                                <path
                                    stroke-linecap="round"
                                    stroke-linejoin="round"
                                    d="M16.023 9.348h4.992v-.001M2.985 19.644v-4.992m0 0h4.992m-4.993 0 3.181 3.183a8.25 8.25 0 0 0 13.803-3.7M4.031 9.865a8.25 8.25 0 0 1 13.803-3.7l3.181 3.182m0-4.991v4.99"
                                />
                            </svg>
                        </button>
                        <button
                            type="button"
                            class="pane-card__search-btn"
                            title="搜索"
                            @click="handleTreeSearch"
                        >
                            <svg
                                xmlns="http://www.w3.org/2000/svg"
                                fill="none"
                                viewBox="0 0 24 24"
                                stroke-width="1.5"
                                stroke="currentColor"
                                class="pane-card__search-icon"
                            >
                                <path
                                    stroke-linecap="round"
                                    stroke-linejoin="round"
                                    d="m21 21-5.197-5.197m0 0A7.5 7.5 0 1 0 5.196 5.196a7.5 7.5 0 0 0 10.607 10.607Z"
                                />
                            </svg>
                        </button>
                    </div>
                </template>
            </div>
            <FileTree
                :nodes="displayTreeData"
                :active-path="activePath"
                :search-active="isTreeFiltering"
                @open-file="(node) => emit('open-file', node)"
                @load-folder="(node) => emit('load-folder', node)"
                @show-in-file-manager="(node) => emit('show-in-file-manager', node)"
            />
        </div>
    </aside>
</template>

<script setup>
import { computed, nextTick, ref } from "vue";
import FileTree from "./FileTree.vue";

const props = defineProps({
    // 根目录显示名（取自 selectedFolder 的末段），空时展示「文件树」
    folderName: {
        type: String,
        default: "",
    },
    // 文件树原始节点（组件内部负责按关键字过滤）
    nodes: {
        type: Array,
        default: () => [],
    },
    activePath: {
        type: String,
        default: "",
    },
    treeRefreshing: {
        type: Boolean,
        default: false,
    },
});

const emit = defineEmits([
    "open-file",
    "load-folder",
    "show-in-file-manager",
    "refresh",
]);

const treeSearchActive = ref(false);
const treeSearchQuery = ref("");
const treeSearchInput = ref(null);

// 是否处于有效的搜索过滤状态（激活且有非空关键字）
const isTreeFiltering = computed(
    () => treeSearchActive.value && treeSearchQuery.value.trim() !== "",
);

// 根据搜索关键字过滤文件树：保留名称匹配的文件，以及包含匹配项的文件夹（并强制展开）。
// 注意：文件树为懒加载，未展开的文件夹其子节点尚未加载，过滤仅作用于已加载的节点。
const displayTreeData = computed(() => {
    if (!isTreeFiltering.value) {
        return props.nodes;
    }
    const keyword = treeSearchQuery.value.trim().toLowerCase();
    return filterTreeNodes(props.nodes, keyword);
});

function filterTreeNodes(nodes, keyword) {
    const result = [];
    for (const node of nodes) {
        const nameMatched = (node.name || "").toLowerCase().includes(keyword);
        if (node.type === "folder") {
            const filteredChildren = filterTreeNodes(
                node.children || [],
                keyword,
            );
            if (nameMatched || filteredChildren.length > 0) {
                result.push({
                    ...node,
                    children: filteredChildren,
                    // 过滤时强制展开以显示命中的后代节点
                    forceExpanded: filteredChildren.length > 0,
                });
            }
        } else if (nameMatched) {
            result.push({ ...node });
        }
    }
    return result;
}

function handleTreeSearch() {
    treeSearchActive.value = true;
    nextTick(() => {
        treeSearchInput.value?.focus();
    });
}

function closeTreeSearch() {
    treeSearchActive.value = false;
    treeSearchQuery.value = "";
}
</script>

<style scoped>
.pane-container {
    border-radius: 10px;
    display: flex;
    flex-direction: column;
    height: 100%;
    min-height: 0;
    overflow: hidden;
    background-color: var(--bg-surface);
}

.pane-card__header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 2px 12px;
    font-size: 16px;
    border-bottom: 1px solid var(--border-subtle);
    font-weight: 600;
}

.pane-card__title {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
}

.pane-card__header-actions {
    display: flex;
    align-items: center;
    flex-shrink: 0;
}

.pane-card__search-btn {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    padding: 4px;
    border: none;
    background: transparent;
    border-radius: 4px;
    color: var(--text-secondary, inherit);
    cursor: pointer;
    opacity: 0;
    visibility: hidden;
    transition: opacity 0.15s ease, background-color 0.15s ease;
}

.workspace__sidebar {
    padding: 0 0 4px;
    background: var(--bg-titlebar);
}

.workspace__sidebar:hover .pane-card__search-btn,
.pane-card__search-btn--busy {
    opacity: 1;
    visibility: visible;
}

.pane-card__search-btn:hover {
    background-color: var(--bg-hover, rgba(0, 0, 0, 0.08));
}

.pane-card__search-icon {
    width: 16px;
    height: 16px;
}

.pane-card__header--search {
    gap: 6px;
}

.pane-card__search-icon--input {
    flex-shrink: 0;
    color: var(--text-secondary, inherit);
}

.pane-card__search-input {
    flex: 1;
    min-width: 0;
    border: none;
    outline: none;
    background: transparent;
    font-size: 14px;
    font-weight: 400;
    color: var(--text-primary, inherit);
}

.pane-card__search-input::placeholder {
    color: var(--text-muted, #999);
}

.pane-card__search-btn:disabled {
    cursor: default;
}

.pane-card__search-btn:disabled:hover {
    background-color: transparent;
}

.pane-card__search-icon--spinning {
    animation: pane-card-icon-spin 0.8s linear infinite;
}

@keyframes pane-card-icon-spin {
    to {
        transform: rotate(360deg);
    }
}

.pane-card__search-btn--close {
    opacity: 1;
    visibility: visible;
    flex-shrink: 0;
}
</style>

