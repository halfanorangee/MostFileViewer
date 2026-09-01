<template>
  <div class="media-error">
    <div class="media-error__panel">
      <svg
        class="media-error__icon"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        stroke-width="1.6"
        stroke-linecap="round"
        stroke-linejoin="round"
        aria-hidden="true"
      >
        <circle cx="12" cy="12" r="9" />
        <path d="M12 7.5v5.5" />
        <circle cx="12" cy="16.4" r="0.4" fill="currentColor" />
      </svg>
      <p class="media-error__title">{{ title }}</p>
      <p v-if="fileName" class="media-error__file" :title="fileName">{{ fileName }}</p>
      <p class="media-error__hint">{{ hint }}</p>
      <div class="media-error__actions">
        <button
          v-if="canReload"
          type="button"
          class="media-error__btn media-error__btn--primary"
          @click="emit('reload')"
        >
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
            <path d="M20 12a8 8 0 1 1-2.34-5.66" />
            <path d="M20 4v4h-4" />
          </svg>
          重新加载
        </button>
        <button
          type="button"
          class="media-error__btn"
          @click="emit('open-system')"
        >
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
            <rect x="3" y="5" width="18" height="12" rx="2" />
            <path d="M10 20h4M12 17v3" />
            <path d="m10.5 9.5 4 2.5-4 2.5z" fill="currentColor" stroke="none" />
          </svg>
          使用系统播放器打开
        </button>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed } from "vue";
import { mediaErrorMessage } from "../composables/useMediaPlayer";

const props = defineProps({
  errorCode: {
    type: String,
    default: "unknown",
  },
  errorMessage: {
    type: String,
    default: "",
  },
  fileName: {
    type: String,
    default: "",
  },
});

const emit = defineEmits(["reload", "open-system"]);

// 依据设计：decode / source-not-supported / unsupported 只有系统播放器兜底；
// 其余错误先允许重新加载（token 重签后可能恢复）。
const RELOADABLE_CODES = new Set(["aborted", "network", "file-changed", "unknown"]);

const title = computed(
  () => props.errorMessage || mediaErrorMessage(props.errorCode),
);

const canReload = computed(() => RELOADABLE_CODES.has(props.errorCode));

const hint = computed(() => {
  switch (props.errorCode) {
    case "unsupported":
    case "source-not-supported":
    case "decode":
      return "应用内不转码，可交给系统默认播放器打开。";
    case "file-changed":
      return "源文件可能已被替换，重新加载即可读取最新内容。";
    default:
      return "可尝试重新加载，或交给系统默认播放器打开。";
  }
});
</script>

<style scoped>
.media-error {
  position: absolute;
  inset: 0;
  z-index: 2;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--bg-surface);
  color: var(--text-primary);
}

.media-error__panel {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 6px;
  max-width: 420px;
  padding: 24px 28px;
  text-align: center;
}

.media-error__icon {
  width: 34px;
  height: 34px;
  color: var(--status-error, #d93026);
}

.media-error__title {
  margin: 0;
  font-size: 15px;
  font-weight: 600;
  color: var(--text-primary);
}

.media-error__file {
  margin: 0;
  max-width: 100%;
  overflow: hidden;
  font-size: 12px;
  color: var(--text-secondary);
  text-overflow: ellipsis;
  white-space: nowrap;
}

.media-error__hint {
  margin: 0;
  font-size: 12px;
  color: var(--text-secondary);
}

.media-error__actions {
  display: flex;
  flex-wrap: wrap;
  justify-content: center;
  gap: 10px;
  margin-top: 12px;
}

.media-error__btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 7px 14px;
  border: 1px solid var(--border-default);
  border-radius: 8px;
  background: var(--bg-surface);
  color: var(--text-primary);
  font-size: 13px;
  cursor: pointer;
  transition: background-color 0.15s, border-color 0.15s;
}

.media-error__btn svg {
  width: 16px;
  height: 16px;
}

.media-error__btn:hover {
  background: var(--bg-hover);
  border-color: var(--accent-primary);
}

.media-error__btn--primary {
  border-color: var(--accent-primary);
  background: var(--accent-primary);
  color: #fff;
}

.media-error__btn--primary:hover {
  background: var(--accent-hover);
  border-color: var(--accent-hover);
  color: #fff;
}

.media-error__btn:focus-visible {
  outline: 2px solid var(--accent-primary);
  outline-offset: 2px;
}
</style>
