# About update-ipsets

You will learn what update-ipsets does, what value it provides, and where its boundaries are.

## What it is

update-ipsets is a tool that downloads, normalizes, compares, and publishes public IP-based threat and blocking feeds. It turns many heterogeneous sources into one consistent, comparable collection.

## The comparative observatory

The value is not any single feed. The value is tracking many feeds over time and comparing them against each other. update-ipsets is a **comparative observatory**: it gives you factual evidence about how feeds relate, overlap, and change.

## What it does

- **Collects** live feeds and supporting datasets (ASN, geolocation, bogon references)
- **Normalizes** each feed into a canonical format
- **Preserves** historical evidence so you can reason about change over time
- **Computes** pairwise comparisons, retention analysis, and country/ASN breakdowns
- **Publishes** machine-readable artifacts, a public website, and an admin UI

## Core entities

| Entity | Purpose |
|--------|---------|
| **Feeds** | Processable inputs that produce a public IP or network set |
| **Artifact parents** | Downloadable upstream files that spawn one or more child feeds |
| **Provider databases** | Supporting datasets used to enrich feeds (ASN, geolocation, bogon lists) |
| **Published artifacts** | The outputs consumed by humans, APIs, and downstream tools |

## The public website

The website shows the results: a feed explorer, IP search, pairwise comparisons, country and ASN analysis, and historical timelines.

## The admin UI

Operators use the admin UI to monitor download and processing queues, inspect feed status, trigger rechecks and reprocessing, and run integrity checks.

## What it does not do

update-ipsets reports facts. It does not rank feeds, tell you which one is "best", or make policy decisions. It does not support IPv6 yet — IPv4 is fully implemented.
