<template>
  <div class="file-tree" @click="closeContextMenu">
    <div v-if="!nodes.length" class="file-tree__empty">当前文件夹为空</div>
    <FileTreeNode
      v-for="node in nodes"
      :key="node.path"
      :node="node"
      :active-path="activePath"
      :context-active-path="contextMenu.node?.path || ''"
      :search-active="searchActive"
      @open-file="$emit('open-file', $event)"
      @load-folder="$emit('load-folder', $event)"
      @node-context-menu="openContextMenu"
    />
    <div
      v-if="contextMenuOpen"
      ref="contextMenuEl"
      class="menu-panel file-tree__context-menu"
      :style="menuStyle"
      @click.stop
      @contextmenu.prevent
    >
      <button
        type="button"
        class="menu-item"
        @click="handleShowInFileManager"
      >
        打开所在位置
      </button>
      <div class="menu-divider"></div>
      <button
        type="button"
        class="menu-item"
        @click="handleFileInfo"
      >
        文件信息
      </button>
    </div>
    <FileInfoDialog
      :visible="fileInfoDialogVisible"
      :info="fileInfoData"
      :loading="fileInfoLoading"
      :error="fileInfoError"
      @close="closeFileInfoDialog"
    />
  </div>
</template>

<script setup>
import { computed, nextTick, onBeforeUnmount, reactive, ref, watch } from 'vue';
import FileTreeNode from './FileTreeNode.vue';
import FileInfoDialog from './FileInfoDialog.vue';
import { App } from '../../bindings/MostFileViewer';
import { useMenuPosition } from '../composables/useMenuPosition';
import { useExclusiveMenu } from '../composables/useExclusiveMenu';

defineProps({
  nodes: {
    type: Array,
    default: () => []
  },
  activePath: {
    type: String,
    default: ''
  },
  searchActive: {
    type: Boolean,
    default: false
  }
});

const emit = defineEmits(['open-file', 'load-folder', 'show-in-file-manager']);

const contextMenu = reactive({
  node: null
});
const {
  isMenuOpen,
  openMenu: openExclusiveMenu,
  closeMenu: closeExclusiveMenu,
} = useExclusiveMenu();
const contextMenuOpen = computed(() => isMenuOpen('context'));

const contextMenuEl = ref(null);

// 右键菜单定位：空间不足时自动翻转方向/夹紧边界，仍放不下时改为菜单内部滚动
const { menuStyle, positionMenu, stopAutoUpdate } = useMenuPosition({
  placement: 'bottom',
  align: 'start'
});

const fileInfoDialogVisible = ref(false);
const fileInfoData = ref(null);
const fileInfoLoading = ref(false);
const fileInfoError = ref('');

onBeforeUnmount(() => {
  closeContextMenu();
  stopAutoUpdate();
  document.removeEventListener('click', closeContextMenu);
});

function openContextMenu({ node, x, y }) {
  document.removeEventListener('click', closeContextMenu);
  contextMenu.node = node;
  openExclusiveMenu('context');
  nextTick(() => {
    if (contextMenuOpen.value && contextMenuEl.value) {
      positionMenu(null, contextMenuEl.value, { x, y });
    }
  });
  document.addEventListener('click', closeContextMenu, { once: true });
}

function closeContextMenu() {
  closeExclusiveMenu();
}

watch(contextMenuOpen, (isOpen) => {
  if (isOpen) {
    return;
  }
  contextMenu.node = null;
  stopAutoUpdate();
  document.removeEventListener('click', closeContextMenu);
});

function handleShowInFileManager() {
  if (!contextMenu.node) {
    return;
  }

  emit('show-in-file-manager', contextMenu.node);
  closeContextMenu();
}

async function handleFileInfo() {
  const node = contextMenu.node;
  if (!node) {
    return;
  }

  closeContextMenu();
  fileInfoDialogVisible.value = true;
  fileInfoLoading.value = true;
  fileInfoData.value = null;
  fileInfoError.value = '';

  try {
    const info = await App.GetFileInfo(node.path);
    fileInfoData.value = info;
  } catch (error) {
    fileInfoError.value = error.message || '获取文件信息失败';
  } finally {
    fileInfoLoading.value = false;
  }
}

function closeFileInfoDialog() {
  fileInfoDialogVisible.value = false;
  fileInfoData.value = null;
  fileInfoError.value = '';
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
}
</style>
