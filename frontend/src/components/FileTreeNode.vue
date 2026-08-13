<template>
    <div class="tree-node">
        <button
            type="button"
            class="tree-node__row"
            :class="{ 'tree-node__row--active': isHighlighted }"
            :style="{ paddingLeft: `${depth * 16 + 12}px` }"
            @click="handleClick"
            @contextmenu.prevent.stop="handleContextMenu"
        >
            <span class="tree-node__caret">{{ caret }}</span>
            <FileIcon
                :type="isFolder ? 'folder' : 'file'"
                :name="node.name"
                :extension="node.extension"
                :open="isFolder && isExpanded"
            />
            <span class="tree-node__name">{{ node.name }}</span>
        </button>

        <div v-if="isFolder && isExpanded" class="tree-node__children">
            <FileTreeNode
                v-for="child in node.children || []"
                :key="child.path"
                :node="child"
                :depth="depth + 1"
                :active-path="activePath"
                :context-active-path="contextActivePath"
                :search-active="searchActive"
                @open-file="$emit('open-file', $event)"
                @load-folder="$emit('load-folder', $event)"
                @node-context-menu="$emit('node-context-menu', $event)"
            />
        </div>
    </div>
</template>

<script setup>
import { computed, ref, watch } from "vue";
import FileIcon from "./FileIcon.vue";
import { App } from "../../bindings/MostFileViewer";

const props = defineProps({
    node: {
        type: Object,
        required: true,
    },
    depth: {
        type: Number,
        default: 0,
    },
    activePath: {
        type: String,
        default: "",
    },
    contextActivePath: {
        type: String,
        default: "",
    },
    searchActive: {
        type: Boolean,
        default: false,
    },
});

const emit = defineEmits(["open-file", "load-folder", "node-context-menu"]);
const isFolder = computed(() => props.node.type === "folder");
const expanded = ref(false);

// 首次展开（false → true）时通知后端把本目录加入 fsnotify watcher。
// 后端靠 watchedDirs 判重，重复调用安全；折叠时不摘除（watch 集合单调增长）。
watch(expanded, (val, old) => {
    if (val && !old) {
        App.WatchFolder(props.node.path, true).catch(() => {});
    }
});

// 搜索过滤时以父级下发的 forceExpanded 为准强制展开，否则使用本地展开状态
const isExpanded = computed(() => {
    if (props.searchActive && isFolder.value) {
        return props.node.forceExpanded === true;
    }
    return expanded.value;
});

// 同一时间只展示一条背景色：右键菜单打开时以右键选中项为准，否则以左键选中项为准
const isHighlighted = computed(() => {
    if (props.contextActivePath) {
        return props.contextActivePath === props.node.path;
    }
    return props.activePath === props.node.path;
});

const caret = computed(() => {
    if (!isFolder.value) {
        return "·";
    }
    return isExpanded.value ? "▾" : "▸";
});

function handleClick() {
    if (isFolder.value) {
        // 搜索过滤时文件夹展开由 forceExpanded 强制控制，点击不切换本地展开状态
        if (props.searchActive) {
            return;
        }
        expanded.value = !expanded.value;
        if (expanded.value && props.node.hasChild && !props.node.loaded) {
            emit("load-folder", props.node);
        }
        return;
    }

    emit("open-file", props.node);
}

function handleContextMenu(event) {
    emit("node-context-menu", {
        node: props.node,
        x: event.clientX,
        y: event.clientY,
    });
}
</script>

<style scoped>
.tree-node__row {
    min-width: max-content;
    display: flex;
    align-items: center;
    gap: 4px;
    width: 100%;
    padding: 2px 4px;
    border: 0;
    background: transparent;
    color: var(--text-primary);
    text-align: left;
    cursor: pointer;
}

.tree-node__row:hover {
    background: var(--bg-hover);
}

.tree-node__row--active {
    background: var(--bg-active);
    color: var(--text-active);
}

.tree-node__caret {
    width: 12px;
    color: var(--text-muted);
    flex-shrink: 0;
}

.tree-node__name {
    flex: 0 0 auto;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
}
</style>
