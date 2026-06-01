# Methodology Endpoints

You will learn how to retrieve methodology pages that explain how metrics and insights are defined.

## What methodology pages are

Methodology pages explain the user-facing meaning of a signal and its interpretation limits. They describe what a metric measures, how to interpret it, and what it does not cover.

Methodology pages are not implementation guides. They do not contain config schemas, code paths, artifact filenames, or internal validation mechanics.

## Methodology index

```
GET /api/v1/methodology
```

Returns a list of all available methodology pages with slug, title, and summary.

Example response (simplified):

```json
{
  "items": [
    {
      "slug": "feed-health",
      "title": "Feed Health Classification",
      "summary": "How we classify feed health and what each class means."
    },
    {
      "slug": "retention",
      "title": "IP Retention Analysis",
      "summary": "How we measure how long IPs stay in a feed."
    }
  ]
}
```

## Methodology page content

```
GET /api/v1/methodology/{slug}
```

Returns the full content of a single methodology page.

Example:

```
GET /api/v1/methodology/feed-health
```

Key response fields:

- `slug` — the page identifier
- `title` — the page title
- `summary` — a short description
- `body` — rendered HTML for the page body

## How pages are used

The public website links to methodology pages from relevant UI surfaces. For example, a feed detail page links to the health methodology from the health badge, and the retention chart links to the retention methodology.

The API returns page metadata and rendered HTML so consumers can embed methodology content without scraping the public website.
