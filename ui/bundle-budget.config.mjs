const KiB = 1024;

export default {
  assetsDir: "dist/assets",
  warnAt: 0.9,
  budgets: [
    {
      name: "public shell javascript",
      patterns: ["^index-[\\w-]+\\.js$"],
      maxBytes: 520 * KiB,
      maxGzipBytes: 170 * KiB,
    },
    {
      name: "public shell css",
      patterns: ["^index-[\\w-]+\\.css$"],
      maxBytes: 115 * KiB,
      maxGzipBytes: 20 * KiB,
    },
    {
      name: "home route",
      patterns: ["^home-[\\w-]+\\.js$"],
      maxBytes: 48 * KiB,
      maxGzipBytes: 14 * KiB,
    },
    {
      name: "feed detail route visualizations",
      patterns: ["^feed-detail-[\\w-]+\\.js$"],
      maxBytes: 520 * KiB,
      maxGzipBytes: 150 * KiB,
    },
    {
      name: "admin route",
      patterns: ["^admin-[\\w-]+\\.js$", "^admin-layout-[\\w-]+\\.js$"],
      maxBytes: 170 * KiB,
      maxGzipBytes: 50 * KiB,
    },
    {
      name: "entity detail routes",
      patterns: [
        "^asn-detail-[\\w-]+\\.js$",
        "^country-detail-[\\w-]+\\.js$",
        "^maintainer-detail-[\\w-]+\\.js$",
      ],
      maxBytes: 40 * KiB,
      maxGzipBytes: 12 * KiB,
    },
  ],
};
