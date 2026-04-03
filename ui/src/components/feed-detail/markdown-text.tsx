import { Fragment, type ReactNode } from "react";
import { safeExternalUrl } from "@/lib/safe-url";
import { cn } from "@/lib/utils";

const LINK_RE = /\[([^\]]+)\]\(([^)]+)\)/g;
const BOLD_RE = /\*\*([^*]+)\*\*/g;

export function MarkdownText({
  value,
  className,
  dropCap = false,
  prose = "default",
}: {
  value: string | null | undefined;
  className?: string;
  dropCap?: boolean;
  prose?: "default" | "editorial";
}) {
  const blocks = splitBlocks(value);
  if (blocks.length === 0) return null;

  const bodySize = prose === "editorial" ? "text-[17px] leading-[1.75]" : "text-[15px] leading-relaxed";
  const paragraphTone = prose === "editorial" ? "text-foreground/90" : "text-muted-foreground";

  return (
    <div className={cn("space-y-4", bodySize, "text-foreground", className)}>
      {blocks.map((block, index) => {
        if (isListBlock(block)) {
          return (
            <ul key={index} className="list-disc space-y-2 pl-5 text-muted-foreground">
              {block.map((line, lineIndex) => (
                <li key={lineIndex}>{renderInline(line.replace(/^-+\s*/, ""))}</li>
              ))}
            </ul>
          );
        }
        const isFirstParagraph = index === 0;
        return (
          <p
            key={index}
            className={cn(
              paragraphTone,
              dropCap && isFirstParagraph && "editorial-dropcap",
            )}
          >
            {renderInline(block.join(" "))}
          </p>
        );
      })}
    </div>
  );
}

function splitBlocks(value: string | null | undefined): string[][] {
  const trimmed = value?.trim();
  if (!trimmed) return [];
  return trimmed
    .split(/\n\s*\n/)
    .map((block) => block.split("\n").map((line) => line.trim()).filter(Boolean))
    .filter((block) => block.length > 0);
}

function isListBlock(block: string[]): boolean {
  return block.every((line) => /^-\s+/.test(line));
}

function renderInline(value: string): ReactNode {
  const parts: ReactNode[] = [];
  let lastIndex = 0;
  for (const match of value.matchAll(LINK_RE)) {
    if (match.index > lastIndex) {
      parts.push(renderBold(value.slice(lastIndex, match.index), parts.length));
    }
    const href = safeExternalUrl(match[2]);
    parts.push(
      href ? (
        <a
          key={parts.length}
          className="text-primary underline-offset-4 hover:underline"
          href={href}
          target="_blank"
          rel="noopener noreferrer"
        >
          {renderBold(match[1], 0)}
        </a>
      ) : (
        <Fragment key={parts.length}>{match[1]}</Fragment>
      ),
    );
    lastIndex = match.index + match[0].length;
  }
  if (lastIndex < value.length) {
    parts.push(renderBold(value.slice(lastIndex), parts.length));
  }
  return parts;
}

function renderBold(value: string, keySeed: number): ReactNode {
  const parts: ReactNode[] = [];
  let lastIndex = 0;
  for (const match of value.matchAll(BOLD_RE)) {
    if (match.index > lastIndex) {
      parts.push(value.slice(lastIndex, match.index));
    }
    parts.push(
      <strong key={`${keySeed}-${parts.length}`} className="font-semibold text-foreground">
        {match[1]}
      </strong>,
    );
    lastIndex = match.index + match[0].length;
  }
  if (lastIndex < value.length) {
    parts.push(value.slice(lastIndex));
  }
  return parts.length === 1 ? parts[0] : parts;
}
