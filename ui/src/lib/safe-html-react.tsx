import { createElement, Fragment, useMemo, type ReactNode } from "react";
import { sanitizeHtml } from "@/lib/safe-html";

const ALLOWED_TAGS = new Set([
  "a",
  "blockquote",
  "br",
  "code",
  "div",
  "em",
  "h1",
  "h2",
  "h3",
  "h4",
  "h5",
  "h6",
  "hr",
  "li",
  "ol",
  "p",
  "pre",
  "span",
  "strong",
  "table",
  "tbody",
  "td",
  "th",
  "thead",
  "tr",
  "ul",
]);

const VOID_TAGS = new Set(["br", "hr"]);
const SIMPLE_ID_RE = /^[A-Za-z][A-Za-z0-9:_.-]*$/;

export function SanitizedHtml({
  html,
  className,
}: {
  html: string;
  className?: string;
}) {
  const nodes = useMemo(() => renderSanitizedHtml(html), [html]);
  return <div className={className}>{nodes}</div>;
}

function renderSanitizedHtml(html: string): ReactNode {
  const safeHtml = sanitizeHtml(html);
  if (typeof DOMParser === "undefined") {
    return safeHtml;
  }
  const doc = new DOMParser().parseFromString(safeHtml, "text/html");
  return renderChildren(doc.body);
}

function renderChildren(parent: Node): ReactNode[] {
  return Array.from(parent.childNodes).map((child, index) =>
    renderNode(child, index),
  );
}

function renderNode(node: Node, key: number): ReactNode {
  if (node.nodeType === Node.TEXT_NODE) {
    return node.textContent;
  }
  if (node.nodeType !== Node.ELEMENT_NODE) {
    return null;
  }
  return renderElement(node as Element, key);
}

function renderElement(element: Element, key: number): ReactNode {
  const sourceTag = element.tagName.toLowerCase();
  const tag = sourceTag === "b" ? "strong" : sourceTag === "i" ? "em" : sourceTag;
  const children = renderChildren(element);
  if (!ALLOWED_TAGS.has(tag)) {
    return <Fragment key={key}>{children}</Fragment>;
  }

  const props = elementProps(element, key, tag);
  if (VOID_TAGS.has(tag)) {
    return createElement(tag, props);
  }
  return createElement(tag, props, children);
}

function elementProps(element: Element, key: number, tag: string) {
  const props: Record<string, string | number | undefined> = { key };
  const id = element.getAttribute("id");
  if (id && SIMPLE_ID_RE.test(id)) {
    props.id = id;
  }
  if (tag === "a") {
    const href = safeHref(element.getAttribute("href"));
    if (href) {
      props.href = href;
      if (isExternalHref(href)) {
        props.target = "_blank";
        props.rel = "noopener noreferrer";
      }
    }
  }
  return props;
}

function safeHref(raw: string | null): string | undefined {
  const value = raw?.trim();
  if (!value) {
    return undefined;
  }
  if (value.startsWith("#")) {
    return value;
  }
  const origin = globalThis.location?.origin ?? "http://localhost";
  try {
    const parsed = new URL(value, origin);
    if (parsed.origin === origin) {
      return `${parsed.pathname}${parsed.search}${parsed.hash}`;
    }
    if (parsed.protocol === "http:" || parsed.protocol === "https:" || parsed.protocol === "mailto:") {
      return parsed.href;
    }
  } catch {
    return undefined;
  }
  return undefined;
}

function isExternalHref(href: string): boolean {
  return href.startsWith("http://") || href.startsWith("https://");
}
