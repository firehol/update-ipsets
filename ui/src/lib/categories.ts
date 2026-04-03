import { useMemo } from "react";
import { useQuery } from "@tanstack/react-query";
import type { CategoryMeta } from "@/lib/api-types";
import { categoriesOptions } from "@/lib/queries/catalog";

export function useCategoriesQuery() {
  return useQuery(categoriesOptions());
}

export function useCategoryMap(): Map<string, CategoryMeta> {
  const query = useCategoriesQuery();
  return useMemo(() => {
    const out = new Map<string, CategoryMeta>();
    for (const category of query.data ?? []) {
      out.set(category.name, category);
    }
    return out;
  }, [query.data]);
}

export function useCategoryAccent(category: string | null | undefined): string | undefined {
  const map = useCategoryMap();
  if (!category) return undefined;
  const color = map.get(category)?.color;
  if (!color || !/^#[0-9a-fA-F]{6}$/.test(color)) return undefined;
  return color;
}

export function orderCategories(
  categories: CategoryMeta[],
  names: Iterable<string>,
): string[] {
  const meta = new Map<string, CategoryMeta>();
  for (const category of categories) {
    meta.set(category.name, category);
  }
  const out = Array.from(new Set(names));
  out.sort((a, b) => {
    const am = meta.get(a);
    const bm = meta.get(b);
    const ao = am?.sort_order ?? Number.MAX_SAFE_INTEGER;
    const bo = bm?.sort_order ?? Number.MAX_SAFE_INTEGER;
    if (ao !== bo) return ao - bo;
    const al = am?.label ?? a;
    const bl = bm?.label ?? b;
    return al.localeCompare(bl);
  });
  return out;
}
