import { ref, computed } from "vue";
import { App } from "../../bindings/MostFileViewer";

const STORAGE_KEY = "mfv-theme";
const THEME_MODES = ["auto", "light", "dark"];

const currentTheme = ref("light");
const themeMode = ref("auto");
let initialized = false;
let mediaQuery = null;
let systemListenerActive = false;

function resolveSystemTheme() {
    return window.matchMedia("(prefers-color-scheme: dark)").matches
        ? "dark"
        : "light";
}

function getStoredMode() {
    try {
        const stored = localStorage.getItem(STORAGE_KEY) || "auto";
        return THEME_MODES.includes(stored) ? stored : "auto";
    } catch (e) {
        return "auto";
    }
}

function resolveThemeMode(mode) {
    return mode === "auto" ? resolveSystemTheme() : mode;
}

function applyTheme(theme) {
    currentTheme.value = theme;
    document.documentElement.setAttribute("data-theme", theme);
}

function applyMode(mode) {
    if (!THEME_MODES.includes(mode)) return;

    themeMode.value = mode;
    applyTheme(resolveThemeMode(mode));

    if (mode === "auto") {
        startSystemListener();
    } else {
        stopSystemListener();
    }
}

function persistTheme(mode) {
    try {
        localStorage.setItem(STORAGE_KEY, mode);
    } catch (e) {
        // Ignore when localStorage is unavailable.
    }
}

function applyRemoteTheme(mode) {
    if (!THEME_MODES.includes(mode)) return;
    applyMode(mode);
    notifyWindowBackgroundChange(currentTheme.value);
}

function onStorageChange(event) {
    if (event.key !== STORAGE_KEY || !event.newValue) return;
    applyRemoteTheme(event.newValue);
}

function onSystemThemeChange() {
    if (themeMode.value !== "auto") return;

    const nextTheme = resolveSystemTheme();
    applyTheme(nextTheme);
    notifyWindowBackgroundChange(nextTheme);
}

function startSystemListener() {
    if (systemListenerActive) return;
    systemListenerActive = true;
    mediaQuery = window.matchMedia("(prefers-color-scheme: dark)");
    mediaQuery.addEventListener("change", onSystemThemeChange);
}

function stopSystemListener() {
    if (!systemListenerActive) return;
    systemListenerActive = false;
    if (mediaQuery) {
        mediaQuery.removeEventListener("change", onSystemThemeChange);
        mediaQuery = null;
    }
}

async function notifyWindowBackgroundChange(theme) {
    try {
        await App.SetWindowBackground(theme);
    } catch (e) {
        // Frontend theme switching still works without the backend bridge.
    }
}

async function notifyBackendThemeChange(mode) {
    try {
        await App.SetWindowBackground(resolveThemeMode(mode));
        await App.BroadcastThemeChange(mode);
    } catch (e) {
        // Frontend theme switching still works without the backend bridge.
    }
}

async function syncInitialWindowBackground() {
    await notifyWindowBackgroundChange(currentTheme.value);
}

export function useTheme() {
    if (!initialized) {
        initialized = true;
        themeMode.value = getStoredMode();

        const applied = document.documentElement.getAttribute("data-theme");
        if (applied === "light" || applied === "dark") {
            currentTheme.value = applied;
        } else {
            applyTheme(resolveThemeMode(themeMode.value));
        }

        if (themeMode.value === "auto") {
            startSystemListener();
        }

        syncInitialWindowBackground();
        window.addEventListener("storage", onStorageChange);
    }

    function toggle() {
        setTheme(currentTheme.value === "dark" ? "light" : "dark");
    }

    function setTheme(mode) {
        if (!THEME_MODES.includes(mode)) return;
        applyMode(mode);
        persistTheme(mode);
        notifyBackendThemeChange(mode);
    }

    const isDark = computed(() => currentTheme.value === "dark");
    const isAuto = computed(() => themeMode.value === "auto");

    return {
        currentTheme,
        themeMode,
        isDark,
        isAuto,
        toggle,
        setTheme,
        applyRemoteTheme,
    };
}
