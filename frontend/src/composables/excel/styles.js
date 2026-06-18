import {
  THEME_COLORS,
  INDEXED_COLORS,
  POINT_TO_PX,
  DEFAULT_COLUMN_WIDTH,
  MIN_COLUMN_WIDTH,
  MAX_COLUMN_WIDTH,
  DEFAULT_ROW_HEIGHT,
  MIN_ROW_HEIGHT,
} from "./constants";

export function getColumnWidth(index, column, customColumnWidths, sizeKeyFn) {
  const customWidth = customColumnWidths.value[sizeKeyFn(index)];
  if (customWidth) return customWidth;

  if (!column?.width) return DEFAULT_COLUMN_WIDTH;
  return Math.min(
    MAX_COLUMN_WIDTH,
    Math.max(MIN_COLUMN_WIDTH, column.width * 8),
  );
}

export function getRowHeight(index, row, zoom, customRowHeights, sizeKeyFn) {
  const minimumHeight = getMinimumRowHeight(zoom);
  const customHeight = customRowHeights.value[sizeKeyFn(index)];
  if (customHeight) return Math.max(minimumHeight, customHeight);

  if (!row?.height) return Math.max(minimumHeight, DEFAULT_ROW_HEIGHT);
  return Math.max(minimumHeight, row.height * POINT_TO_PX);
}

export function getMinimumRowHeight(zoom) {
  return Math.max(MIN_ROW_HEIGHT, 10 + 12 / zoom);
}

export function getBaseCellStyle(column, index, zoom, getColumnWidthFn) {
  const width = getColumnWidthFn(index, column);
  return {
    width: `${width * zoom}px`,
    minWidth: `${width * zoom}px`,
    maxWidth: `${width * zoom}px`,
  };
}

export function getColumnStyle(column, index, zoom, getColumnWidthFn) {
  const width = getColumnWidthFn(index, column);
  return {
    width: `${width * zoom}px`,
    minWidth: `${width * zoom}px`,
    maxWidth: `${width * zoom}px`,
  };
}

export function getCellStyle(cell, zoom = 1) {
  const style = {};
  const styleSource = cell.style ?? {};
  applyFontStyle(style, cell.font ?? styleSource.font, zoom);
  applyFillStyle(style, cell.fill ?? styleSource.fill);
  applyAlignmentStyle(style, cell.alignment ?? styleSource.alignment);
  applyBorderStyle(style, cell.border ?? styleSource.border);
  return style;
}

export function getRichText(value, zoom) {
  if (!value || typeof value !== "object" || !value.richText) return [];
  return value.richText.map((part) => {
    const style = {};
    applyFontStyle(style, part.font ?? part.style?.font, zoom);
    return {
      text: part.text ?? "",
      style,
    };
  });
}

export function formatCellValue(value, cell) {
  if (value === null || value === undefined) return "";
  if (typeof value === "object") {
    if (value instanceof Date) {
      return value.toLocaleDateString();
    }
    if (value.richText) {
      return value.richText.map((rt) => rt.text).join("");
    }
    if (value.formula || value.result !== undefined) {
      return value.result ?? cell?.text ?? value.formula ?? "";
    }
    if (value.text) return value.text;
    if (value.hyperlink) return value.text ?? value.hyperlink;
    return JSON.stringify(value);
  }
  return cell?.text ?? String(value);
}

export function applyFontStyle(style, font, zoom) {
  if (!font) return;
  if (font.name) style.fontFamily = font.name;
  if (font.size) style.fontSize = `${font.size * POINT_TO_PX * zoom}px`;
  if (font.bold) style.fontWeight = "700";
  if (font.italic) style.fontStyle = "italic";
  const color = excelColorToCss(font.color);
  if (color) style.color = color;

  const decorations = [];
  if (font.underline) decorations.push("underline");
  if (font.strike) decorations.push("line-through");
  if (decorations.length) style.textDecoration = decorations.join(" ");
}

export function applyFillStyle(style, fill) {
  if (!fill) return;
  const color = excelColorToCss(fill.fgColor ?? fill.bgColor);
  if (fill.type === "pattern" && color) {
    style.backgroundColor = color;
  }
}

export function applyAlignmentStyle(style, alignment) {
  if (!alignment) return;
  if (alignment.horizontal) style.textAlign = alignment.horizontal;
  if (alignment.vertical) style.verticalAlign = alignment.vertical;
  if (alignment.wrapText) {
    style.whiteSpace = "pre-wrap";
    style.wordBreak = "break-word";
  }
  if (alignment.indent) style.paddingLeft = `${8 + alignment.indent * 12}px`;
}

export function applyBorderStyle(style, border) {
  if (!border) return;
  for (const side of ["top", "right", "bottom", "left"]) {
    if (!border[side]) continue;
    style[`border${capitalize(side)}`] = borderToCss(border[side]);
  }
}

function borderToCss(border) {
  const width = getBorderWidth(border.style);
  const lineStyle = getBorderLineStyle(border.style);
  const color = excelColorToCss(border.color) ?? "#1f2937";
  return `${width}px ${lineStyle} ${color}`;
}

function getBorderWidth(borderStyle) {
  if (!borderStyle || borderStyle === "none") return 0;
  if (borderStyle.includes("thick") || borderStyle === "double") return 3;
  if (borderStyle.includes("medium")) return 2;
  if (borderStyle === "hair") return 1;
  return 1;
}

function getBorderLineStyle(borderStyle) {
  if (!borderStyle || borderStyle === "none") return "solid";
  if (borderStyle === "double") return "double";
  if (borderStyle.includes("Dash") || borderStyle.includes("dash")) {
    return "dashed";
  }
  if (borderStyle.includes("Dot") || borderStyle.includes("dot")) {
    return "dotted";
  }
  return "solid";
}

export function excelColorToCss(color) {
  if (!color) return undefined;
  if (color.argb) return normalizeHexColor(color.argb);
  if (color.rgb) return normalizeHexColor(color.rgb);
  if (Number.isInteger(color.indexed)) {
    return INDEXED_COLORS[color.indexed];
  }
  if (Number.isInteger(color.theme)) {
    const themeColor = THEME_COLORS[color.theme];
    return color.tint ? applyTint(themeColor, color.tint) : themeColor;
  }
  return undefined;
}

function normalizeHexColor(value) {
  if (!value) return undefined;
  const hex = String(value).replace(/^#/, "").slice(-6);
  return hex.length === 6 ? `#${hex}` : undefined;
}

function applyTint(color, tint) {
  const rgb = hexToRgb(color);
  if (!rgb) return color;

  const apply = (channel) => {
    const next =
      tint < 0 ? channel * (1 + tint) : channel + (255 - channel) * tint;
    return Math.max(0, Math.min(255, Math.round(next)));
  };

  return rgbToHex(apply(rgb.r), apply(rgb.g), apply(rgb.b));
}

function hexToRgb(color) {
  const hex = normalizeHexColor(color)?.slice(1);
  if (!hex) return null;
  return {
    r: parseInt(hex.slice(0, 2), 16),
    g: parseInt(hex.slice(2, 4), 16),
    b: parseInt(hex.slice(4, 6), 16),
  };
}

function rgbToHex(r, g, b) {
  return `#${[r, g, b]
    .map((value) => value.toString(16).padStart(2, "0"))
    .join("")}`;
}

function capitalize(value) {
  return value.charAt(0).toUpperCase() + value.slice(1);
}
