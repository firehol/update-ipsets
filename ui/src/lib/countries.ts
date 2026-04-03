const countryDisplayNames =
  typeof Intl !== "undefined" && typeof Intl.DisplayNames === "function"
    ? new Intl.DisplayNames(["en"], { type: "region" })
    : null;

export function isISORegionCode(code: string | null | undefined): boolean {
  return /^[A-Z]{2}$/.test((code ?? "").trim().toUpperCase());
}

export function formatCountryLabel(code: string | null | undefined): string {
  const normalized = (code ?? "").trim().toUpperCase();
  if (!normalized) return "";
  if (normalized === "COUNTRYLESS") {
    return "Countryless";
  }
  if (!isISORegionCode(normalized)) {
    return normalized
      .replace(/[_-]+/g, " ")
      .toLowerCase()
      .replace(/\b\w/g, (match) => match.toUpperCase());
  }
  try {
    return countryDisplayNames?.of(normalized) ?? normalized;
  } catch {
    return normalized;
  }
}
