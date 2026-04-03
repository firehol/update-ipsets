import { describe, expect, test } from "vitest";
import { externalUrlLabel, safeExternalUrl } from "./safe-url";

describe("safeExternalUrl", () => {
  test("allows HTTP and HTTPS URLs", () => {
    expect(safeExternalUrl("https://example.test/path")).toBe(
      "https://example.test/path",
    );
    expect(safeExternalUrl("http://example.test/path")).toBe(
      "http://example.test/path",
    );
  });

  test("rejects non-web schemes and malformed URLs", () => {
    expect(safeExternalUrl("artifact://feed/source")).toBeUndefined();
    expect(safeExternalUrl("javascript:alert(1)")).toBeUndefined();
    expect(safeExternalUrl("/relative/path")).toBeUndefined();
    expect(safeExternalUrl("not a url")).toBeUndefined();
  });
});

describe("externalUrlLabel", () => {
  test("formats safe URLs by host and preserves unsafe text as text", () => {
    expect(externalUrlLabel("https://www.example.test/path")).toBe(
      "example.test",
    );
    expect(externalUrlLabel("artifact://feed/source")).toBe(
      "artifact://feed/source",
    );
  });
});
