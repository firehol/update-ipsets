# How we classify license and redistribution

iplists.firehol.org publishes many IP blocklists. For each feed we record:

- **license** — the terms under which the feed is published
- **redistributable** — whether we are permitted to republish the feed's
  contents

This page explains how we set those values.

## We assess our direct upstream only

For every feed, the catalog downloads from a specific URL. We base our
license and redistribution decisions on the terms of THAT upstream only.

Many feeds are republications, mirrors, or aggregations of other feeds.
While researching a feed, we sometimes find restrictive terms at the
**upstream of our upstream** — the source's source. We do not apply those
terms to our flags.

The relationship between our direct upstream and its own upstream is for
those two parties to manage. Our published flags describe only the
relationship that we are actually in.

## "Publicly available" defaults

When our direct upstream is "publicly available" — meaning its URL
responds to an unauthenticated HTTP GET without requiring API keys,
login, payment, or other special access — and the upstream itself states
no license and no redistribution rule, we default to:

- **redistributable: true**
- **license: "public feed"**

CDN-cached and rate-limited public URLs are still publicly available.
URLs behind login, paywall, or API token are not.

These defaults do NOT mean we ignore stated terms. If the upstream
publishes a license (Creative Commons, MIT, GPL, custom Fair Use Policy,
etc.), we honor that license as recorded. Defaults apply only when the
direct upstream is silent.

## When the upstream is not publicly available

For feeds that the catalog downloads through authenticated APIs,
paywalled endpoints, or commercial subscriptions, the upstream's specific
access agreement controls. We classify based on whatever terms that
agreement provides.

## What you'll see on each feed page

Each feed page shows:

- The license string we have recorded for that feed
- A `redistributable` flag
- An "AI Research" section that may surface license context found during
  research — including terms found at upstream-of-upstream layers, which
  we record but do **not** use to set our flags

## If you maintain a feed and disagree

If you publish a feed listed here and you believe we have classified it
incorrectly, contact us. We are happy to revisit, but the conversation is
about **your** feed's terms, not your upstream's.
