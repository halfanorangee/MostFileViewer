// CodeMirror 编辑器的视觉配置：语法高亮配色、编辑器主题样式、搜索面板中文化。
// 均为无响应式状态的静态配置，从 CodePreview 组件抽离以精简主体。
// 配色统一引用 theme.css 中的 --cm-* 变量，自动联动明暗主题。
import { EditorView } from "codemirror";
import { HighlightStyle } from "@codemirror/language";
import { tags } from "@lezer/highlight";
import { EditorState as CMState } from "@codemirror/state";

// 语法高亮配色，tag → --cm-syntax-* 变量。
export const codeHighlightStyle = HighlightStyle.define([
    {
        tag: tags.comment,
        color: "var(--cm-syntax-comment)",
        fontStyle: "italic",
    },
    {
        tag: [tags.keyword, tags.operatorKeyword, tags.modifier],
        color: "var(--cm-syntax-keyword)",
    },
    {
        tag: [tags.string, tags.character, tags.attributeValue],
        color: "var(--cm-syntax-string)",
    },
    {
        tag: [tags.number, tags.bool, tags.null, tags.atom],
        color: "var(--cm-syntax-constant)",
    },
    {
        tag: [tags.variableName, tags.propertyName, tags.attributeName],
        color: "var(--cm-syntax-variable)",
    },
    {
        tag: [
            tags.function(tags.variableName),
            tags.function(tags.propertyName),
            tags.labelName,
        ],
        color: "var(--cm-syntax-function)",
    },
    {
        tag: [tags.typeName, tags.className, tags.namespace, tags.tagName],
        color: "var(--cm-syntax-type)",
    },
    {
        tag: [tags.operator, tags.punctuation, tags.bracket],
        color: "var(--cm-syntax-operator)",
    },
    {
        tag: [tags.regexp, tags.escape, tags.special(tags.string)],
        color: "var(--cm-syntax-special)",
    },
    {
        tag: [tags.meta, tags.annotation, tags.processingInstruction],
        color: "var(--cm-syntax-meta)",
    },
    {
        tag: [tags.heading, tags.strong],
        color: "var(--cm-syntax-heading)",
        fontWeight: "600",
    },
    {
        tag: tags.link,
        color: "var(--cm-syntax-link)",
        textDecoration: "underline",
    },
    {
        tag: tags.invalid,
        color: "var(--cm-syntax-invalid)",
        textDecoration: "underline wavy var(--cm-syntax-invalid)",
    },
]);

// 搜索 / 替换面板的界面文案中文化。
export const chineseSearchPhrases = CMState.phrases.of({
    "Control character": "控制字符",
    Search: "搜索",
    Replace: "替换",
    next: "下一个",
    previous: "上一个",
    all: "全部",
    "match case": "区分大小写",
    regexp: "正则表达式",
    "by word": "全词匹配",
    replace: "替换",
    "replace all": "全部替换",
    close: "关闭",
});

// 编辑器主题样式：布局、配色、搜索面板控件样式，均引用 --cm-* 变量。
export const editorTheme = EditorView.theme({
    "&": {
        height: "100%",
        fontSize: "13px",
        backgroundColor: "var(--cm-bg)",
        color: "var(--cm-text)",
    },
    ".cm-scroller": {
        fontFamily: '"JetBrains Mono", "Fira Code", Consolas, monospace',
    },
    ".cm-content": {
        padding: "16px 0",
        position: "relative",
        zIndex: 2,
        backgroundColor: "transparent",
    },
    ".cm-gutters": {
        backgroundColor: "var(--cm-gutter-bg)",
        color: "var(--cm-gutter-text)",
        borderRight: "1px solid var(--cm-gutter-border)",
    },
    ".cm-activeLine": {
        backgroundColor: "var(--cm-active-line-bg)",
    },
    ".cm-activeLineGutter": {
        backgroundColor: "var(--cm-active-line-gutter-bg)",
    },
    ".cm-selectionLayer": {
        zIndex: "3 !important",
        pointerEvents: "none",
    },
    ".cm-selectionBackground, &.cm-focused > .cm-scroller > .cm-selectionLayer .cm-selectionBackground":
        {
            background: "var(--cm-selection-bg) !important",
        },
    ".cm-line::selection, .cm-line ::selection": {
        backgroundColor: "var(--cm-selection-line-bg)",
    },
    ".cm-cursor, .cm-dropCursor": {
        borderLeftColor: "var(--cm-cursor-color)",
    },
    ".cm-searchMatch": {
        backgroundColor: "var(--cm-search-match-bg)",
        border: "1px solid var(--cm-search-match-border)",
        borderRadius: "4px",
    },
    ".cm-searchMatch.cm-searchMatch-selected": {
        backgroundColor: "var(--cm-search-match-selected-bg)",
        borderColor: "var(--cm-search-match-selected-border)",
    },
    ".cm-selectionMatch": {
        backgroundColor: "var(--cm-selection-match-bg)",
    },
    ".cm-panels": {
        backgroundColor: "var(--cm-panels-bg)",
        color: "var(--cm-panels-text)",
        borderBottom: "1px solid var(--cm-panels-border)",
    },
    ".cm-panels-top": {
        borderTopLeftRadius: "14px",
        borderTopRightRadius: "14px",
    },
    ".cm-search": {
        padding: "10px 12px",
        gap: "8px",
        alignItems: "center",
        flexWrap: "wrap",
        fontSize: "13px",
    },
    ".cm-search > *": {
        display: "inline-flex",
        alignItems: "center",
        verticalAlign: "middle",
    },
    ".cm-search label": {
        display: "inline-flex",
        alignItems: "center",
        gap: "6px",
        color: "var(--cm-search-label-text)",
        fontSize: "13px",
        verticalAlign: "middle",
    },
    ".cm-search input": {
        fontSize: "13px",
        height: "27px",
        padding: "0 8px",
        borderRadius: "4px",
        border: "1px solid var(--cm-search-input-border)",
        backgroundColor: "var(--cm-search-input-bg)",
        color: "var(--cm-search-input-text)",
        outline: "none",
    },
    ".cm-search input[type='checkbox']": {
        appearance: "none",
        WebkitAppearance: "none",
        width: "14px",
        height: "14px",
        margin: 0,
        padding: 0,
        borderRadius: "4px",
        border: "1px solid var(--cm-search-checkbox-border)",
        backgroundColor: "var(--cm-search-checkbox-bg)",
        position: "relative",
        cursor: "pointer",
        flexShrink: 0,
    },
    ".cm-search input[type='checkbox']:hover": {
        borderColor: "var(--cm-search-checkbox-hover-border)",
        backgroundColor: "var(--cm-search-checkbox-hover-bg)",
    },
    ".cm-search input[type='checkbox']:focus": {
        borderColor: "var(--accent-primary)",
        boxShadow: "0 0 0 2px var(--cm-search-checkbox-focus-ring)",
    },
    ".cm-search input[type='checkbox']:checked": {
        backgroundColor: "var(--cm-search-checkbox-checked-bg)",
        borderColor: "var(--cm-search-checkbox-checked-border)",
    },
    ".cm-search input[type='checkbox']:checked::after": {
        content: '""',
        position: "absolute",
        left: "4px",
        top: "1px",
        width: "4px",
        height: "8px",
        border: "solid var(--cm-search-checkbox-check-color)",
        borderWidth: "0 2px 2px 0",
        transform: "rotate(45deg)",
    },
    ".cm-search input:focus": {
        borderColor: "var(--accent-primary)",
    },
    ".cm-search button": {
        fontSize: "13px",
        height: "27px",
        padding: "0 8px",
        border: "1px solid var(--cm-search-button-border)",
        borderRadius: "4px",
        background: "var(--cm-search-button-bg)",
        color: "var(--cm-search-button-text)",
        cursor: "pointer",
    },
    ".cm-search button:hover": {
        backgroundColor: "var(--cm-search-button-hover-bg)",
        borderColor: "var(--cm-search-button-hover-border)",
    },
    ".cm-search button:focus": {
        outline: "none",
        borderColor: "var(--accent-primary)",
    },
    ".cm-search button[name='close']": {
        width: "20px",
        borderRadius: "4px",
    },
    ".cm-search label:has(input[type='checkbox'])": {
        display: "inline-flex",
        alignItems: "center",
        justifyContent: "center",
        height: "27px",
        padding: "0 8px",
        color: "var(--cm-search-label-checkbox-text)",
        cursor: "pointer",
        lineHeight: 1,
    },
    ".cm-button": {
        font: "inherit",
    },
});
