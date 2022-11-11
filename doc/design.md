# counterbase

This document describes the design of counterbase.

## Context

The number of sources of mobility counts is growing. Halifax has five in-street automated cycling counters, [13 automated pedestrian counters](http://www.eco-public.com:8080/ParcPublic/?id=4468), [automated transit passenger counts with weekly updates](https://catalogue-hrm.opendata.arcgis.com/datasets/a0ece3efdc7144d69cb1881b90cd93fe_0), and occasional one-off automated and manual counts.

Some effort has been put into collecting and reporting on this data, such as [the halifax-data git repo](https://github.com/danp/halifax-data/tree/main/mobility/cycling/counters) and the [bikehfxstats](https://twitter.com/bikehfxstats/) and [transithfxstats](https://twitter.com/transithfxstats) automated twitter accounts.

A number of the sources are behind proprietary, authenticated, and occasionally unreliable endpoints. They can be cumbersome to work with and it's not ideal to spread credentials around.

Some data comes via irregular manual dumps. For example, [Halifax Harbour Bridges](https://www.hdbc.ca/) has previously maintained cycling counters on the Macdonald bridge and plans to introduce a new one soon. [Data from the old counters](https://github.com/danp/halifax-data/blob/main/mobility/cycling/counters/bridge-2014.csv) was delivered via manual dump and it's likely data from the new counter will also come via monthly-ish dumps.

Consumers independently fetching data from origin sources is also limiting their functionality, or duplicating it. For example, updating the halifax-data repo and generating the bikehfxstats tweets both require fetching data from the relevant sources. But since they are not connected the data they fetch can't be used by the other. Similarly, the bikehfxstats app could be extended to track trends, records, and counts over longer periods if it had access to historical data.

As the number of sources grows and changes it's becoming harder to consistently keep track of everything. There's no centralized list of sources and how to access their data. This leads to inconsistent names across consumers. For example, the halifax-data counters directory has no high-level metadata about the counters, while the bikehfxstats app [maintains a static list of counters](https://github.com/danp/bikehfx/blob/cbb3b1dbbc8af1cd9205ad7228cee3acc59ce88f/cmd/bikehfx-tweet/main.go#L80-L97).

The frequency of counter adds, removes, and changes is expected to be low. Likely 10 or fewer per year. For transit, 2021 will be the [last year for implementation of the Moving Forward Together plan](https://www.halifaxexaminer.ca/city-hall/halifax-transits-budget-moves-forward-here-are-the-route-changes-coming-this-year/) which will mean changes to 24 routes.

The highest frequency of origin updates is currently a day with at most hourly data available in those updates.

Historical cycling counter data is available back to 2014 across various counters and time ranges. All with hourly resolution.

Historical per-route transit passenger count data is available back to 2017. All with weekly resolution.

Historical pedestrian counter data is available back to 2013 (needs verification), depending on counter. All with hourly resolution.

## Goals

* Create a central directory of counting sources.
* Make it easier to add/change/remove counters and their sources.
* Make data available in a standard, mostly reliable, source-agnostic way.
* Convert consumers to this system.
* Keep cost low.
* Use and provide open source components.

## Non-goals

* Be a general metrics / time series platform.
* Support huge datasets.
* Maintain extremely high uptime.

## Overview

At a high level, an API provides a counter directory, submitting of counter data, and querying of counter data.

Services which produce data submit to the API. The source crawler service regularly polls sources for new data and submits it to the API. The manual import source assists in submitting manually-acquired data to the API.

Consumers use the API to query the counter directory and counter data. The halifax-data repo updater queries the API for new counters and data and keeps the repo up to date. bikehfxstats queries for cycling counters and their data instead of maintaining a static counter list and querying origin sources.

The API may consist of more than one endpoint. For example, the Directory could be static data on S3 but Submit and Query could be separate "live" services.

## Details

The core pieces of the system are:

* Directory
* Submit
* Query
* Source

### Representing counters

Considerations:
* identifiers: name, some kind of id/slug, description
* modes: cycling, walking, transit, multi (eg ped + cycling counters)
* directions, flows, routes, etc: ped counter on one side counting both directions, uni- and bi-directional cycling counters, a transit route
* service ranges: when things came into or went out of service, seasonal and temporary counters, etc
  * need per direction?
* location: lon/lat, textual, civic
* kind: cycling tube, in-street or video, etc
* notes: when the counter had issues, nearby construction, etc
* frequency: how often data is likely to be updated, and maybe when
* resolution: the expected resolution of the data
* source: URL of some kind, eg `ecocounter://private/hrm/123456` or `hfxtransit:10`
  * this allows for getter per scheme
  * and perhaps a file or pipe scheme
* time zones: at least eco counter returns counter-local times. Halifax Transit counts are (as far as we know midnight Sunday to midnight Saturday Halifax time)

### API: In general

The API will be used via HTTP.

The API may not necessarily be behind a single endpoint.

### API: Directory

The directory is the central list counters and their metadata.

Consumers use the directory to discover counters, their sources, their current and past status, and what data is available for them.

The directory should have a low rate of change. The directory's origin data will be stored in a git repository and made available either directly or via generation to enable pagination, querying on several dimensions, etc.

Considerations:
* Rate of change (low)
* How many are likely? cycling counters past and present + ped counters + bus routes? * 10?
* Pagination
* Discovery of Query endpoint?

Inputs:
* Request for a list of counters, possibly with filters
* Request for a specific counter

Effects:
* None

Outputs:
* Counter(s) that satisfy requests, if any
* Pagination info

### API: Submit

As new data becomes available, it's submitted via the API. This places it into storage which makes it available for querying.

Considerations:
* Single datapoint vs bulk
* Idempotency

Inputs:
* Counter identifier
* Time period(s) and data point(s)
* Metadata (eg source, crawled or manual)

Effects:
* Stores data

Outputs:
* Success
* Failure

### API: Query

Querying data is done via the API.

Is Datasette the query API?

Considerations:
* Pagination
* Resolutions / aggregation (eg max hour/day/week/etc)
* Querying multiple counters at once
* Querying year-over-year or similar

Inputs:
* Query
* Resolution(s)
* Time range(s)

Effects:
* None

Outputs:
* Data points, by counter and time period

### Source crawler

The crawler runs periodically, probably every day, and begins by querying the directory.

The directory data informs the crawler what sources are active and should be polled.

For each active source, the crawler compares data available via Query to data from the source.

It submits any new source data to the API.

The crawler is the only part of the system that needs credentials for authenticated sources.

The crawler is the only part of the system that needs to understand how to access sources.

### Manual import source

The manual import service submits data in standard format (CSV, TSV, JSON, etc) to the API. It's used when receiving one-off or irregular dumps of data from sources which can't be polled by the crawler.

### Storage

Directory storage covered above.

Data storage will be sqlite.

The Submit and Query services can either use the same file on disk if they're co-located or [Litestream](https://litestream.io/) could be used to replicate changes from Submit to Query.

## Alternatives

### Storage: Prometheus

[Prometheus](https://prometheus.io/) was considered for data storage but rejected after research and [discussing in Gophers #performance](https://gophers.slack.com/archives/C0VP8EF3R/p1617489861291200).

It may have been nice to use its query API and PromQL for querying counter data. However, it's not really meant for long-term archival or data that's infrequently scraped.
