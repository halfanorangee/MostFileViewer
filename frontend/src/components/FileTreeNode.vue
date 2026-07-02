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
                :open="isFolder && expanded"
            />
            <span class="tree-node__name">{{ node.name }}</span>
        </button>

        <div v-if="isFolder && expanded" class="tree-node__children">
            <FileTreeNode
                v-for="child in node.children || []"
                :key="child.path"
                :node="child"
                :depth="depth + 1"
                :active-path="activePath"
                :context-active-path="contextActivePath"
                @open-file="$emit('open-file', $event)"
                @load-folder="$emit('load-folder', $event)"
                @node-context-menu="$emit('node-context-menu', $event)"
            />
        </div>
    </div>
</template>

<script setup>
import { computed, ref } from "vue";
import FileIcon from "./FileIcon.vue";

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
});

const emit = defineEmits(["open-file", "load-folder", "node-context-menu"]);
const isFolder = computed(() => props.node.type === "folder");
const expanded = ref(false);

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
    return expanded.value ? "▾" : "▸";
});

function handleClick() {
    if (isFolder.value) {
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
