import { onBeforeUnmount, ref, watch } from 'vue'
import { getMediaType, MEDIA_MIME_BY_EXTENSION } from './useFileTypes'
import {
  claimExclusivePlayback,
  registerPlaybackController,
  unregisterPlaybackController,
} from './useMediaPlaybackCoordinator'

// 通用倍速档位；[ / ] 快捷键在此列表内移动。
export const MEDIA_PLAYBACK_RATES = [0.5, 0.75, 1, 1.25, 1.5, 2]

// canPlayType 只用于预检，真实可播放性仍以加载结果为准；无法映射的扩展名
// 按 maybe 处理，避免错误否定。MKV 特别处理如下：
// Chromium 的 canPlayType 对 video/x-matroska 恒返回空串，但内核实际能解大部分
// MKV（H.264/VP9 等）。为了支持 MKV 内嵌字幕检测，mkv 恒按 maybe 处理，
// 由真实 error 事件兜底（无法解码时走系统播放器错误面板）。
const MEDIA_MAYBE_EXTENSIONS = new Set(['.mkv'])

const mediaProbeElements = { audio: null, video: null }

/**
 * 无副作用的可播放性预检。返回 'probably' | 'maybe' | 'unsupported'。
 * 扩展名无法形成 MIME 时返回 'maybe'（按可加载处理，由真实 error 兜底）。
 */
export function getMediaCapability(previewType, extension) {
  const ext = String(extension || '').toLowerCase()
  const mime = MEDIA_MIME_BY_EXTENSION[ext]
  if (!mime || getMediaType(ext) !== previewType) {
    return 'maybe'
  }
  if (MEDIA_MAYBE_EXTENSIONS.has(ext)) {
    return 'maybe'
  }
  if (typeof document === 'undefined') {
    return 'maybe'
  }
  const kind = previewType === 'video' ? 'video' : 'audio'
  if (!mediaProbeElements[kind]) {
    mediaProbeElements[kind] = document.createElement(kind)
  }
  const verdict = mediaProbeElements[kind].canPlayType(mime)
  if (verdict === 'probably') return 'probably'
  if (verdict === 'maybe') return 'maybe'
  return 'unsupported'
}

const MEDIA_ERROR_MESSAGES = {
  unsupported: '当前 WebView 不支持该格式或编码',
  aborted: '加载被中断',
  network: '本地媒体服务或文件读取失败',
  decode: '文件损坏或当前解码器不支持',
  'source-not-supported': '当前来源或格式不受支持',
  'file-changed': '文件可能已变更',
  unknown: '无法播放',
}

export function mediaErrorMessage(code) {
  return MEDIA_ERROR_MESSAGES[code] || MEDIA_ERROR_MESSAGES.unknown
}

/**
 * 把 HTMLMediaElement.error（MediaError）、play() 拒绝的 DOMException
 * 或普通异常标准化为 {code, message}。code 取值见 MEDIA_ERROR_MESSAGES。
 */
export function normalizeMediaErrorPayload(error) {
  if (error && typeof error === 'object') {
    // MediaError 没有 name 属性，code 为 1-4；DOMException 有 name。
    if (typeof error.code === 'number' && error.code >= 1 && error.code <= 4 && !error.name) {
      const code = ['aborted', 'network', 'decode', 'source-not-supported'][error.code - 1]
      return { code, message: mediaErrorMessage(code) }
    }
    if (error.name === 'NotAllowedError') {
      return { code: 'unknown', message: '浏览器阻止了播放，请点击播放按钮重试' }
    }
    if (error.name === 'AbortError') {
      return { code: 'aborted', message: mediaErrorMessage('aborted') }
    }
    if (error.message) {
      return { code: 'unknown', message: String(error.message) }
    }
  }
  if (error) {
    return { code: 'unknown', message: String(error) }
  }
  return { code: 'unknown', message: mediaErrorMessage('unknown') }
}

function isEditableEventTarget(target) {
  const el = target
  if (!el) return false
  if (el.isContentEditable) return true
  const tag = String(el.tagName || '').toLowerCase()
  return tag === 'input' || tag === 'textarea' || tag === 'select'
}

/**
 * 创建媒体快捷键处理器。actions 中缺失的回调自动跳过（如音频无全屏）。
 * Ctrl/Meta/Alt 组合与输入控件内的按键不拦截，保留给浏览器与系统。
 */
export function createMediaKeydownHandler(actions = {}) {
  const noop = () => {}
  const run = (fn, ...args) => (fn || noop)(...args)

  return function handleMediaKeydown(event) {
    if (event.ctrlKey || event.metaKey || event.altKey) {
      return
    }
    if (isEditableEventTarget(event.target)) {
      return
    }

    const seekStep = event.shiftKey ? 30 : 10
    switch (event.key) {
      case ' ':
      case 'k':
      case 'K':
        event.preventDefault()
        run(actions.togglePlay)
        return
      case 'ArrowLeft':
        event.preventDefault()
        run(actions.seekBy, -seekStep)
        return
      case 'ArrowRight':
        event.preventDefault()
        run(actions.seekBy, seekStep)
        return
      case 'ArrowUp':
        event.preventDefault()
        run(actions.adjustVolume, 0.05)
        return
      case 'ArrowDown':
        event.preventDefault()
        run(actions.adjustVolume, -0.05)
        return
      case 'm':
      case 'M':
        run(actions.toggleMute)
        return
      case '[':
        run(actions.cyclePlaybackRate, -1)
        return
      case ']':
        run(actions.cyclePlaybackRate, 1)
        return
      case 'l':
      case 'L':
        run(actions.toggleLoop)
        return
      case 'a':
      case 'A':
        run(actions.setAbMarker)
        return
      case 'f':
      case 'F':
        run(actions.toggleFullscreen)
        return
      case ',':
        run(actions.stepFrame, -1)
        return
      case '.':
        run(actions.stepFrame, 1)
        return
      case 'n':
      case 'N':
        run(actions.nextTrack)
        return
      case 'p':
      case 'P':
        run(actions.prevTrack)
        return
      default:
        return
    }
  }
}

/**
 * 媒体播放状态机与命令集。HTMLMediaElement 事件是唯一事实来源：
 * 点击处理函数只发出命令，isPlaying/loadState 均由原生事件回写。
 *
 * @param {Object} options
 * @param {() => string} options.getSource  返回当前应加载的 URL；'' 表示不绑定
 * @param {(payload: {code: string, message: string}) => void} [options.onError]
 * @param {() => void} [options.onEnded]
 */
export function useMediaPlayer({ getSource, onError, onEnded, playbackKey } = {}) {
  const mediaRef = ref(null)
  const loadState = ref('idle')
  const isPlaying = ref(false)
  const currentTime = ref(0)
  const duration = ref(0)
  const volume = ref(1)
  const muted = ref(false)
  const playbackRate = ref(1)
  const isLooping = ref(false)
  const abLoop = ref({ start: null, end: null })
  const abHint = ref('')

  let volumeBeforeMute = 1
  let boundElement = null
  let abHintTimer = null

  // 独占播放：分屏后多个 pane 的播放器会同时挂载，播放开始时抢占独占权，
  // 由协调器暂停上一个播放器（见 useMediaPlaybackCoordinator 的时序说明）。
  const playbackKeyValue =
    typeof playbackKey === 'function'
      ? String(playbackKey() || '')
      : String(playbackKey || '')
  const playbackController = {
    pause() {
      const el = mediaRef.value
      if (el && !el.paused) {
        el.pause()
      }
    },
  }
  if (playbackKeyValue) {
    registerPlaybackController(playbackKeyValue, playbackController)
  }

  function emitError(payload) {
    loadState.value = 'failed'
    isPlaying.value = false
    if (onError) {
      onError(payload)
    }
  }

  function setCommandError(error) {
    emitError(normalizeMediaErrorPayload(error))
  }

  function showAbHint(text) {
    abHint.value = text
    if (abHintTimer) {
      clearTimeout(abHintTimer)
    }
    abHintTimer = setTimeout(() => {
      abHint.value = ''
      abHintTimer = null
    }, 2000)
  }

  function applyVolumeState(el) {
    el.volume = volume.value
    el.muted = muted.value
  }

  const eventHandlers = {
    loadstart() {
      if (loadState.value !== 'failed') {
        loadState.value = 'loading'
      }
    },
    loadedmetadata(el) {
      duration.value = Number.isFinite(el.duration) ? el.duration : 0
      loadState.value = 'ready'
    },
    durationchange(el) {
      if (Number.isFinite(el.duration)) {
        duration.value = el.duration
      }
    },
    canplay() {
      if (loadState.value !== 'failed') {
        loadState.value = 'ready'
      }
    },
    playing() {
      loadState.value = 'ready'
      isPlaying.value = true
      if (playbackKeyValue) {
        claimExclusivePlayback(playbackKeyValue)
      }
    },
    waiting() {
      if (loadState.value !== 'failed') {
        loadState.value = 'buffering'
      }
    },
    stalled() {
      if (loadState.value !== 'failed') {
        loadState.value = 'buffering'
      }
    },
    timeupdate(el) {
      currentTime.value = el.currentTime
      const { start, end } = abLoop.value
      if (end !== null && el.currentTime >= end && !el.paused) {
        el.currentTime = start ?? 0
        currentTime.value = el.currentTime
      }
    },
    pause() {
      isPlaying.value = false
      if (loadState.value !== 'failed') {
        loadState.value = 'ready'
      }
    },
    ended(el) {
      isPlaying.value = false
      if (loadState.value !== 'failed') {
        loadState.value = 'ready'
      }
      if (!isLooping.value && abLoop.value.end === null) {
        currentTime.value = 0
        el.currentTime = 0
      }
      if (onEnded) {
        onEnded()
      }
    },
    error(el) {
      emitError(normalizeMediaErrorPayload(el.error))
    },
  }

  let boundHandlers = null

  function attachListeners(el) {
    detachListeners()
    boundElement = el
    boundHandlers = {
      loadstart: () => eventHandlers.loadstart(),
      loadedmetadata: () => eventHandlers.loadedmetadata(el),
      durationchange: () => eventHandlers.durationchange(el),
      canplay: () => eventHandlers.canplay(),
      playing: () => eventHandlers.playing(),
      waiting: () => eventHandlers.waiting(),
      stalled: () => eventHandlers.stalled(),
      timeupdate: () => eventHandlers.timeupdate(el),
      pause: () => eventHandlers.pause(),
      ended: () => eventHandlers.ended(el),
      error: () => eventHandlers.error(el),
    }
    for (const [type, handler] of Object.entries(boundHandlers)) {
      el.addEventListener(type, handler)
    }

    // 换源/重挂载后恢复用户当前的音量、倍速与循环设置
    applyVolumeState(el)
    el.playbackRate = playbackRate.value
    el.loop = isLooping.value
  }

  function detachListeners() {
    const el = boundElement
    const handlers = boundHandlers
    boundElement = null
    boundHandlers = null
    if (!el || !handlers) {
      return
    }
    for (const [type, handler] of Object.entries(handlers)) {
      el.removeEventListener(type, handler)
    }
  }

  watch(mediaRef, (el) => {
    if (el) {
      attachListeners(el)
    } else {
      detachListeners()
    }
  })

  function resetStateForNewSource() {
    loadState.value = 'idle'
    isPlaying.value = false
    currentTime.value = 0
    duration.value = 0
    abLoop.value = { start: null, end: null }
    abHint.value = ''
  }

  // flush:'pre' 确保在 Vue 把新 src 写进 DOM 前先停掉当前加载，
  // 释放旧连接并复位状态。
  watch(
    () => (getSource ? getSource() : ''),
    (nextSource, previousSource) => {
      if (nextSource === previousSource) {
        return
      }
      const el = mediaRef.value
      if (el && previousSource) {
        el.pause()
        el.removeAttribute('src')
        el.load()
      }
      resetStateForNewSource()
    },
    { flush: 'pre' },
  )

  async function togglePlay() {
    const el = mediaRef.value
    if (!el) {
      return
    }
    if (el.paused) {
      try {
        await el.play()
      } catch (error) {
        // play() 被拒绝时元素仍处于 paused，事件流不会纠正 isPlaying，
        // 需要在这里显式进入统一错误路径。
        if (el.error) {
          emitError(normalizeMediaErrorPayload(el.error))
        } else {
          setCommandError(error)
        }
      }
    } else {
      el.pause()
    }
  }

  function seekTo(time) {
    const el = mediaRef.value
    if (!el) {
      return
    }
    let target = Number(time)
    if (!Number.isFinite(target)) {
      return
    }
    target = Math.max(target, 0)
    if (Number.isFinite(el.duration) && el.duration > 0) {
      target = Math.min(target, el.duration)
    }
    el.currentTime = target
    currentTime.value = target
  }

  function seekBy(delta) {
    const el = mediaRef.value
    if (!el || !Number.isFinite(el.duration) || el.duration <= 0) {
      return
    }
    seekTo(el.currentTime + delta)
  }

  function setVolume(value) {
    const next = Math.min(Math.max(Number(value) || 0, 0), 1)
    if (next === 0 && volume.value > 0) {
      volumeBeforeMute = volume.value
    }
    volume.value = next
    muted.value = next === 0
    if (boundElement) {
      applyVolumeState(boundElement)
    }
  }

  function adjustVolume(delta) {
    setVolume(volume.value + delta)
  }

  function toggleMute() {
    if (muted.value || volume.value === 0) {
      volume.value = volumeBeforeMute > 0 ? volumeBeforeMute : 1
      muted.value = false
    } else {
      volumeBeforeMute = volume.value
      muted.value = true
    }
    if (boundElement) {
      applyVolumeState(boundElement)
    }
  }

  function setPlaybackRate(rate) {
    const next = MEDIA_PLAYBACK_RATES.includes(rate) ? rate : 1
    playbackRate.value = next
    if (boundElement) {
      boundElement.playbackRate = next
    }
  }

  function cyclePlaybackRate(direction) {
    const fallbackIndex = MEDIA_PLAYBACK_RATES.indexOf(1)
    let index = MEDIA_PLAYBACK_RATES.indexOf(playbackRate.value)
    if (index === -1) {
      index = fallbackIndex === -1 ? 0 : fallbackIndex
    }
    const next = Math.min(Math.max(index + direction, 0), MEDIA_PLAYBACK_RATES.length - 1)
    setPlaybackRate(MEDIA_PLAYBACK_RATES[next])
  }

  function toggleLoop() {
    isLooping.value = !isLooping.value
    if (isLooping.value) {
      // 原生循环与 A-B 互斥
      abLoop.value = { start: null, end: null }
    }
    if (boundElement) {
      boundElement.loop = isLooping.value
    }
  }

  function clearAbLoop() {
    abLoop.value = { start: null, end: null }
  }

  function setAbMarker() {
    const el = mediaRef.value
    if (!el || !Number.isFinite(el.duration) || el.duration <= 0) {
      return
    }
    const t = el.currentTime
    if (abLoop.value.start === null) {
      abLoop.value = { start: t, end: null }
      showAbHint(`A 点已设置：${formatMediaTime(t)}`)
      return
    }
    if (abLoop.value.end === null) {
      if (t <= abLoop.value.start + 0.1) {
        showAbHint('B 点需晚于 A 点 0.1 秒以上')
        return
      }
      abLoop.value = { start: abLoop.value.start, end: t }
      if (isLooping.value) {
        isLooping.value = false
        if (boundElement) {
          boundElement.loop = false
        }
      }
      showAbHint(`A-B 循环：${formatMediaTime(abLoop.value.start)} → ${formatMediaTime(t)}`)
      return
    }
    clearAbLoop()
    showAbHint('已清除 A-B 循环')
  }

  function stopMedia() {
    const el = mediaRef.value
    if (!el) {
      return
    }
    el.pause()
    el.removeAttribute('src')
    el.load()
  }

  onBeforeUnmount(() => {
    if (abHintTimer) {
      clearTimeout(abHintTimer)
      abHintTimer = null
    }
    stopMedia()
    detachListeners()
    if (playbackKeyValue) {
      unregisterPlaybackController(playbackKeyValue, playbackController)
    }
  })

  return {
    mediaRef,
    loadState,
    isPlaying,
    currentTime,
    duration,
    volume,
    muted,
    playbackRate,
    isLooping,
    abLoop,
    abHint,
    togglePlay,
    seekTo,
    seekBy,
    setVolume,
    adjustVolume,
    toggleMute,
    setPlaybackRate,
    cyclePlaybackRate,
    toggleLoop,
    setAbMarker,
    stopMedia,
  }
}

export function formatMediaTime(seconds) {
  const total = Number(seconds) || 0
  if (!Number.isFinite(total) || total < 0) {
    return '0:00'
  }
  const hour = Math.floor(total / 3600)
  const min = Math.floor((total % 3600) / 60)
  const sec = Math.floor(total % 60)
  const secText = sec.toString().padStart(2, '0')
  if (hour > 0) {
    return `${hour}:${min.toString().padStart(2, '0')}:${secText}`
  }
  return `${min}:${secText}`
}
