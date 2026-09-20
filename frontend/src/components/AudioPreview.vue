<template>
  <div
    ref="rootRef"
    class="audio-preview"
    tabindex="0"
    @keydown="handleKeydown"
    @pointerdown="handleRootPointerDown"
    @wheel.prevent
  >
    <div class="audio-preview__toolbar">
      <span>{{ statusText }}</span>
      <span v-if="player.abHint.value" class="audio-preview__hint">
        {{ player.abHint.value }}
      </span>
      <span v-if="media?.notice" class="audio-preview__hint" :title="media.notice">
        {{ media.notice }}
      </span>
    </div>

    <div class="audio-preview__content">
      <audio ref="mediaRef" class="audio-preview__player" :src="resolvedSrc" preload="metadata" />
      <MediaErrorState
        v-if="isFailed"
        :error-code="errorCode"
        :error-message="errorMessage"
        :file-name="name"
        @reload="emit('media-reload')"
        @open-system="emit('media-open-system')"
      />
    </div>

    <MediaControls
      :is-playing="player.isPlaying.value"
      :load-state="player.loadState.value"
      :duration="player.duration.value"
      :current-time="player.currentTime.value"
      :volume="player.volume.value"
      :muted="player.muted.value"
      :playback-rate="player.playbackRate.value"
      :is-looping="player.isLooping.value"
      :ab-loop="player.abLoop.value"
      :show-playlist="showPlaylistControls"
      :queue-mode="queueMode"
      :has-prev="hasPrev"
      :has-next="hasNext"
      @toggle-play="player.togglePlay"
      @seek-by="player.seekBy"
      @scrub-commit="player.seekTo"
      @set-volume="player.setVolume"
      @toggle-mute="player.toggleMute"
      @set-playback-rate="player.setPlaybackRate"
      @toggle-loop="player.toggleLoop"
      @set-ab-marker="player.setAbMarker"
      @prev-track="emit('request-prev')"
      @next-track="emit('request-next', true)"
      @set-queue-mode="(mode) => emit('set-queue-mode', mode)"
      @open-system-player="emit('media-open-system')"
    />
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from "vue";
import MediaControls from "./MediaControls.vue";
import MediaErrorState from "./MediaErrorState.vue";
import {
  createMediaKeydownHandler,
  formatMediaTime,
  getMediaCapability,
  useMediaPlayer,
} from "../composables/useMediaPlayer";

const props = defineProps({
  src: {
    type: String,
    default: "",
  },
  path: {
    type: String,
    default: "",
  },
  extension: {
    type: String,
    default: "",
  },
  media: {
    type: Object,
    default: null,
  },
  queueMode: {
    type: String,
    default: "off",
  },
  hasPrev: {
    type: Boolean,
    default: false,
  },
  hasNext: {
    type: Boolean,
    default: false,
  },
});

const emit = defineEmits([
  "media-error",
  "media-reload",
  "media-open-system",
  "request-prev",
  "request-next",
]);

const rootRef = ref(null);

// 防御性复核：预检不支持的扩展名不绑定远程 src（父层预检失败时 src 也为空）
const capability = computed(() => getMediaCapability("audio", props.extension));
const resolvedSrc = computed(() =>
  capability.value === "unsupported" || props.media?.loadState === "failed"
    ? ""
    : props.src,
);

const player = useMediaPlayer({
  getSource: () => resolvedSrc.value,
  onError: (payload) => emit("media-error", payload),
  onEnded: () => emit("request-next"),
  playbackKey: () => props.path,
});

// 模板 ref 需要绑定到 setup 顶层名字；指向 composable 内部的同一个 ref
const mediaRef = player.mediaRef;

const isFailed = computed(
  () =>
    capability.value === "unsupported" ||
    player.loadState.value === "failed" ||
    props.media?.loadState === "failed",
);

const errorCode = computed(
  () =>
    props.media?.errorCode ||
    (capability.value === "unsupported" ? "unsupported" : "unknown"),
);
const errorMessage = computed(
  () => props.media?.errorMessage || "",
);

const showPlaylistControls = computed(() => props.hasPrev || props.hasNext || props.queueMode !== "off");

const statusText = computed(() => {
  if (player.loadState.value === "loading") {
    return "加载中…";
  }
  if (player.duration.value) {
    return `${formatMediaTime(player.currentTime.value)} / ${formatMediaTime(player.duration.value)}`;
  }
  return "音频预览";
});

const handleKeydown = createMediaKeydownHandler({
  togglePlay: player.togglePlay,
  seekBy: player.seekBy,
  adjustVolume: player.adjustVolume,
  toggleMute: player.toggleMute,
  cyclePlaybackRate: player.cyclePlaybackRate,
  toggleLoop: player.toggleLoop,
  setAbMarker: player.setAbMarker,
  nextTrack: () => emit("request-next"),
  prevTrack: () => emit("request-prev"),
});

// 点击非交互区域时把焦点收回播放器根节点，保证快捷键可用
function handleRootPointerDown(event) {
  if (event.target.closest("button, input, select, a, textarea")) {
    return;
  }
  rootRef.value?.focus();
}

onMounted(() => {
  rootRef.value?.focus();
});
</script>

<style scoped>
.audio-preview {
  display: flex;
  flex: 1;
  min-width: 0;
  min-height: 0;
  height: 100%;
  flex-direction: column;
  background: var(--bg-preview);
  color: var(--text-primary);
  overflow: hidden;
}

.audio-preview:focus {
  outline: none;
}

.audio-preview__toolbar {
  display: flex;
  height: 42px;
  flex-shrink: 0;
  align-items: center;
  justify-content: flex-start;
  gap: 12px;
  padding: 0 16px;
  border-bottom: 1px solid var(--border-subtle);
  background: var(--bg-toolbar);
  font-size: 13px;
}

.audio-preview__hint {
  overflow: hidden;
  font-size: 12px;
  color: var(--text-secondary);
  text-overflow: ellipsis;
  white-space: nowrap;
}

.audio-preview__content {
  position: relative;
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 0;
  background: var(--bg-surface);
}

.audio-preview__player {
  width: 100%;
  max-width: 640px;
  min-width: 280px;
  background: var(--bg-surface);
  border-radius: 8px;
  padding: 8px;
}
</style>
