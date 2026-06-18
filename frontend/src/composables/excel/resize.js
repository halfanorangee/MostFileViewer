import { nextTick } from "vue";
import {
  MIN_ZOOM,
  MAX_ZOOM,
  ZOOM_STEP,
  MIN_COLUMN_WIDTH,
  MAX_COLUMN_WIDTH,
  MIN_ROW_HEIGHT,
} from "./constants";

export function createResizeHandlers(state) {
  const {
    zoom,
    customColumnWidths,
    customRowHeights,
    sizeKeyFn,
    getColumnWidthFn,
    getRowHeightFn,
    updateViewport,
  } = state;

  let resizeState = null;

  function handleZoom(event) {
    const delta = event.deltaY < 0 ? ZOOM_STEP : -ZOOM_STEP;
    zoom.value = Number(
      Math.min(MAX_ZOOM, Math.max(MIN_ZOOM, zoom.value + delta)).toFixed(2),
    );
    nextTick(updateViewport);
  }

  function startColumnResize(index, event) {
    resizeState = {
      type: "column",
      index,
      startPosition: event.clientX,
      startSize: getColumnWidthFn(index) * zoom.value,
    };
    bindResizeEvents();
  }

  function startRowResize(index, event) {
    resizeState = {
      type: "row",
      index,
      startPosition: event.clientY,
      startSize: getRowHeightFn(index) * zoom.value,
    };
    bindResizeEvents();
  }

  function handleResizeMove(event) {
    if (!resizeState) return;

    if (event.buttons === 0) {
      stopResize();
      return;
    }

    const currentPosition =
      resizeState.type === "column" ? event.clientX : event.clientY;
    const delta = currentPosition - resizeState.startPosition;
    const minSize =
      resizeState.type === "column" ? MIN_COLUMN_WIDTH : MIN_ROW_HEIGHT;
    const rawSize = (resizeState.startSize + delta) / zoom.value;
    const nextSize = resizeState.type === "column"
      ? Math.min(MAX_COLUMN_WIDTH, Math.max(minSize, rawSize))
      : Math.max(minSize, rawSize);

    if (resizeState.type === "column") {
      customColumnWidths.value = {
        ...customColumnWidths.value,
        [sizeKeyFn(resizeState.index)]: nextSize,
      };
    } else {
      customRowHeights.value = {
        ...customRowHeights.value,
        [sizeKeyFn(resizeState.index)]: nextSize,
      };
    }
  }

  function stopResize() {
    resizeState = null;
    unbindResizeEvents();
  }

  function bindResizeEvents() {
    window.addEventListener("mousemove", handleResizeMove);
    window.addEventListener("mouseup", stopResize);
    window.addEventListener("mouseleave", stopResize);
    document.body.style.cursor =
      resizeState?.type === "column" ? "col-resize" : "row-resize";
    document.body.style.userSelect = "none";
  }

  function unbindResizeEvents() {
    window.removeEventListener("mousemove", handleResizeMove);
    window.removeEventListener("mouseup", stopResize);
    window.removeEventListener("mouseleave", stopResize);
    document.body.style.cursor = "";
    document.body.style.userSelect = "";
  }

  function resetColumnWidth(index) {
    const next = { ...customColumnWidths.value };
    delete next[sizeKeyFn(index)];
    customColumnWidths.value = next;
  }

  function resetRowHeight(index) {
    const next = { ...customRowHeights.value };
    delete next[sizeKeyFn(index)];
    customRowHeights.value = next;
  }

  function cleanup() {
    unbindResizeEvents();
  }

  return {
    handleZoom,
    startColumnResize,
    startRowResize,
    resetColumnWidth,
    resetRowHeight,
    cleanup,
  };
}
