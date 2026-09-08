import { useEffect, useState } from "react";

// Delays reflecting a fast-changing value (a search input) until it's
// stopped changing for `delayMs` - keeps the Command Palette from firing a
// real API request on every keystroke.
export function useDebouncedValue<T>(value: T, delayMs: number): T {
  const [debounced, setDebounced] = useState(value);

  useEffect(() => {
    const timer = setTimeout(() => setDebounced(value), delayMs);
    return () => clearTimeout(timer);
  }, [value, delayMs]);

  return debounced;
}
