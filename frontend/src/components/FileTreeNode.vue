<template>
    <div class="tree-node">
        <button
            type="button"
            class="tree-node__row"
            :class="{
                'tree-node__row--active': activePath === node.path,
            }"
            :style="{ paddingLeft: `${depth * 16 + 12}px` }"
            @click="handleClick"
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
                @open-file="$emit('open-file', $event)"
                @load-folder="$emit('load-folder', $event)"
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
});

const emit = defineEmits(["open-file", "load-folder"]);
const isFolder = computed(() => props.node.type === "folder");
const expanded = ref(false);

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
</script>

<style scoped>
.tree-node__row {
    display: flex;
    align-items: center;
    gap: 8px;
    width: 100%;
    padding: 8px 12px;
    border: 0;
    background: transparent;
    color: #1f2937;
    text-align: left;
    cursor: pointer;
}

.tree-node__row:hover {
    background: #f5f8fc;
}

.tree-node__row--active {
    background: #eaf2ff;
    color: #0f172a;
}

.tree-node__caret {
    width: 12px;
    color: #64748b;
    flex-shrink: 0;
}

.tree-node__name {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
}
</style>
