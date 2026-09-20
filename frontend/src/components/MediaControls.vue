<template>
  <div ref="rootRef" class="media-controls">
    <div class="media-controls__progress">
      <div class="media-controls__track">
        <input
          type="range"
          class="media-controls__progress-bar"
          :style="{ '--progress': displayPercent + '%' }"
          :min="0"
          :max="duration || 0"
          :step="0.1"
          :value="displayTime"
          :disabled="!duration || loadState === 'failed'"
          :aria-label="'播放进度'"
          @input="handleScrubInput"
          @change="handleScrubCommit"
        />
        <span
          v-if="abStartPercent !== null"
          class="media-controls__ab-mark"
          :style="{ left: abStartPercent + '%' }"
          title="A 点"
        ></span>
        <span
          v-if="abEndPercent !== null"
          class="media-controls__ab-mark"
          :style="{ left: abEndPercent + '%' }"
          title="B 点"
        ></span>
      </div>
      <div class="media-controls__time">
        <span>{{ formatTime(displayTime) }}</span>
        <span>/</span>
        <span>{{ formatTime(duration) }}</span>
      </div>
    </div>

    <div class="media-controls__buttons">
      <template v-if="showPlaylist">
        <button
          type="button"
          class="media-controls__btn"
          :disabled="!hasPrev"
          title="上一首 (P)"
          aria-label="上一首"
          @click="emit('prev-track')"
        >
          <svg viewBox="0 0 24 24" aria-hidden="true">
            <path d="M6 5h2v14H6z" />
            <path d="M19 5.8v12.4a1 1 0 0 1-1.54.84l-9.4-6.2a1 1 0 0 1 0-1.68l9.4-6.2A1 1 0 0 1 19 5.8z" />
          </svg>
        </button>
      </template>

      <button
        type="button"
        class="media-controls__btn media-controls__btn--primary"
        :class="{ active: isPlaying }"
        :disabled="loadState === 'failed'"
        title="播放/暂停 (空格)"
        aria-label="播放或暂停"
        @click="emit('toggle-play')"
      >
        <svg v-if="isPlaying" viewBox="0 0 24 24" aria-hidden="true">
          <rect x="6" y="4" width="4" height="16" rx="1" />
          <rect x="14" y="4" width="4" height="16" rx="1" />
        </svg>
        <svg v-else viewBox="0 0 24 24" aria-hidden="true">
          <path d="M8 5.5v13a1 1 0 0 0 1.53.85l9.5-6.5a1 1 0 0 0 0-1.7l-9.5-6.5A1 1 0 0 0 8 5.5Z" />
        </svg>
      </button>

      <template v-if="showPlaylist">
        <button
          type="button"
          class="media-controls__btn"
          :disabled="!hasNext"
          title="下一首 (N)"
          aria-label="下一首"
          @click="emit('next-track')"
        >
          <svg viewBox="0 0 24 24" aria-hidden="true">
            <path d="M16 5h2v14h-2z" />
            <path d="M5 5.8v12.4a1 1 0 0 0 1.54.84l9.4-6.2a1 1 0 0 0 0-1.68l-9.4-6.2A1 1 0 0 0 5 5.8z" />
          </svg>
        </button>
      </template>

      <button
        type="button"
        class="media-controls__btn"
        :disabled="!duration"
        title="后退 10 秒 (←)"
        aria-label="后退"
        @click="emit('seek-by', -10)"
      >
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
          <path d="M4 12a8 8 0 1 0 2.34-5.66" />
          <path d="M4 4v4h4" />
        </svg>
      </button>

      <button
        type="button"
        class="media-controls__btn"
        :disabled="!duration"
        title="前进 10 秒 (→)"
        aria-label="前进"
        @click="emit('seek-by', 10)"
      >
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
          <path d="M20 12a8 8 0 1 1-2.34-5.66" />
          <path d="M20 4v4h-4" />
        </svg>
      </button>

      <button
        type="button"
        class="media-controls__btn media-controls__btn--ab"
        :class="{ active: abLoop.start !== null }"
        :disabled="!duration"
        title="设置/清除 A-B 循环 (A)"
        aria-label="设置或清除 A-B 循环"
        @click="emit('set-ab-marker')"
      >
        A-B
      </button>

      <button
        type="button"
        class="media-controls__btn"
        :class="{ active: isLooping }"
        :disabled="!duration"
        title="循环播放 (L)"
        aria-label="切换循环播放"
        @click="emit('toggle-loop')"
      >
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
          <path d="M17 3l3 3-3 3" />
          <path d="M20 6H8a4 4 0 0 0-4 4v1" />
          <path d="M7 21l-3-3 3-3" />
          <path d="M4 18h12a4 4 0 0 0 4-4v-1" />
        </svg>
      </button>

      <div class="media-controls__menu-wrap">
        <button
          type="button"
          class="media-controls__btn"
          :class="{ active: playbackRate !== 1 }"
          title="播放速度 ([ / ])"
          aria-label="调整播放速度"
          @click="toggleMenu('speed', $event)"
        >
          <span class="media-controls__rate-text">{{ rateLabel }}</span>
        </button>
        <div
          v-if="openMenu === 'speed'"
          class="menu-panel media-controls__menu media-controls__menu--speed"
          :style="menuStyle"
          tabindex="-1"
          @keydown.esc.stop.prevent="closeMenu"
          @keydown.stop
        >
          <button
            v-for="rate in playbackRates"
            :key="rate"
            type="button"
            class="menu-item media-controls__option-item"
            :class="{ 'media-controls__option-item--active': rate === playbackRate }"
            @click="selectPlaybackRate(rate)"
          >
            <span class="media-controls__option-label">{{ rate }}x</span>
            <span
              v-if="rate === playbackRate"
              class="media-controls__option-check"
              aria-hidden="true"
            >✓</span>
          </button>
        </div>
      </div>

      <template v-if="showPlaylist">
        <div class="media-controls__menu-wrap">
          <button
            type="button"
            class="media-controls__btn"
            :class="{ active: queueMode !== 'off' }"
            title="连续播放模式"
            aria-label="选择连续播放模式"
            @click="toggleMenu('queue', $event)"
          >
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
              <path v-if="queueMode === 'shuffle'" d="M3 7h4l10 10h4m0 0-3-3m3 3-3 3M3 17h4l2.5-2.5M13.5 9.5 17 7h4m0 0-3-3m3 3-3 3" />
              <template v-else>
                <path d="M4 6h16M4 12h16M4 18h10" />
                <path v-if="queueMode === 'sequential'" d="m18 15 2 3-2 3" />
              </template>
            </svg>
          </button>
          <div
            v-if="openMenu === 'queue'"
            class="media-controls__menu"
            :style="menuStyle"
            tabindex="-1"
            @keydown.esc.stop.prevent="closeMenu"
            @keydown.stop
          >
            <button
              v-for="mode in queueModes"
              :key="mode.value"
              type="button"
              class="menu-item media-controls__option-item"
              :class="{ 'media-controls__option-item--active': mode.value === queueMode }"
              @click="selectQueueMode(mode.value)"
            >
              <span class="media-controls__option-label">{{ mode.label }}</span>
              <span v-if="mode.value === queueMode" class="media-controls__option-check" aria-hidden="true">✓</span>
            </button>
          </div>
        </div>
      </template>

      <button
        type="button"
        class="media-controls__btn media-controls__btn--mute"
        :class="{ active: muted }"
        :title="muted ? '取消静音 (M)' : '静音 (M)'"
        :aria-label="muted ? '取消静音' : '静音'"
        @click="emit('toggle-mute')"
      >
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
          <path d="M11 5 6 9H3v6h3l5 4V5Z" />
          <template v-if="muted || volume === 0">
            <path d="m19 9-4 6" />
            <path d="m15 9 4 6" />
          </template>
          <template v-else>
            <path d="M15.5 9.5a3.5 3.5 0 0 1 0 5" />
            <path d="M18 7a7 7 0 0 1 0 10" />
          </template>
        </svg>
      </button>

      <div class="media-controls__volume">
        <input
          type="range"
          class="media-controls__volume-slider"
          :min="0"
          :max="1"
          :step="0.05"
          :value="displayVolume"
          :style="{ '--progress': volumePercent + '%' }"
          :aria-label="'音量'"
          @input="handleVolumeInput"
        />
      </div>

      <template v-if="showSubtitles">
        <div class="media-controls__menu-wrap">
          <button
            type="button"
            class="media-controls__btn"
            :class="{ active: subtitleEnabled }"
            title="字幕"
            aria-label="字幕设置"
            @click="toggleMenu('subtitle', $event)"
          >
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
              <rect x="3" y="5" width="18" height="14" rx="2" />
              <path d="M7 14h3M13 14h4M7 10.5h2M11.5 10.5h3M17.5 10.5h-.5" />
            </svg>
          </button>
          <div
            v-if="openMenu === 'subtitle'"
            class="menu-panel media-controls__menu media-controls__menu--subtitle"
            :style="menuStyle"
            role="menu"
            tabindex="-1"
            @keydown.esc.stop.prevent="closeMenu"
            @keydown.stop
          >
            <button
              v-if="subtitleTracks.length"
              type="button"
              class="menu-item media-controls__option-item"
              :class="{ 'media-controls__option-item--active': !subtitleEnabled }"
              role="menuitemradio"
              :aria-checked="!subtitleEnabled"
              @click="selectSubtitleTrack('off')"
            >
              <span class="media-controls__option-label">关闭字幕</span>
              <span
                v-if="!subtitleEnabled"
                class="media-controls__option-check"
                aria-hidden="true"
              >✓</span>
            </button>
            <button
              v-for="track in subtitleTracks"
              :key="track.id"
              type="button"
              class="menu-item media-controls__option-item"
              :class="{ 'media-controls__option-item--active': track.active }"
              role="menuitemradio"
              :aria-checked="track.active"
              :aria-busy="subtitleLoadingId === track.id"
              @click="selectSubtitleTrack(track.id)"
            >
              <span class="media-controls__option-label">{{ track.label }}</span>
              <span
                v-if="subtitleLoadingId === track.id"
                class="media-controls__menu-spinner"
                aria-hidden="true"
              ></span>
              <span
                v-else-if="track.active"
                class="media-controls__option-check"
                aria-hidden="true"
              >✓</span>
            </button>
            <div v-if="!subtitleTracks.length" class="media-controls__menu-empty">暂无可用字幕</div>
            <div v-if="subtitleError" class="media-controls__menu-error">{{ subtitleError }}</div>
            <div
              v-if="subtitleLoadingId && !subtitleTracks.some((track) => track.id === subtitleLoadingId)"
              class="media-controls__menu-empty"
            >
              正在加载字幕…
            </div>
            <div class="menu-divider"></div>
            <button
              type="button"
              class="menu-item media-controls__option-item"
              role="menuitem"
              @click="pickSubtitleFile"
            >
              <span class="media-controls__option-label">选择字幕文件</span>
            </button>
          </div>
        </div>

        <button
          type="button"
          class="media-controls__btn"
          :disabled="!frameStepEnabled"
          title="上一帧 (,)"
          aria-label="上一帧"
          @click="emit('step-frame', -1)"
        >
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
            <path d="M11 6 4.8 11.4a.8.8 0 0 0 0 1.2L11 18z" />
            <path d="M19 6l-6.2 5.4a.8.8 0 0 0 0 1.2L19 18z" />
          </svg>
        </button>

        <button
          type="button"
          class="media-controls__btn"
          :disabled="!frameStepEnabled"
          title="下一帧 (.)"
          aria-label="下一帧"
          @click="emit('step-frame', 1)"
        >
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
            <path d="M13 6l6.2 5.4a.8.8 0 0 1 0 1.2L13 18z" />
            <path d="M5 6l6.2 5.4a.8.8 0 0 1 0 1.2L5 18z" />
          </svg>
        </button>
      </template>

      <button
        v-if="showFullscreen"
        type="button"
        class="media-controls__btn"
        :title="isFullscreen ? '退出全屏 (F)' : '全屏 (F)'"
        :aria-label="isFullscreen ? '退出全屏' : '进入全屏'"
        @click="emit('toggle-fullscreen')"
      >
        <svg v-if="isFullscreen" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
          <path d="M8 3v3a2 2 0 0 1-2 2H3m18 0h-3a2 2 0 0 1-2-2V3m0 18v-3a2 2 0 0 1 2-2h3M3 16h3a2 2 0 0 1 2 2v3" />
        </svg>
        <svg v-else viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
          <path d="M8 3H5a2 2 0 0 0-2 2v3m18 0V5a2 2 0 0 0-2-2h-3m0 18h3a2 2 0 0 0 2-2v-3M3 16v3a2 2 0 0 0 2 2h3" />
        </svg>
      </button>

      <button
        type="button"
        class="media-controls__btn media-controls__btn--system"
        title="使用系统播放器打开"
        aria-label="使用系统播放器打开"
        @click="emit('open-system-player')"
      >
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
          <rect x="3" y="5" width="18" height="12" rx="2" />
          <path d="M10 20h4M12 17v3" />
          <path d="m10.5 9.5 4 2.5-4 2.5z" fill="currentColor" stroke="none" />
        </svg>
      </button>
    </div>
  </div>
</template>

<script setup>
import { computed, nextTick, onBeforeUnmount, ref, watch } from "vue";
import { MEDIA_PLAYBACK_RATES, formatMediaTime } from "../composables/useMediaPlayer";
import { useMenuPosition } from "../composables/useMenuPosition";
import { useExclusiveMenu } from "../composables/useExclusiveMenu";

const props = defineProps({
  isPlaying: {
    type: Boolean,
    default: false,
  },
  loadState: {
    type: String,
    default: "idle",
  },
  duration: {
    type: Number,
    default: 0,
  },
  currentTime: {
    type: Number,
    default: 0,
  },
  volume: {
    type: Number,
    default: 1,
  },
  muted: {
    type: Boolean,
    default: false,
  },
  playbackRate: {
    type: Number,
    default: 1,
  },
  isLooping: {
    type: Boolean,
    default: false,
  },
  abLoop: {
    type: Object,
    default: () => ({ start: null, end: null }),
  },
  showPlaylist: {
    type: Boolean,
    default: false,
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
  showFullscreen: {
    type: Boolean,
    default: false,
  },
  isFullscreen: {
    type: Boolean,
    default: false,
  },
  showFrameStep: {
    type: Boolean,
    default: false,
  },
  frameStepEnabled: {
    type: Boolean,
    default: false,
  },
  showSubtitles: {
    type: Boolean,
    default: false,
  },
  subtitleEnabled: {
    type: Boolean,
    default: false,
  },
  subtitleTracks: {
    type: Array,
    default: () => [],
  },
  subtitleLoadingId: {
    type: String,
    default: "",
  },
  subtitleError: {
    type: String,
    default: "",
  },
});

const emit = defineEmits([
  "toggle-play",
  "seek-by",
  "scrub-commit",
  "set-volume",
  "toggle-mute",
  "set-playback-rate",
  "toggle-loop",
  "set-ab-marker",
  "toggle-fullscreen",
  "step-frame",
  "select-subtitle",
  "pick-subtitle-file",
  "prev-track",
  "next-track",
  "set-queue-mode",
  "open-system-player",
]);

const playbackRates = MEDIA_PLAYBACK_RATES;
const queueModes = [
  { value: "off", label: "关闭连播" },
  { value: "sequential", label: "顺序连播" },
  { value: "shuffle", label: "随机连播" },
];

const rootRef = ref(null);
const {
  activeMenu: openMenu,
  toggleMenu: toggleExclusiveMenu,
  closeMenu: closeExclusiveMenu,
} = useExclusiveMenu();
let removeOutsideClickListener = null;

// 菜单定位：空间不足时自动翻转方向/夹紧边界，仍放不下时改为菜单内部滚动
const { menuStyle, positionMenu, stopAutoUpdate } = useMenuPosition({
  placement: "top",
  align: "start",
  maxHeight: 320,
});

const rateLabel = computed(() => (props.playbackRate === 1 ? "倍速" : `${props.playbackRate}x`));

const isScrubbing = ref(false);
const scrubValue = ref(0);
const displayTime = computed(() =>
  isScrubbing.value ? scrubValue.value : props.currentTime,
);

const displayPercent = computed(() => {
  if (!props.duration) {
    return 0;
  }
  return Math.min(100, Math.max(0, (displayTime.value / props.duration) * 100));
});

const displayVolume = computed(() => (props.muted ? 0 : props.volume));
const volumePercent = computed(() => Math.min(100, Math.max(0, displayVolume.value * 100)));

const abStartPercent = computed(() => {
  if (!props.duration || props.abLoop?.start === null || props.abLoop?.start === undefined) {
    return null;
  }
  return Math.min(100, Math.max(0, (props.abLoop.start / props.duration) * 100));
});

const abEndPercent = computed(() => {
  if (!props.duration || props.abLoop?.end === null || props.abLoop?.end === undefined) {
    return null;
  }
  return Math.min(100, Math.max(0, (props.abLoop.end / props.duration) * 100));
});

function formatTime(seconds) {
  return formatMediaTime(seconds);
}

// 拖动进度条时仅更新预览值，释放（change）时一次性 seek
function handleScrubInput(event) {
  isScrubbing.value = true;
  scrubValue.value = Number(event.target.value);
}

function handleScrubCommit(event) {
  const value = Number(event.target.value);
  isScrubbing.value = false;
  if (Number.isFinite(value)) {
    emit("scrub-commit", value);
  }
}

function handleVolumeInput(event) {
  emit("set-volume", Number(event.target.value));
}

function toggleMenu(name, event) {
  if (openMenu.value === name) {
    closeMenu();
    return;
  }
  toggleExclusiveMenu(name);
  if (!openMenu.value) {
    return;
  }
  const anchor =
    event && event.currentTarget instanceof Element ? event.currentTarget : null;
  nextTick(() => {
    const menu = rootRef.value?.querySelector('.media-controls__menu[tabindex="-1"]');
    if (menu) {
      positionMenu(anchor, menu);
    }
    menu?.focus();
  });
  syncOutsideClickListener();
}

function closeMenu() {
  closeExclusiveMenu();
  stopAutoUpdate();
  syncOutsideClickListener();
}

function handleOutsideClick(event) {
  if (event.target?.closest?.(".media-controls__menu, .media-controls__menu-wrap")) {
    return;
  }
  closeMenu();
}

function syncOutsideClickListener() {
  if (openMenu.value && !removeOutsideClickListener) {
    document.addEventListener("mousedown", handleOutsideClick);
    removeOutsideClickListener = () => {
      document.removeEventListener("mousedown", handleOutsideClick);
    };
  } else if (!openMenu.value && removeOutsideClickListener) {
    removeOutsideClickListener();
    removeOutsideClickListener = null;
  }
}

watch(openMenu, (menuName) => {
  if (!menuName) {
    stopAutoUpdate();
  }
  syncOutsideClickListener();
});

function selectPlaybackRate(rate) {
  emit("set-playback-rate", rate);
  closeMenu();
}

function selectQueueMode(mode) {
  emit("set-queue-mode", mode);
  closeMenu();
}

function selectSubtitleTrack(trackId) {
  emit("select-subtitle", trackId);
  closeMenu();
}

function pickSubtitleFile() {
  emit("pick-subtitle-file");
  closeMenu();
}

onBeforeUnmount(() => {
  closeMenu();
});
</script>

<style scoped>
.media-controls {
  display: flex;
  flex-direction: column;
  flex-shrink: 0;
  background: var(--bg-surface);
}

.media-controls__progress {
  display: flex;
  align-items: center;
  gap: 14px;
  min-height: 24px;
  padding: 2px 16px 0;
  border-top: 1px solid var(--border-subtle);
}

.media-controls__track {
  position: relative;
  flex: 1;
  min-width: 0;
  display: flex;
  align-items: center;
}

.media-controls__progress-bar {
  width: 100%;
  min-width: 0;
  height: 4px;
  border-radius: 99px;
  appearance: none;
  background: linear-gradient(
    to right,
    var(--accent-primary) var(--progress),
    var(--border-default) var(--progress)
  );
  accent-color: var(--accent-primary);
  cursor: pointer;
}

.media-controls__progress-bar:disabled {
  cursor: default;
  opacity: 0.5;
}

.media-controls__progress-bar::-webkit-slider-runnable-track {
  height: 4px;
  border-radius: 99px;
  background: transparent;
}

.media-controls__progress-bar::-webkit-slider-thumb {
  width: 12px;
  height: 12px;
  margin-top: -4px;
  appearance: none;
  border: 2px solid var(--accent-primary);
  border-radius: 50%;
  background: var(--bg-surface);
  box-shadow: 0 0 0 3px var(--accent-overlay);
}

.media-controls__progress-bar::-moz-range-track {
  height: 4px;
  border-radius: 99px;
  background: var(--border-default);
}

.media-controls__progress-bar::-moz-range-progress {
  height: 4px;
  border-radius: 99px;
  background: var(--accent-primary);
}

.media-controls__progress-bar::-moz-range-thumb {
  width: 8px;
  height: 8px;
  border: 2px solid var(--accent-primary);
  border-radius: 50%;
  background: var(--bg-surface);
}

.media-controls__ab-mark {
  position: absolute;
  top: 50%;
  width: 2px;
  height: 12px;
  transform: translateY(-50%);
  border-radius: 1px;
  background: var(--accent-primary);
  box-shadow: 0 0 0 1px var(--bg-surface);
  pointer-events: none;
}

.media-controls__time {
  display: flex;
  gap: 2px;
  font-variant-numeric: tabular-nums;
  font-size: 11px;
  color: var(--text-secondary);
  white-space: nowrap;
  min-width: 80px;
  text-align: right;
}

.media-controls__buttons {
  display: flex;
  align-items: center;
  gap: 4px;
  min-height: 36px;
  padding: 0 12px 4px;
}

.media-controls__btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  padding: 0;
  border: 1px solid transparent;
  background: transparent;
  color: var(--text-secondary);
  border-radius: 6px;
  cursor: pointer;
  transition: border-color 0.15s, color 0.15s;
  flex-shrink: 0;
}

.media-controls__btn svg {
  width: 17px;
  height: 17px;
  fill: none;
  stroke: currentColor;
}

.media-controls__btn:disabled {
  cursor: default;
  opacity: 0.4;
}

.media-controls__btn:not(:disabled):hover {
  background: var(--bg-hover);
  border-color: var(--border-default);
  color: var(--text-primary);
}

.media-controls__btn.active {
  color: var(--accent-primary);
  background: var(--accent-overlay);
  border-color: transparent;
}

.media-controls__btn:focus-visible {
  outline: 2px solid var(--accent-primary);
  outline-offset: 2px;
}

.media-controls__btn--primary {
  color: #fff;
  background: var(--accent-primary);
  border-color: var(--accent-primary);
  border-radius: 50%;
}

.media-controls__btn--primary svg {
  fill: currentColor;
  stroke: none;
}

.media-controls__btn--primary:not(:disabled):hover {
  color: #fff;
  background: var(--accent-hover);
  border-color: var(--accent-hover);
}

.media-controls__btn--primary.active {
  color: #fff;
  background: var(--accent-primary);
  border-color: var(--accent-primary);
}

.media-controls__btn--ab {
  font-size: 10px;
  font-weight: 600;
  letter-spacing: 0.02em;
}

.media-controls__jump-text {
  font-size: 9px;
  font-weight: 600;
  fill: currentColor;
  stroke: none;
}

.media-controls__rate-text {
  font-size: 11px;
  font-weight: 600;
}

.media-controls__btn--mute {
  margin-left: auto;
}

.media-controls__volume {
  display: flex;
  align-items: center;
  padding: 0 8px 0 2px;
}

.media-controls__volume-slider {
  width: 86px;
  height: 4px;
  border-radius: 99px;
  appearance: none;
  background: linear-gradient(
    to right,
    var(--accent-primary) var(--progress),
    var(--border-default) var(--progress)
  );
  accent-color: var(--accent-primary);
  cursor: pointer;
}

.media-controls__volume-slider::-webkit-slider-runnable-track {
  height: 4px;
  border-radius: 99px;
  background: transparent;
}

.media-controls__volume-slider::-webkit-slider-thumb {
  width: 12px;
  height: 12px;
  margin-top: -4px;
  appearance: none;
  border: 2px solid var(--accent-primary);
  border-radius: 50%;
  background: var(--bg-surface);
  box-shadow: 0 0 0 3px var(--accent-overlay);
}

.media-controls__volume-slider::-moz-range-track {
  height: 4px;
  border-radius: 99px;
  background: var(--border-default);
}

.media-controls__volume-slider::-moz-range-progress {
  height: 4px;
  border-radius: 99px;
  background: var(--accent-primary);
}

.media-controls__volume-slider::-moz-range-thumb {
  width: 8px;
  height: 8px;
  border: 2px solid var(--accent-primary);
  border-radius: 50%;
  background: var(--bg-surface);
}

.media-controls__menu-wrap {
  position: relative;
  display: flex;
}

.media-controls__menu {
  /* 定位（left/top 等）由 useMenuPosition 动态计算，相对 .media-controls__menu-wrap */
  position: absolute;
  z-index: 10;
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 128px;
  padding: 6px;
  border: 1px solid var(--border-default);
  border-radius: 10px;
  background: var(--bg-surface);
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.18);
}

.media-controls__menu:focus {
  outline: none;
}

.media-controls__menu-spinner {
  display: inline-block;
  width: 10px;
  height: 10px;
  flex-shrink: 0;
  border: 1.5px solid currentColor;
  border-right-color: transparent;
  border-radius: 50%;
  animation: media-controls-menu-spin 0.7s linear infinite;
}

@keyframes media-controls-menu-spin {
  to {
    transform: rotate(360deg);
  }
}

.media-controls__menu--subtitle,
.media-controls__menu--speed {
  max-width: 320px;
  overflow-x: hidden;
  overflow-y: auto !important;
  overscroll-behavior: contain;
  line-height: 1.4;
  gap: 0;
  padding: 4px;
  border: 1px solid var(--border-strong);
  border-radius: 6px;
  box-shadow: var(--shadow-dropdown);
}

.media-controls__menu--subtitle {
  min-width: 180px;
}

.media-controls__menu--speed {
  min-width: 96px;
}

.media-controls__option-item {
  flex: 0 0 auto;
  justify-content: space-between;
  gap: 8px;
  min-height: 0;
}

.media-controls__option-label {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.media-controls__option-check {
  flex-shrink: 0;
  color: var(--accent-primary);
  font-size: 12px;
}

.media-controls__option-item--active {
  color: var(--accent-active);
}

.media-controls__menu-empty {
  flex: 0 0 auto;
  padding: 2px 8px;
  font-size: 13px;
  color: var(--text-hint);
}

.media-controls__menu-error {
  flex: 0 0 auto;
  padding: 2px 8px;
  color: var(--status-error, #c0392b);
  font-size: 12px;
  line-height: 1.4;
}

.media-controls__menu--subtitle .menu-divider {
  flex-shrink: 0;
}

.media-controls__btn--system {
  margin-left: 4px;
}

@media (max-width: 520px) {
  .media-controls__progress {
    padding-inline: 10px;
  }

  .media-controls__buttons {
    padding-inline: 8px;
  }

  .media-controls__volume {
    display: none;
  }
}
</style>
