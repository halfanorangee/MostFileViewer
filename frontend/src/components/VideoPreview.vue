<template>
  <div
    ref="videoContainerRef"
    class="video-preview"
    :class="{
      'video-preview--fullscreen': isFullscreen,
      'video-preview--controls-visible': isFullscreen && showFullscreenControls,
      'video-preview--cursor-hidden': isFullscreen && isCursorHidden,
    }"
    tabindex="0"
    @wheel="resetFullscreenCursorTimer"
    @contextmenu.prevent
    @mousedown.capture="resetFullscreenCursorTimer"
    @mousemove="handleFullscreenMouseMove"
    @keydown="handleKeydown"
    @pointerdown="handleRootPointerDown"
  >
    <div class="video-preview__content">
      <video
        ref="mediaRef"
        class="video-preview__player"
        :src="resolvedSrc"
        :loop="player.isLooping.value"
        :playsinline="true"
        preload="metadata"
        @contextmenu.prevent
        @dblclick="handleVideoDoubleClick"
      >
        <track
          v-if="externalSubtitle"
          ref="externalTrackEl"
          kind="subtitles"
          :src="externalSubtitle.url"
        />
      </video>

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
      :show-fullscreen="true"
      :is-fullscreen="isFullscreen"
      :show-frame-step="true"
      :frame-step-enabled="frameStepEnabled"
      :show-subtitles="true"
      :subtitle-enabled="subtitleEnabled"
      :subtitle-tracks="subtitleTracks"
      :subtitle-loading-id="subtitleLoadingId"
      :subtitle-error="subtitleError"
      @toggle-play="player.togglePlay"
      @seek-by="player.seekBy"
      @scrub-commit="player.seekTo"
      @set-volume="player.setVolume"
      @toggle-mute="player.toggleMute"
      @set-playback-rate="player.setPlaybackRate"
      @toggle-loop="player.toggleLoop"
      @set-ab-marker="player.setAbMarker"
      @toggle-fullscreen="toggleFullscreen"
      @step-frame="stepFrame"
      @select-subtitle="selectSubtitleTrack"
      @pick-subtitle-file="pickSubtitleFile"
      @open-system-player="emit('media-open-system')"
    />
  </div>
</template>

<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from "vue";
import MediaControls from "./MediaControls.vue";
import MediaErrorState from "./MediaErrorState.vue";
import { App } from "../../bindings/MostFileViewer";
import {
  createMediaKeydownHandler,
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
});

const emit = defineEmits(["media-error", "media-reload", "media-open-system"]);

const videoContainerRef = ref(null);

// 防御性复核：预检不支持的扩展名不绑定远程 src（父层预检失败时 src 也为空）
const capability = computed(() => getMediaCapability("video", props.extension));
const resolvedSrc = computed(() =>
  capability.value === "unsupported" || props.media?.loadState === "failed"
    ? ""
    : props.src,
);

const player = useMediaPlayer({
  getSource: () => resolvedSrc.value,
  onError: (payload) => emit("media-error", payload),
  playbackKey: () => props.path,
});

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
const errorMessage = computed(() => props.media?.errorMessage || "");

// ---------------------------------------------------------------------------
// 字幕：外挂同名文件 + 容器内嵌轨道 + 手动选择本地文件。
// 外挂/手动/MKV 提取字幕经前端 Blob URL 提供给 <track>，不直接暴露路径；
// MP4 等容器的内嵌 WebVTT 仍由 WebView 通过 video.textTracks 暴露。
// Chromium 不会把 MKV 内嵌 SRT/ASS 暴露为 textTracks，因此 MKV/WebM 走后端解析。
// ---------------------------------------------------------------------------

const subtitleEnabled = ref(false);
const activeSubtitleId = ref(""); // "sib:<i>" | "picked" | "mkv:<n>" | "emb:<i>" | ""
const siblingSubtitles = ref([]); // [{ path, name }] 同名外挂字幕（仅元数据）
const pickedSubtitle = ref(null); // { path, name } 手动选择的字幕文件
const containerSubtitles = ref([]); // [{ number, label }] 后端从 MKV/WebM 列出的文本轨
const externalSubtitle = ref(null); // { id, name, url } 当前加载的外挂/提取字幕
const externalTrackEl = ref(null);
const embeddedTrackItems = ref([]); // [{ id, label }] 菜单渲染用（Chromium textTracks）
let embeddedTrackObjects = []; // 非响应式：原始 TextTrack 引用
const subtitleBlobUrls = new Map(); // cacheKey -> objectURL
let subtitleLoadSeq = 0; // 换源/重新加载序号，防止旧异步结果覆盖
let subtitleRequestSeq = 0; // 单次选择序号，防止快速点击时旧结果覆盖新选择
const subtitleLoadingId = ref("");
const subtitleError = ref("");
let boundTextTrackList = null;

const subtitleTracks = computed(() => {
  const items = [];
  siblingSubtitles.value.forEach((item, index) => {
    const id = `sib:${index}`;
    items.push({
      id,
      label: item.name,
      active: subtitleEnabled.value && activeSubtitleId.value === id,
    });
  });
  if (pickedSubtitle.value) {
    items.push({
      id: "picked",
      label: pickedSubtitle.value.name,
      active: subtitleEnabled.value && activeSubtitleId.value === "picked",
    });
  }
  containerSubtitles.value.forEach((item) => {
    const id = `mkv:${item.number}`;
    items.push({
      id,
      label: item.label,
      active: subtitleEnabled.value && activeSubtitleId.value === id,
    });
  });
  if (!containerSubtitles.value.length) {
    embeddedTrackItems.value.forEach((item) => {
      items.push({
        id: item.id,
        label: item.label,
        active: subtitleEnabled.value && activeSubtitleId.value === item.id,
      });
    });
  }
  return items;
});

function isExternalSubtitleId(id) {
  return id.startsWith("sib:") || id === "picked" || id.startsWith("mkv:");
}

function subtitleFileName(path) {
  const parts = String(path || "").split(/[\\/]/);
  return parts[parts.length - 1] || String(path || "");
}

function convertSrtToVtt(text) {
  // 最小转换：时间戳毫秒段的逗号改为点号并补 VTT 文件头。
  // 序号行是合法的 VTT cue identifier，无需移除。
  return `WEBVTT\n\n${text
    .replace(/^\uFEFF/, "")
    .replace(/\r/g, "")
    .replace(/(\d{2}:\d{2}:\d{2}),(\d{1,3})/g, "$1.$2")}`;
}

function formatAssTime(value) {
  const match = String(value || "").trim().match(/^(\d+):(\d{2}):(\d{2})[.:](\d{1,3})$/);
  if (!match) {
    return "";
  }
  const [, hours, minutes, seconds, fraction] = match;
  return `${hours.padStart(2, "0")}:${minutes}:${seconds}.${fraction.padEnd(3, "0")}`;
}

function splitAssDialogueFields(value, fieldCount) {
  const fields = [];
  let remaining = value;
  for (let index = 1; index < fieldCount; index += 1) {
    const separator = remaining.indexOf(",");
    if (separator < 0) {
      return [];
    }
    fields.push(remaining.slice(0, separator));
    remaining = remaining.slice(separator + 1);
  }
  fields.push(remaining);
  return fields;
}

function convertAssToVtt(text) {
  const lines = String(text || "").replace(/^\uFEFF/, "").replace(/\r/g, "").split("\n");
  let inEvents = false;
  let fields = [];
  const cues = [];

  for (const line of lines) {
    const trimmed = line.trim();
    if (/^\[events\]$/i.test(trimmed)) {
      inEvents = true;
      continue;
    }
    if (/^\[.+\]$/.test(trimmed)) {
      inEvents = false;
      continue;
    }
    if (!inEvents) {
      continue;
    }
    if (/^format\s*:/i.test(trimmed)) {
      fields = trimmed.slice(trimmed.indexOf(":") + 1).split(",").map((field) => field.trim().toLowerCase());
      continue;
    }
    if (!/^dialogue\s*:/i.test(trimmed) || !fields.length) {
      continue;
    }

    const values = splitAssDialogueFields(trimmed.slice(trimmed.indexOf(":") + 1), fields.length);
    const startIndex = fields.indexOf("start");
    const endIndex = fields.indexOf("end");
    const textIndex = fields.indexOf("text");
    const start = formatAssTime(values[startIndex]);
    const end = formatAssTime(values[endIndex]);
    const subtitleText = values[textIndex];
    if (!start || !end || !subtitleText) {
      continue;
    }
    const normalizedText = subtitleText
      .replace(/\{[^}]*\}/g, "")
      .replace(/\\[Nn]/g, "\n")
      .replace(/\\h/g, " ")
      .trim();
    if (normalizedText) {
      cues.push(`${start} --> ${end}\n${normalizedText}`);
    }
  }

  return cues.length ? `WEBVTT\n\n${cues.join("\n\n")}\n` : "";
}

async function readSubtitleText(path) {
  const content = await App.ReadFile(path);
  const raw = content?.content || "";
  if (!raw.trim()) {
    return "";
  }
  const extension = String(path).toLowerCase().match(/\.(srt|ass|ssa)$/)?.[1];
  if (extension === "srt") {
    return convertSrtToVtt(raw);
  }
  if (extension === "ass" || extension === "ssa") {
    return convertAssToVtt(raw);
  }
  return raw;
}

async function ensureExternalSubtitle(id, seq, requestSeq) {
  if (id.startsWith("mkv:")) {
    const number = Number(id.slice(4));
    const entry = containerSubtitles.value.find((item) => item.number === number);
    if (!entry) {
      return null;
    }
    let url = subtitleBlobUrls.get(id);
    if (!url) {
      const text = await App.ReadEmbeddedSubtitle(props.path, number);
      if (seq !== subtitleLoadSeq || requestSeq !== subtitleRequestSeq) {
        return null;
      }
      if (!text) {
        return null;
      }
      url = URL.createObjectURL(new Blob([text], { type: "text/vtt" }));
      subtitleBlobUrls.set(id, url);
    }
    return { id, name: entry.label, url };
  }
  const entry =
    id === "picked"
      ? pickedSubtitle.value
      : id.startsWith("sib:")
        ? siblingSubtitles.value[Number(id.slice(4))]
        : null;
  if (!entry) {
    return null;
  }
  let url = subtitleBlobUrls.get(entry.path);
  if (!url) {
    const text = await readSubtitleText(entry.path);
    if (seq !== subtitleLoadSeq || requestSeq !== subtitleRequestSeq) {
      return null;
    }
    if (!text) {
      return null;
    }
    url = URL.createObjectURL(new Blob([text], { type: "text/vtt" }));
    subtitleBlobUrls.set(entry.path, url);
  }
  return { id, name: entry.name, url };
}

function releaseSubtitleResources() {
  for (const url of subtitleBlobUrls.values()) {
    URL.revokeObjectURL(url);
  }
  subtitleBlobUrls.clear();
  externalSubtitle.value = null;
  activeSubtitleId.value = "";
  subtitleEnabled.value = false;
  pickedSubtitle.value = null;
  siblingSubtitles.value = [];
  containerSubtitles.value = [];
  embeddedTrackItems.value = [];
  embeddedTrackObjects = [];
  subtitleLoadingId.value = "";
  subtitleError.value = "";
}

function applySubtitleTrackModes() {
  const el = mediaRef.value;
  if (!el) {
    return;
  }
  const externalTrack = externalTrackEl.value?.track || null;
  const activeId = activeSubtitleId.value;
  for (const track of Array.from(el.textTracks || [])) {
    let mode = "disabled";
    if (subtitleEnabled.value) {
      if (track === externalTrack && isExternalSubtitleId(activeId)) {
        mode = "showing";
      } else if (
        embeddedTrackObjects.some(
          (item) => item.track === track && item.id === activeId,
        )
      ) {
        mode = "showing";
      }
    }
    if (track.mode !== mode) {
      track.mode = mode;
    }
  }
}

function refreshEmbeddedTracks() {
  const el = mediaRef.value;
  const externalTrack = externalTrackEl.value?.track || null;
  const items = [];
  const objects = [];
  for (const track of Array.from(el?.textTracks || [])) {
    if (track === externalTrack) {
      continue;
    }
    if (track.kind !== "subtitles" && track.kind !== "captions") {
      continue;
    }
    const id = `emb:${objects.length}`;
    objects.push({ id, track });
    const label =
      String(track.label || track.language || "").trim() ||
      `内嵌字幕 ${objects.length}`;
    items.push({ id, label });
  }
  embeddedTrackItems.value = items;
  embeddedTrackObjects = objects;
  if (
    activeSubtitleId.value.startsWith("emb:") &&
    !objects.some((item) => item.id === activeSubtitleId.value)
  ) {
    activeSubtitleId.value = "";
    applySubtitleTrackModes();
  }
}

function handleTrackListChange() {
  refreshEmbeddedTracks();
}

function unbindTextTrackList() {
  if (!boundTextTrackList) {
    return;
  }
  boundTextTrackList.removeEventListener("addtrack", handleTrackListChange);
  boundTextTrackList.removeEventListener("removetrack", handleTrackListChange);
  boundTextTrackList = null;
}

function bindTextTrackList() {
  const list = mediaRef.value?.textTracks || null;
  if (boundTextTrackList === list) {
    return;
  }
  unbindTextTrackList();
  if (!list) {
    return;
  }
  boundTextTrackList = list;
  list.addEventListener("addtrack", handleTrackListChange);
  list.addEventListener("removetrack", handleTrackListChange);
}

function disableSubtitles() {
  ++subtitleRequestSeq;
  subtitleLoadingId.value = "";
  subtitleError.value = "";
  subtitleEnabled.value = false;
  applySubtitleTrackModes();
}

async function selectSubtitleTrack(id, seq = subtitleLoadSeq) {
  if (id === "off") {
    disableSubtitles();
    return;
  }
  const requestSeq = ++subtitleRequestSeq;
  subtitleLoadingId.value = id;
  subtitleError.value = "";
  if (id.startsWith("emb:")) {
    if (seq !== subtitleLoadSeq || requestSeq !== subtitleRequestSeq) {
      return;
    }
    subtitleEnabled.value = true;
    activeSubtitleId.value = id;
    await nextTick();
    if (seq !== subtitleLoadSeq || requestSeq !== subtitleRequestSeq) {
      return;
    }
    applySubtitleTrackModes();
    subtitleLoadingId.value = "";
    return;
  }
  try {
    const loaded = await ensureExternalSubtitle(id, seq, requestSeq);
    if (!loaded || seq !== subtitleLoadSeq || requestSeq !== subtitleRequestSeq) {
      return;
    }
    externalSubtitle.value = loaded;
    subtitleEnabled.value = true;
    activeSubtitleId.value = id;
    await nextTick();
    if (seq !== subtitleLoadSeq || requestSeq !== subtitleRequestSeq) {
      return;
    }
    applySubtitleTrackModes();
  } catch (error) {
    if (seq === subtitleLoadSeq && requestSeq === subtitleRequestSeq) {
      subtitleError.value = String(error?.message || error || "字幕加载失败");
    }
  } finally {
    if (requestSeq === subtitleRequestSeq) {
      subtitleLoadingId.value = "";
    }
  }
}

async function pickSubtitleFile() {
  try {
    const path = await App.PickSubtitleFile();
    if (!path) {
      return;
    }
    const seq = ++subtitleLoadSeq;
    pickedSubtitle.value = { path, name: subtitleFileName(path) };
    await selectSubtitleTrack("picked", seq);
  } catch {
    // 选择/加载失败不阻断播放
  }
}

async function loadSubtitleSources() {
  const seq = ++subtitleLoadSeq;
  ++subtitleRequestSeq;
  releaseSubtitleResources();
  if (!props.path || !props.src) {
    return;
  }
  bindTextTrackList();
  refreshEmbeddedTracks();
  try {
    const [paths, embedded] = await Promise.all([
      App.ListSiblingSubtitles(props.path).catch(() => []),
      App.ListEmbeddedSubtitles(props.path).catch(() => []),
    ]);
    if (seq !== subtitleLoadSeq) {
      return;
    }
    siblingSubtitles.value = (paths || []).map((p) => ({
      path: p,
      name: subtitleFileName(p),
    }));
    containerSubtitles.value = (embedded || [])
      .map((item) => ({
        number: Number(item?.number),
        label: String(item?.label || "").trim() || `内嵌字幕 ${item?.number ?? ""}`,
      }))
      .filter((item) => Number.isInteger(item.number) && item.number > 0);
    if (siblingSubtitles.value.length) {
      // 保持旧行为：找到同名外挂字幕时自动启用第一条
      await selectSubtitleTrack("sib:0", seq);
    }
  } catch {
    // 字幕加载失败不阻断播放
  }
}

// ---------------------------------------------------------------------------
// 逐帧：requestVideoFrameCallback 估算帧率，失败回退 30fps（近似值）
// ---------------------------------------------------------------------------

const estimatedFrameRate = ref(null);
const frameDeltas = [];
let lastFrameMediaTime = null;
let frameSamplingActive = false;

const frameRateForStepping = computed(() => estimatedFrameRate.value || 30);
const frameStepEnabled = computed(
  () =>
    player.loadState.value === "ready" &&
    !player.isPlaying.value &&
    player.duration.value > 0,
);

function resetFrameSampling() {
  frameSamplingActive = false;
  frameDeltas.length = 0;
  lastFrameMediaTime = null;
  estimatedFrameRate.value = null;
}

function sampleVideoFrame(el) {
  el.requestVideoFrameCallback((_now, metadata) => {
    if (!frameSamplingActive) {
      return;
    }
    const time = metadata.mediaTime;
    if (lastFrameMediaTime !== null) {
      const delta = time - lastFrameMediaTime;
      if (delta > 0.001 && delta < 0.5) {
        frameDeltas.push(delta);
      }
    }
    lastFrameMediaTime = time;

    if (frameDeltas.length >= 12) {
      const sorted = [...frameDeltas].sort((a, b) => a - b);
      const median = sorted[Math.floor(sorted.length / 2)];
      if (median > 0) {
        estimatedFrameRate.value = Math.round((1 / median) * 100) / 100;
      }
      frameSamplingActive = false;
      return;
    }
    sampleVideoFrame(el);
  });
}

watch(player.isPlaying, (playing) => {
  const el = mediaRef.value;
  if (
    !playing ||
    !el ||
    frameSamplingActive ||
    estimatedFrameRate.value !== null ||
    typeof el.requestVideoFrameCallback !== "function"
  ) {
    return;
  }
  frameSamplingActive = true;
  frameDeltas.length = 0;
  lastFrameMediaTime = null;
  sampleVideoFrame(el);
});

function stepFrame(direction) {
  if (!frameStepEnabled.value) {
    return;
  }
  player.seekBy(direction / frameRateForStepping.value);
}

// ---------------------------------------------------------------------------
// 快捷键与焦点
// ---------------------------------------------------------------------------

const handleKeydown = createMediaKeydownHandler({
  togglePlay: player.togglePlay,
  seekBy: player.seekBy,
  adjustVolume: player.adjustVolume,
  toggleMute: player.toggleMute,
  cyclePlaybackRate: player.cyclePlaybackRate,
  toggleLoop: player.toggleLoop,
  setAbMarker: player.setAbMarker,
  toggleFullscreen: toggleFullscreen,
  stepFrame: stepFrame,
});

function handleRootPointerDown(event) {
  if (event.target.closest("button, input, select, a, textarea")) {
    return;
  }
  videoContainerRef.value?.focus();
}

function handleVideoDoubleClick() {
  videoContainerRef.value?.focus();
  player.togglePlay();
}

// ---------------------------------------------------------------------------
// 全屏（沿用容器全屏 + 指针隐藏逻辑）
// ---------------------------------------------------------------------------

const isFullscreen = ref(false);
const showFullscreenControls = ref(false);
const isCursorHidden = ref(false);
const fullscreenControlsZoneHeight = 88;
const fullscreenCursorHideDelay = 5000;
let fullscreenCursorTimer = null;

function syncFullscreenState() {
  const videoContainer = videoContainerRef.value;
  const nextIsFullscreen = Boolean(
    videoContainer &&
      (document.fullscreenElement === videoContainer ||
        document.webkitFullscreenElement === videoContainer),
  );
  isFullscreen.value = nextIsFullscreen;
  showFullscreenControls.value = nextIsFullscreen;
  if (nextIsFullscreen) {
    resetFullscreenCursorTimer();
  } else {
    clearFullscreenCursorTimer();
    isCursorHidden.value = false;
  }
}

function handleFullscreenMouseMove(event) {
  if (!isFullscreen.value) return;
  resetFullscreenCursorTimer();
  showFullscreenControls.value =
    event.clientY >= window.innerHeight - fullscreenControlsZoneHeight;
}

function clearFullscreenCursorTimer() {
  if (fullscreenCursorTimer !== null) {
    window.clearTimeout(fullscreenCursorTimer);
    fullscreenCursorTimer = null;
  }
}

function resetFullscreenCursorTimer() {
  if (!isFullscreen.value) return;

  isCursorHidden.value = false;
  clearFullscreenCursorTimer();
  fullscreenCursorTimer = window.setTimeout(() => {
    isCursorHidden.value = true;
    fullscreenCursorTimer = null;
  }, fullscreenCursorHideDelay);
}

async function toggleFullscreen() {
  const videoContainer = videoContainerRef.value;
  if (!videoContainer) return;

  if (isFullscreen.value) {
    try {
      if (document.exitFullscreen) {
        await document.exitFullscreen();
      } else if (document.webkitExitFullscreen) {
        document.webkitExitFullscreen();
      }
      isFullscreen.value = false;
      showFullscreenControls.value = false;
      clearFullscreenCursorTimer();
      isCursorHidden.value = false;
    } catch {
      syncFullscreenState();
    }
    return;
  }

  try {
    if (videoContainer.requestFullscreen) {
      await videoContainer.requestFullscreen()
    } else if (videoContainer.webkitRequestFullscreen) {
      videoContainer.webkitRequestFullscreen()
    } else {
      return;
    }
    // Some embedded WebViews do not expose a reliable fullscreenchange event.
    isFullscreen.value = true;
    showFullscreenControls.value = true;
    resetFullscreenCursorTimer();
  } catch {
    isFullscreen.value = false;
    showFullscreenControls.value = false;
    clearFullscreenCursorTimer();
    isCursorHidden.value = false;
  }
}

// ---------------------------------------------------------------------------
// 生命周期
// ---------------------------------------------------------------------------

// 换源时重新收集字幕源并复位帧率估算
watch(
  () => props.src,
  () => {
    resetFrameSampling();
    void loadSubtitleSources();
  },
);

watch(player.loadState, (state) => {
  if (state === "ready" || state === "buffering") {
    bindTextTrackList();
    refreshEmbeddedTracks();
  }
});

onMounted(() => {
  document.addEventListener("fullscreenchange", syncFullscreenState);
  document.addEventListener("webkitfullscreenchange", syncFullscreenState);
  bindTextTrackList();
  syncFullscreenState();
  videoContainerRef.value?.focus();
  void loadSubtitleSources();
});

onBeforeUnmount(() => {
  document.removeEventListener("fullscreenchange", syncFullscreenState);
  document.removeEventListener("webkitfullscreenchange", syncFullscreenState);
  unbindTextTrackList();
  clearFullscreenCursorTimer();
  frameSamplingActive = false;
  releaseSubtitleResources();
});
</script>

<style scoped>
.video-preview {
  position: relative;
  display: flex;
  flex: 1;
  min-width: 0;
  min-height: 0;
  height: 100%;
  flex-direction: column;
  background: #000;
  color: var(--text-primary);
  overflow: hidden;
}

.video-preview:focus {
  outline: none;
}

.video-preview__content {
  position: relative;
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 0;
  background: #000;
}

.video-preview__player {
  width: 100%;
  height: 100%;
  min-width: 0;
  max-width: none;
  object-fit: contain;
}

.video-preview--fullscreen {
  width: 100vw;
  height: 100vh;
  flex: none;
}

.video-preview--cursor-hidden {
  cursor: none;
}

.video-preview--fullscreen .video-preview__content {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
}

.video-preview--fullscreen :deep(.media-controls__progress),
.video-preview--fullscreen :deep(.media-controls__buttons) {
  position: absolute;
  right: 0;
  left: 0;
  z-index: 1;
  background: color-mix(in srgb, var(--bg-surface) 88%, transparent);
  backdrop-filter: blur(6px);
  opacity: 0;
  visibility: hidden;
  pointer-events: none;
  transition: opacity 0.15s ease, visibility 0.15s ease;
}

.video-preview--fullscreen.video-preview--controls-visible
  :deep(.media-controls__progress),
.video-preview--fullscreen.video-preview--controls-visible
  :deep(.media-controls__buttons) {
  opacity: 1;
  visibility: visible;
  pointer-events: auto;
}

.video-preview--fullscreen :deep(.media-controls__progress) {
  bottom: 40px;
  border-top-color: transparent;
}

.video-preview--fullscreen :deep(.media-controls__buttons) {
  bottom: 4px;
}
</style>
