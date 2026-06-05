<template>
  <div class="file-tree">
    <div v-if="!nodes.length" class="file-tree__empty">当前文件夹为空</div>
    <FileTreeNode
      v-for="node in nodes"
      :key="node.path"
      :node="node"
      :active-path="activePath"
      @open-file="$emit('open-file', $event)"
    />
  </div>
</template>

<script setup>
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

defineEmits(['open-file']);
</script>
