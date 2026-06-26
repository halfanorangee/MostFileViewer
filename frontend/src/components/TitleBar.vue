<template>
    <header class="title-bar">
        <div
            class="title-bar__drag title-left"
            @dblclick="handleToggleMaximize"
        >
            <span class="title-bar__title">MostFileViewer</span>
            <div class="title-bar__menu" @mousedown.stop>
                <button
                    v-if="showSidebarToggle"
                    class="title-bar__btn title-bar__btn--icon"
                    :class="{ 'title-bar__btn--active': sidebarOpen }"
                    :title="sidebarOpen ? '关闭文件列表' : '打开文件列表'"
                    :aria-label="sidebarOpen ? '关闭文件列表' : '打开文件列表'"
                    @click.stop="emit('toggle-sidebar')"
                    @dblclick.stop.prevent
                >
                    <svg
                        xmlns="http://www.w3.org/2000/svg"
                        fill="none"
                        viewBox="0 0 24 24"
                        stroke-width="1.5"
                        stroke="currentColor"
                        class="title-bar__icon"
                    >
                        <path
                            stroke-linecap="round"
                            stroke-linejoin="round"
                            d="M3.75 5.25A2.25 2.25 0 0 1 6 3h12a2.25 2.25 0 0 1 2.25 2.25v13.5A2.25 2.25 0 0 1 18 21H6a2.25 2.25 0 0 1-2.25-2.25V5.25Zm5.25 0v13.5"
                        />
                    </svg>
                </button>
                <div class="title-bar__file-menu">
                    <button
                        class="title-bar__btn"
                        title="文件"
                        @click.stop="toggleMenu"
                        @dblclick.stop.prevent
                    >
                        文件
                    </button>
                    <div v-if="menuOpen" class="menu-panel dropdown-menu">
                        <button
                            class="menu-item"
                            @click.stop="handleSelectFile"
                        >
                            选择文件
                        </button>
                        <button
                            class="menu-item"
                            @click.stop="handleSelectFolder"
                        >
                            选择文件夹
                        </button>
                        <div class="menu-divider"></div>
                        <button
                            class="menu-item"
                            @click.stop="handleNewWindow"
                        >
                            新窗口
                        </button>
                    </div>
                </div>
            </div>
        </div>
        <div class="title-bar__controls">
            <div class="title-bar__theme-menu" @mousedown.stop>
                <button
                    class="title-bar__btn title-bar__btn--icon"
                    :title="themeButtonTitle"
                    :aria-label="themeButtonTitle"
                    @click.stop="handleCycleTheme"
                    @dblclick.stop.prevent
                >
                    <svg
                        v-if="isAuto"
                        xmlns="http://www.w3.org/2000/svg"
                        fill="none"
                        viewBox="0 0 24 24"
                        stroke-width="1.5"
                        stroke="currentColor"
                        class="title-bar__icon"
                    >
                        <path
                            stroke-linecap="round"
                            stroke-linejoin="round"
                            d="M9.813 15.904 9 18.75l-.813-2.846a4.5 4.5 0 0 0-3.09-3.09L2.25 12l2.846-.813a4.5 4.5 0 0 0 3.09-3.09L9 5.25l.813 2.846a4.5 4.5 0 0 0 3.09 3.09L15.75 12l-2.846.813a4.5 4.5 0 0 0-3.09 3.09ZM18.259 8.715 18 9.75l-.259-1.035a3.375 3.375 0 0 0-2.456-2.456L14.25 6l1.035-.259a3.375 3.375 0 0 0 2.456-2.456L18 2.25l.259 1.035a3.375 3.375 0 0 0 2.456 2.456L21.75 6l-1.035.259a3.375 3.375 0 0 0-2.456 2.456ZM16.894 20.567 16.5 21.75l-.394-1.183a2.25 2.25 0 0 0-1.423-1.423L13.5 18.75l1.183-.394a2.25 2.25 0 0 0 1.423-1.423l.394-1.183.394 1.183a2.25 2.25 0 0 0 1.423 1.423l1.183.394-1.183.394a2.25 2.25 0 0 0-1.423 1.423Z"
                        />
                    </svg>
                    <svg
                        v-else-if="isDark"
                        xmlns="http://www.w3.org/2000/svg"
                        fill="none"
                        viewBox="0 0 24 24"
                        stroke-width="1.5"
                        stroke="currentColor"
                        class="title-bar__icon"
                    >
                        <path
                            stroke-linecap="round"
                            stroke-linejoin="round"
                            d="M21.752 15.002A9.718 9.718 0 0 1 18 15.75c-5.385 0-9.75-4.365-9.75-9.75 0-1.33.266-2.597.748-3.752A9.753 9.753 0 0 0 3 11.25C3 16.635 7.365 21 12.75 21a9.753 9.753 0 0 0 9.002-5.998Z"
                        />
                    </svg>
                    <svg
                        v-else
                        xmlns="http://www.w3.org/2000/svg"
                        fill="none"
                        viewBox="0 0 24 24"
                        stroke-width="1.5"
                        stroke="currentColor"
                        class="title-bar__icon"
                    >
                        <path
                            stroke-linecap="round"
                            stroke-linejoin="round"
                            d="M12 3v2.25m6.364.386-1.591 1.591M21 12h-2.25m-.386 6.364-1.591-1.591M12 18.75V21m-4.773-4.227-1.591 1.591M5.25 12H3m4.227-4.773L5.636 5.636M15.75 12a3.75 3.75 0 1 1-7.5 0 3.75 3.75 0 0 1 7.5 0Z"
                        />
                    </svg>
                </button>
            </div>
            <button
                class="title-bar__btn"
                title="最小化"
                @mousedown.stop
                @click.stop="handleMinimize"
            >
                <svg
                    xmlns="http://www.w3.org/2000/svg"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke-width="1.5"
                    stroke="currentColor"
                    class="title-bar__icon"
                >
                    <path
                        stroke-linecap="round"
                        stroke-linejoin="round"
                        d="M5 12h14"
                    />
                </svg>
            </button>
            <button
                class="title-bar__btn"
                :title="isMaximized ? '还原' : '最大化'"
                @mousedown.stop
                @click.stop="handleToggleMaximize"
            >
                <svg
                    v-if="isMaximized"
                    xmlns="http://www.w3.org/2000/svg"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke-width="1.5"
                    stroke="currentColor"
                    class="title-bar__icon"
                >
                    <path
                        stroke-linecap="round"
                        stroke-linejoin="round"
                        d="M9 9V4.5M9 9H4.5M9 9 3.75 3.75M9 15v4.5M9 15H4.5M9 15l-5.25 5.25M15 9h4.5M15 9V4.5M15 9l5.25-5.25M15 15h4.5M15 15v4.5m0-4.5 5.25 5.25"
                    />
                </svg>
                <svg
                    v-else
                    xmlns="http://www.w3.org/2000/svg"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke-width="1.5"
                    stroke="currentColor"
                    class="title-bar__icon"
                >
                    <path
                        stroke-linecap="round"
                        stroke-linejoin="round"
                        d="M3.75 3.75v4.5m0-4.5h4.5m-4.5 0L9 8.25M3.75 20.25v-4.5m0 4.5h4.5m-4.5 0L9 15.75M20.25 3.75h-4.5m4.5 0v4.5m0-4.5L15 8.25M20.25 20.25h-4.5m4.5 0v-4.5m0 4.5L15 15.75"
                    />
                </svg>
            </button>
            <button
                class="title-bar__btn title-bar__btn--close"
                title="关闭"
                @mousedown.stop
                @click.stop="handleClose"
            >
                <svg
                    xmlns="http://www.w3.org/2000/svg"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke-width="1.5"
                    stroke="currentColor"
                    class="title-bar__icon"
                >
                    <path
                        stroke-linecap="round"
                        stroke-linejoin="round"
                        d="M6 18 18 6M6 6l12 12"
                    />
                </svg>
            </button>
        </div>
    </header>
</template>

<script setup>
import { computed, ref, onMounted, onBeforeUnmount } from "vue";
import { Window } from "@wailsio/runtime";
import { useTheme } from "../composables/useTheme";

defineProps({
    showSidebarToggle: {
        type: Boolean,
        default: false,
    },
    sidebarOpen: {
        type: Boolean,
        default: true,
    },
});

const emit = defineEmits([
    "select-file",
    "select-folder",
    "toggle-sidebar",
    "new-window",
]);

const isMaximized = ref(false);
const menuOpen = ref(false);

const { isDark, themeMode, setTheme } = useTheme();

const themeModes = ["auto", "light", "dark"];

const isAuto = computed(() => themeMode.value === "auto");

const themeButtonTitle = computed(() => {
    if (themeMode.value === "auto") return "主题：自动";
    return isDark.value ? "主题：深色" : "主题：浅色";
});

function handleCycleTheme() {
    const currentIndex = themeModes.indexOf(themeMode.value);
    const nextIndex = (currentIndex + 1) % themeModes.length;
    setTheme(themeModes[nextIndex]);
    menuOpen.value = false;
}

function handleMinimize() {
    try {
        Window.Minimise();
    } catch (e) {
        console.error("[TitleBar] Minimise failed:", e);
    }
}

function handleToggleMaximize() {
    try {
        Window.ToggleMaximise();
        isMaximized.value = !isMaximized.value;
    } catch (e) {
        console.error("[TitleBar] ToggleMaximise failed:", e);
    }
}

function handleClose() {
    try {
        Window.Close();
    } catch (e) {
        console.error("[TitleBar] Close failed:", e);
    }
}

function syncMaximized() {
    try {
        Window.IsMaximised()
            .then(function (v) {
                isMaximized.value = v;
            })
            .catch(function () {
                isMaximized.value =
                    window.outerWidth >= screen.availWidth &&
                    window.outerHeight >= screen.availHeight;
            });
    } catch (e) {
        isMaximized.value =
            window.outerWidth >= screen.availWidth &&
            window.outerHeight >= screen.availHeight;
    }
}

onMounted(function () {
    syncMaximized();
    window.addEventListener("resize", syncMaximized);
    document.addEventListener("click", closeMenuOnOutsideClick);
});

onBeforeUnmount(function () {
    window.removeEventListener("resize", syncMaximized);
    document.removeEventListener("click", closeMenuOnOutsideClick);
});

function toggleMenu() {
    menuOpen.value = !menuOpen.value;
}

function closeMenu() {
    menuOpen.value = false;
}

function closeMenuOnOutsideClick() {
    if (!menuOpen.value) return;
    closeMenu();
}

function handleSelectFile() {
    closeMenu();
    emit("select-file");
}

function handleSelectFolder() {
    closeMenu();
    emit("select-folder");
}

function handleNewWindow() {
    closeMenu();
    emit("new-window");
}
</script>

<style scoped>
.title-bar__drag {
    display: flex;
    align-items: center;
    gap: 15px;
}

.title-bar__menu {
    display: flex;
    align-items: center;
    height: 100%;
}

.title-bar__file-menu {
    position: relative;
    height: 100%;
}

.title-bar__btn--icon {
    width: 40px;
}

.title-bar__btn--active {
    color: var(--accent-active);
    background: var(--accent-overlay);
}

.dropdown-menu {
    position: absolute;
    top: 100%;
    left: 50%;
    transform: translateX(-50%);
    z-index: 100;
}

.dropdown-menu__icon {
    width: 16px;
    height: 16px;
    color: var(--text-muted);
    flex-shrink: 0;
}

.menu-item:hover .dropdown-menu__icon {
    color: var(--accent-active);
}
</style>
