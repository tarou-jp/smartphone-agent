import { useCallback, useState } from 'react';

/** The width of the element the returned ref is attached to, kept current. */
export function useWidth<T extends HTMLElement>(): [(el: T | null) => void, number] {
  const [width, setWidth] = useState(0);
  const ref = useCallback((el: T | null) => {
    if (!el) return;
    setWidth(el.getBoundingClientRect().width);
    const ro = new ResizeObserver(([e]) => setWidth(e.contentRect.width));
    ro.observe(el);
    // React 19 runs a ref callback's return value when the element goes.
    return () => ro.disconnect();
  }, []);
  return [ref, width];
}
