/**
 * 音视频互斥播放协调器（模块级单例）。
 *
 * 设计背景：单 pane 时代，音频 / 视频 tab 依靠「非激活 tab 不渲染组件」
 * 隐式保证同一时刻只有一个播放器。分屏后多个 pane 的激活 tab 会同时挂载，
 * 隐式约束失效，因此需要一个显式的协调者。
 *
 * 时序（无死循环）：
 *   1. 播放器 A 真正开始播放 → 浏览器派发 'playing' → claimExclusivePlayback(A)
 *   2. 协调器对上一个播放器 B 调用原生 el.pause()
 *   3. B 的 'pause' 事件回写它的 isPlaying=false；pause 不触发 'playing'，
 *      因此不会再次进入 claim，循环终止。
 */

/** key → { pause: () => void }。key 采用 tab 的唯一标识（path）。 */
const controllers = new Map();
/** 当前持有独占播放权的 key。 */
let activeKey = "";

export function registerPlaybackController(key, controller) {
    if (!key || !controller) return;
    controllers.set(key, controller);
}

/** 仅在注册项仍是同一实例时移除，避免重挂载时误删新实例。 */
export function unregisterPlaybackController(key, controller) {
    if (!key) return;
    if (controller && controllers.get(key) !== controller) {
        return;
    }
    controllers.delete(key);
    if (activeKey === key) {
        activeKey = "";
    }
}

/** 播放开始时调用：暂停上一个持有独占权的播放器。 */
export function claimExclusivePlayback(key) {
    if (!key || activeKey === key) {
        return;
    }
    const previousKey = activeKey;
    activeKey = key;
    const previous = previousKey ? controllers.get(previousKey) : null;
    if (previous?.pause) {
        try {
            previous.pause();
        } catch (error) {
            // 元素可能已从 DOM 移除，忽略。
        }
    }
}

/** 主动让出独占权（如播放结束、加载失败、卸载）。 */
export function releaseExclusivePlayback(key) {
    if (key && activeKey === key) {
        activeKey = "";
    }
}

/** 仅供测试 / 调试：当前持有独占权的 key。 */
export function getExclusivePlaybackKey() {
    return activeKey;
}
