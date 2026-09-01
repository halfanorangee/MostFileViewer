import { computed, onBeforeUnmount, ref } from "vue";

const activeMenuId = ref("");
let nextMenuScopeId = 0;

export function useExclusiveMenu() {
    const scopeId = `menu-${++nextMenuScopeId}`;
    const toMenuId = (name) => `${scopeId}:${name}`;

    const activeMenu = computed(() => {
        const prefix = `${scopeId}:`;
        return activeMenuId.value.startsWith(prefix)
            ? activeMenuId.value.slice(prefix.length)
            : "";
    });

    function isMenuOpen(name) {
        return activeMenuId.value === toMenuId(name);
    }

    function openMenu(name) {
        activeMenuId.value = toMenuId(name);
    }

    function toggleMenu(name) {
        if (isMenuOpen(name)) {
            closeMenu();
            return;
        }
        openMenu(name);
    }

    function closeMenu() {
        if (activeMenu.value) {
            activeMenuId.value = "";
        }
    }

    onBeforeUnmount(closeMenu);

    return {
        activeMenu,
        isMenuOpen,
        openMenu,
        toggleMenu,
        closeMenu,
    };
}
