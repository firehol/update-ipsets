import { expect, test } from "vitest";
import { screen } from "@testing-library/react";
import { DataTable, type DataTableColumn } from "./data-table";
import { renderUI } from "@/test/render";

interface Row {
  name: string;
  ips: number;
}

const rows: Row[] = [
  { name: "alpha_feed", ips: 50 },
  { name: "beta_malware", ips: 100 },
];

const columns: DataTableColumn<Row>[] = [
  {
    key: "name",
    label: "Feed",
    sortValue: (row) => row.name,
    render: (row) => row.name,
  },
  {
    key: "ips",
    label: "IPs",
    align: "right",
    sortValue: (row) => row.ips,
    render: (row) => row.ips.toLocaleString(),
  },
];

test("sortable headers are keyboard reachable and expose sort state", async () => {
  const { user } = renderUI(
    <DataTable
      rows={rows}
      columns={columns}
      rowKey={(row) => row.name}
      initialSortKey="ips"
      initialSortDir="desc"
      searchPlaceholder="Filter rows"
    />,
  );

  expect(screen.getByRole("columnheader", { name: "IPs" }))
    .toHaveAttribute("aria-sort", "descending");
  expect(screen.getByRole("button", { name: "Sort by IPs ascending" }))
    .toBeVisible();

  await user.tab();
  expect(screen.getByPlaceholderText("Filter rows")).toHaveFocus();
  await user.tab();
  expect(screen.getByRole("button", { name: "Export CSV" })).toHaveFocus();
  await user.tab();

  const feedSort = screen.getByRole("button", {
    name: "Sort by Feed ascending",
  });
  expect(feedSort).toHaveFocus();
  await user.keyboard("{Enter}");

  expect(
    screen.getByRole("columnheader", { name: "Feed" }),
  ).toHaveAttribute("aria-sort", "ascending");
  expect(screen.getByRole("button", { name: "Sort by Feed descending" }))
    .toBeVisible();
});
