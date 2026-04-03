import { useTheme } from "next-themes";
import { Moon, Sun } from "lucide-react";
import { Button } from "@/components/ui/button";
import { HoverTip } from "@/components/editorial/hover-tip";

/**
 * Compact light/dark toggle for the header. Uses the resolved theme so the
 * icon always reflects what the user currently sees, not the persisted
 * preference (which might be "system").
 */
export function ThemeToggle() {
  const { resolvedTheme, setTheme } = useTheme();
  const current = resolvedTheme === "light" ? "light" : "dark";
  const next = current === "dark" ? "light" : "dark";
  return (
    <HoverTip text={`Switch to ${next} mode`}>
      <Button
        variant="ghost"
        size="icon"
        onClick={() => setTheme(next)}
        aria-label={`Switch to ${next} mode`}
      >
        {current === "dark" ? (
          <Sun className="h-4 w-4" />
        ) : (
          <Moon className="h-4 w-4" />
        )}
        <span className="sr-only">Toggle theme</span>
      </Button>
    </HoverTip>
  );
}
