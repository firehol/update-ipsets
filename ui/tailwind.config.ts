import type { Config } from "tailwindcss";
import typography from "@tailwindcss/typography";

const config: Config = {
  darkMode: "class",
  content: ["./index.html", "./src/**/*.{ts,tsx}"],
  theme: {
    container: {
      center: true,
      padding: "1rem",
      screens: {
        "2xl": "1280px",
      },
    },
    extend: {
      colors: {
        // shadcn primitives — every shadcn component reads these
        border: "hsl(var(--border))",
        input: "hsl(var(--input))",
        ring: "hsl(var(--ring))",
        background: "hsl(var(--background))",
        foreground: "hsl(var(--foreground))",
        primary: {
          DEFAULT: "hsl(var(--primary))",
          foreground: "hsl(var(--primary-foreground))",
        },
        secondary: {
          DEFAULT: "hsl(var(--secondary))",
          foreground: "hsl(var(--secondary-foreground))",
        },
        destructive: {
          DEFAULT: "hsl(var(--destructive))",
          foreground: "hsl(var(--destructive-foreground))",
        },
        muted: {
          DEFAULT: "hsl(var(--muted))",
          foreground: "hsl(var(--muted-foreground))",
        },
        accent: {
          DEFAULT: "hsl(var(--accent))",
          foreground: "hsl(var(--accent-foreground))",
        },
        popover: {
          DEFAULT: "hsl(var(--popover))",
          foreground: "hsl(var(--popover-foreground))",
        },
        card: {
          DEFAULT: "hsl(var(--card))",
          foreground: "hsl(var(--card-foreground))",
        },
        // FireHOL semantic tokens — used sparingly by feed-specific components.
        firehol: {
          red: "#dc2626",
          blue: "#2563eb",
          green: "#16a34a",
          amber: "#d97706",
          purple: "#7c3aed",
          cyan: "#0891b2",
        },
        // Threat-category palette — these colours are SEMANTIC, not
        // decorative. Each colour identifies the kind of threat the feed
        // tracks, so users learn the mapping. Don't use these for any
        // other purpose.
        //
        // Each category has TWO variants:
        //   - the base shade (used for chip backgrounds at low opacity,
        //     borders, and light-mode text)
        //   - the bright shade (used for dark-mode text where the base
        //     is too dark to read against the navy surface)
        //
        // Both shades share the same hue so the user still learns the
        // mapping; only the lightness differs per theme.
        category: {
          attacks: "#dc2626",
          "attacks-bright": "#f87171",
          abuse: "#ea580c",
          "abuse-bright": "#fb923c",
          malware: "#be123c",
          "malware-bright": "#fb7185",
          spam: "#ca8a04",
          "spam-bright": "#facc15",
          reputation: "#7c3aed",
          "reputation-bright": "#a78bfa",
          anonymizers: "#0891b2",
          "anonymizers-bright": "#22d3ee",
          organizations: "#2563eb",
          "organizations-bright": "#60a5fa",
          unroutable: "#64748b",
          "unroutable-bright": "#94a3b8",
          geolocation: "#0d9488",
          "geolocation-bright": "#2dd4bf",
          asn: "#0d9488",
          "asn-bright": "#2dd4bf",
          unroutable_alt: "#737373",
        },
        led: {
          ok: "#22c55e",
          warn: "#eab308",
          error: "#dc2626",
          stale: "#a3a3a3",
        },
        status: {
          healthy: "var(--status-healthy)",
          warning: "var(--status-warning)",
          delayed: "var(--status-delayed)",
          risky: "var(--status-risky)",
          archived: "var(--status-archived)",
          info: "var(--status-info)",
        },
        // Chart palette — wired to CSS variables so charts inherit the
        // active theme. Consumers should reference these via
        // `text-chart-accent`, `fill-chart-accent`, etc. instead of
        // hardcoded hex values.
        chart: {
          accent: "var(--chart-accent)",
          "accent-soft": "var(--chart-accent-soft)",
          secondary: "var(--chart-secondary)",
          context: "var(--chart-context)",
          grid: "var(--chart-grid)",
          axis: "var(--chart-axis)",
        },
      },
      backgroundColor: {
        display: "hsl(var(--display-bg))",
      },
      backgroundImage: {
        "accent-gradient": "var(--accent-gradient)",
        "accent-gradient-2stop": "var(--accent-gradient-2stop)",
      },
      textColor: {
        "display-fg": "hsl(var(--display-fg))",
        "display-muted": "hsl(var(--display-muted))",
      },
      borderColor: {
        "display-border": "hsl(var(--display-border))",
      },
      borderRadius: {
        /* Softer scale, derived from the 10px --radius base.
           Old Alpine app had sm=4 md=8 lg=12 xl=16 — we echo that here:
             sm  = --radius - 4  =  6px
             md  = --radius - 2  =  8px
             lg  = --radius      = 10px
             xl  = --radius + 4  = 14px
             2xl = --radius + 8  = 18px
           shadcn components default to "lg" which now means 10px. */
        lg: "var(--radius)",
        md: "calc(var(--radius) - 2px)",
        sm: "calc(var(--radius) - 4px)",
        xl: "calc(var(--radius) + 4px)",
        "2xl": "calc(var(--radius) + 8px)",
      },
      fontFamily: {
        sans: [
          "Inter",
          "-apple-system",
          "BlinkMacSystemFont",
          "Segoe UI",
          "Roboto",
          "sans-serif",
        ],
        display: [
          "Inter Display",
          "Inter",
          "-apple-system",
          "BlinkMacSystemFont",
          "sans-serif",
        ],
        mono: [
          "JetBrains Mono",
          "Fira Code",
          "SF Mono",
          "Consolas",
          "monospace",
        ],
      },
      keyframes: {
        "accordion-down": {
          from: { height: "0" },
          to: { height: "var(--radix-accordion-content-height)" },
        },
        "accordion-up": {
          from: { height: "var(--radix-accordion-content-height)" },
          to: { height: "0" },
        },
      },
      animation: {
        "accordion-down": "accordion-down 0.2s ease-out",
        "accordion-up": "accordion-up 0.2s ease-out",
      },
    },
  },
  plugins: [typography],
};

export default config;
