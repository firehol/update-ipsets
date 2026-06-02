import { expect, test } from "vitest";
import { screen } from "@testing-library/react";
import { SectionSpecs } from "./section-specs";
import { makeQueryClient, renderUI } from "@/test/render";
import { sampleFeedMetadata } from "@/test/fixtures";
import { queryKeys } from "@/lib/query-keys";

test("renders unsafe commit-history values as text, not links", () => {
  const client = makeQueryClient();
  client.setQueryData(queryKeys.feeds(), []);

  renderUI(
    <SectionSpecs
      feed={sampleFeedMetadata({ commit_history: "javascript:alert(1)" })}
    />,
    { client },
  );

  expect(screen.getByText("javascript:alert(1)")).toBeVisible();
  expect(
    screen.queryByRole("link", { name: /view revision history/i }),
  ).toBeNull();
});

test("links safe commit-history URLs", () => {
  const client = makeQueryClient();
  client.setQueryData(queryKeys.feeds(), []);

  renderUI(
    <SectionSpecs
      feed={sampleFeedMetadata({
        commit_history: "https://example.invalid/known_feed_changesets.json",
      })}
    />,
    { client },
  );

  expect(
    screen.getByRole("link", { name: /view revision history/i }),
  ).toHaveAttribute(
    "href",
    "https://example.invalid/known_feed_changesets.json",
  );
});
