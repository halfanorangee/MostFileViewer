<template>
  <div class="file-tree" @click="closeContextMenu">
    <div v-if="!nodes.length" class="file-tree__empty">当前文件夹为空</div>
    <FileTreeNode
      v-for="node in nodes"
      :key="node.path"
      :node="node"
      :active-path="activePath"
      :context-active-path="contextMenu.node?.path || ''"
      @open-file="$emit('open-file', $event)"
      @load-folder="$emit('load-folder', $event)"
      @node-context-menu="openContextMenu"
    />
    <div
      v-if="contextMenu.open"
      class="file-tree__context-menu"
      :style="contextMenuStyle"
      @click.stop
      @contextmenu.prevent
    >
      <button
        type="button"
        class="file-tree__context-menu-item"
        @click="handleShowInFileManager"
      >
        在文件管理器中显示
      </button>
    </div>
  </div>
</template>

<script setup>
import { computed, onBeforeUnmount, reactive } from 'vue';
import FileTreeNode from './FileTreeNode.vue';

defineProps({
  nodes: {
    type: Array,
    default: () => []
  },
  activePath: {
    type: String,
    default: ''
  }
});

const emit = defineEmits(['open-file', 'load-folder', 'show-in-file-manager']);

const contextMenu = reactive({
  open: false,
  x: 0,
  y: 0,
  node: null
});

const contextMenuStyle = computed(() => ({
  left: `${contextMenu.x}px`,
  top: `${contextMenu.y}px`
}));

onBeforeUnmount(() => {
  document.removeEventListener('click', closeContextMenu);
});

function openContextMenu({ node, x, y }) {
  document.removeEventListener('click', closeContextMenu);
  contextMenu.open = true;
  contextMenu.x = x;
  contextMenu.y = y;
  contextMenu.node = node;
  document.addEventListener('click', closeContextMenu, { once: true });
}

function closeContextMenu() {
  contextMenu.open = false;
  document.removeEventListener('click', closeContextMenu);
}

function handleShowInFileManager() {
  if (!contextMenu.node) {
    return;
  }

  emit('show-in-file-manager', contextMenu.node);
  closeContextMenu();
}
</script>

<style scoped>
.file-tree {
  position: relative;
  height: 100%;
  min-height: 0;
}

.file-tree__context-menu {
  position: fixed;
  z-index: 200;
  min-width: 160px;
  padding: 4px;
  background: #fff;
  border: 1px solid #d1d9e6;
  border-radius: 6px;
  box-shadow: 0 8px 24px rgba(15, 23, 42, 0.12);
}

.file-tree__context-menu-item {
  width: 100%;
  padding: 8px 12px;
  border: 0;
  border-radius: 4px;
  background: transparent;
  color: #334155;
  font-size: 13px;
  text-align: left;
  cursor: pointer;
}

.file-tree__context-menu-item:hover {
  background: #f3f7ff;
  color: #154ec1;
}
</style>
