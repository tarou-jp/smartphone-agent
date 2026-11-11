// Pinky's own window on macOS (internal/webshell) has no title bar: the
// page runs to the top edge and the traffic lights sit in the sidebar's
// first row, which holds nothing else. The page lets the window be dragged
// by its top strip. In a browser none of this applies.

interface NativeBridge {
  postMessage(message: string): void;
}

const bridge: NativeBridge | undefined =
  typeof window !== 'undefined'
    ? (window as unknown as { webkit?: { messageHandlers?: { external?: NativeBridge } } }).webkit?.messageHandlers?.external
    : undefined;

/** Whether the page runs in Pinky's title-bar-less window on macOS. */
export const inMacWindow = !!bridge && typeof navigator !== 'undefined' && navigator.platform.startsWith('Mac');

/**
 * The height of every top row: the sidebar's, the page's and the chat
 * panel's. The window's hidden title bar is the compact one, the same
 * height, so the traffic lights sit in the middle of it
 * (internal/webshell/titlebar_darwin.go).
 */
export const topRow = 38;

/** The height of the strip at the top that drags the window. */
const dragStrip = topRow;

// What keeps its own mouse: controls, text people select, and anything in
// a dialog or menu.
const keep =
  'button, a, input, textarea, select, [role="button"], [role="tab"], [role="separator"], [role="application"], [contenteditable], .selectable, .MuiModal-root, .MuiPopover-root';

/** Lets the top strip of the page drag the window, as a title bar would. */
export function installWindowDrag(): void {
  if (!inMacWindow || !bridge) return;
  window.addEventListener('mousedown', (e) => {
    if (e.button !== 0 || e.clientY > dragStrip) return;
    const el = e.target instanceof Element ? e.target : null;
    if (!el || el.closest(keep)) return;
    bridge.postMessage('drag');
  });
}
