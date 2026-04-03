import { ThemeProvider as NextThemeProvider } from "next-themes";
import type { ReactNode } from "react";

const STORAGE_KEY = "firehol-theme";

/**
 * Theme provider — owns the dark/light mode toggle through next-themes.
 * It writes `light`/`dark` classes on <html> so Tailwind's `dark:`
 * variants and Sonner toasts read the same theme state.
 *
 * Defaults to "dark": the navy palette is the site's visual identity and
 * first-time visitors should see it before any OS preference intervenes.
 * Users who pick light or system explicitly get that preference honoured
 * across sessions.
 */
export function ThemeProvider({ children }: { children: ReactNode }) {
  return (
    <NextThemeProvider
      attribute="class"
      defaultTheme="dark"
      enableSystem
      storageKey={STORAGE_KEY}
    >
      {children}
    </NextThemeProvider>
  );
}
