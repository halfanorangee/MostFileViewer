<template>
    <header class="title-bar">
        <div
            class="title-bar__drag title-left"
            @dblclick="handleToggleMaximize"
        >
            <span class="title-bar__title">AnyFileViewer</span>
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
                    >
                        文件
                    </button>
                    <div v-if="menuOpen" class="dropdown-menu">
                        <button
                            class="dropdown-menu__item"
                            @click.stop="handleSelectFile"
                        >
                            <svg
                                xmlns="http://www.w3.org/2000/svg"
                                fill="none"
                                viewBox="0 0 24 24"
                                stroke-width="1.5"
                                stroke="currentColor"
                                class="dropdown-menu__icon"
                            >
                                <path
                                    stroke-linecap="round"
                                    stroke-linejoin="round"
                                    d="M19.5 14.25v-2.625a3.375 3.375 0 0 0-3.375-3.375h-1.5A1.125 1.125 0 0 1 13.5 7.125v-1.5a3.375 3.375 0 0 0-3.375-3.375H8.25m2.25 0H5.625c-.621 0-1.125.504-1.125 1.125v17.25c0 .621.504 1.125 1.125 1.125h12.75c.621 0 1.125-.504 1.125-1.125V11.25a9 9 0 0 0-9-9Z"
                                />
                            </svg>
                            选择文件
                        </button>
                        <button
                            class="dropdown-menu__item"
                            @click.stop="handleSelectFolder"
                        >
                            <svg
                                xmlns="http://www.w3.org/2000/svg"
                                fill="none"
                                viewBox="0 0 24 24"
                                stroke-width="1.5"
                                stroke="currentColor"
                                class="dropdown-menu__icon"
                            >
                                <path
                                    stroke-linecap="round"
                                    stroke-linejoin="round"
                                    d="M2.25 12.75V12A2.25 2.25 0 0 1 4.5 9.75h15A2.25 2.25 0 0 1 21.75 12v.75m-8.69-6.44-2.12-2.12a1.5 1.5 0 0 0-1.061-.44H4.5A2.25 2.25 0 0 0 2.25 6v12a2.25 2.25 0 0 0 2.25 2.25h15A2.25 2.25 0 0 0 21.75 18V9a2.25 2.25 0 0 0-2.25-2.25h-5.379a1.5 1.5 0 0 1-1.06-.44Z"
                                />
                            </svg>
                            选择文件夹
                        </button>
                    </div>
                </div>
            </div>
        </div>
        <div class="title-bar__controls">
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
import { ref, onMounted, onBeforeUnmount } from "vue";
import { Window } from "@wailsio/runtime";

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

const emit = defineEmits(["select-file", "select-folder", "toggle-sidebar"]);

const isMaximized = ref(false);
const menuOpen = ref(false);

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

function closeMenuOnOutsideClick(event) {
    if (menuOpen.value) {
        menuOpen.value = false;
    }
}

function handleSelectFile() {
    closeMenu();
    emit("select-file");
}

function handleSelectFolder() {
    closeMenu();
    emit("select-folder");
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
    color: #154ec1;
    background: rgba(37, 99, 235, 0.1);
}

.dropdown-menu {
    position: absolute;
    top: 100%;
    left: 50%;
    transform: translateX(-50%);
    min-width: 150px;
    background: #ffffff;
    border: 1px solid #d1d9e6;
    border-radius: 6px;
    box-shadow: 0 8px 24px rgba(15, 23, 42, 0.12);
    padding: 4px;
    z-index: 100;
}

.dropdown-menu__item {
    display: flex;
    align-items: center;
    gap: 10px;
    width: 100%;
    padding: 8px 12px;
    border: 0;
    border-radius: 4px;
    background: transparent;
    color: #334155;
    font-size: 13px;
    cursor: pointer;
    text-align: left;
    transition: background 0.15s;
}

.dropdown-menu__item:hover {
    background: #f3f7ff;
    color: #154ec1;
}

.dropdown-menu__icon {
    width: 16px;
    height: 16px;
    color: #64748b;
    flex-shrink: 0;
}

.dropdown-menu__item:hover .dropdown-menu__icon {
    color: #154ec1;
}
</style>
