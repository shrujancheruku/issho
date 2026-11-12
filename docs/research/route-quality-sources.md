# Research: where knowledge of a pleasant walk can come from

Issue [#538](https://github.com/shrujancheruku/issho/issues/538), part of map [#532](https://github.com/shrujancheruku/issho/issues/532). Researched 2026-11-12 against main at `9c60fee`. This branch is never merged; it feeds the ticket "Routes worth walking".

The question: Google gives the shortest walk between two stops, but a much nicer path may be a couple of minutes longer. Where can "nicer" come from, for **Da Nang and Hoi An** specifically, how good is it there, and what does it cost? Plus, briefly, the same for a **ride** between stops.

Numbers marked *measured* come from Overpass queries run on 2026-11-12 (OSM data as of 2026-11-12T07:42Z) over two boxes: **central Da Nang** (lat 15.98–16.12, lon 108.13–108.27: Hai Chau, Thanh Khe, Son Tra, Ngu Hanh Son, most of Cam Le) and **Hoi An** (lat 15.85–15.92, lon 108.28–108.36: Old Town, Cam Pho, Cam Chau, An Bang edge). The queries are reproducible from the "Method" section. Everything else carries a source URL; claims I could not confirm say **unverified**.

## Answer in brief

- **OpenStreetMap's road network is good in Da Nang; its pleasantness tags are close to empty.** Central Da Nang has 3,018 km of walkable ways (*measured*), 62% of it tagged with surface and 58% named, so the map is complete enough to route on. But only **1.8% of the length carries any `sidewalk` tag**, 4.6% carries `lit`, 0.4% `smoothness`, and practically none `width` or shade. Hoi An is thinner overall (32% surface) but slightly richer where it matters: 7.5% sidewalk, 10% lit. **No router can weigh what isn't tagged**, so the off-the-shelf "prefer sidewalks / prefer lit" knobs do almost nothing here.
- **What OSM can reliably tell us here is the shape of the network, not its comfort:** road class (a proxy for traffic: trunk and primary roads are loud and hostile), foot-only ways and living streets (2,655 foot-only ways in Da Nang, 121 living streets in Hoi An), the `kiệt` alleys (1,484 named ways), parks (134 in Da Nang, 49 in Hoi An), water edges, and crossings (6,352 crossing nodes in Da Nang, a sign of recent organised mapping). That is enough for a crude "avoid big roads, prefer water, parks, alleys and footways" score.
- **Shade, the comfort factor that matters most in a tropical city, has to come from outside OSM:** the Meta/WRI 1 m global canopy height map ([CC BY 4.0](https://registry.opendata.aws/dataforgood-fb-forests/)) or ESA WorldCover 10 m ([CC BY 4.0](https://developers.google.com/earth-engine/datasets/catalog/ESA_WorldCover_v200)), joined to each way as a "share of length under trees" score. OSM has only 937 tree nodes and 93 tree rows in all of central Da Nang.
- **Routers:** only self-hosted ones can take a custom per-way score. GraphHopper's custom models and openrouteservice's CSV storage both can; Valhalla has the richest *built-in* pedestrian knobs (walkways, sidewalks, alleys, lit, hills) but they read OSM tags that Da Nang mostly lacks; OSRM's foot profile is pure travel time. Google Routes (WALK) and Mapbox walking offer almost no preference control (`avoidIndoor`; `walkway_bias`). openrouteservice's hosted "green" and "quiet" options were built from Germany-only data, so they should not be expected to work in Vietnam.
- **Popularity data is closed to us.** Strava's heatmap may be used "for tracing into OSM only", Strava Metro is for organisations that plan infrastructure and bans AI/ML use and redistribution, and the Strava API bans showing other users' data. None is usable inside Issho.
- **Published local walks exist but are few and generic:** the Han River promenade and the Bach Dang walking street in Da Nang, the Old Town in Hoi An. OSM has **zero** walking, hiking or cycling route relations anywhere in Da Nang–Hoi An (*measured*). Hoi An's time-windowed motorbike ban on the Old Town streets, the single biggest fact about walking there, is mapped on **one** way.
- **Street-level imagery is thin and gated.** Panoramax has 7 images across the whole area (*measured*); Mapillary needs a token and its Da Nang coverage is **unverified**; Google Street View in Vietnam is recent and not licensable as data.
- **So the realistic source of "pleasant" for the Pilot is people, backed by a computed score:** a hand-curated set of good segments and routes (drawn once by the owner or trusted locals, stored as building blocks), plus an automatic score from OSM shape and canopy data for everything else, fed to a self-hosted GraphHopper (or openrouteservice) with a detour cap, and Google's route only as the baseline time.
- **For a ride**, pleasantness barely matters; time and availability do. Google Routes supports `TWO_WHEELER` in Vietnam (beta), Valhalla has a `motor_scooter` costing with a "use primary roads" knob, and riding in practice means Grab or a rented motorbike.

## 1. OpenStreetMap: what is tagged in Da Nang and Hoi An

### Measured coverage

*Walkable ways* = `highway` of trunk, primary, secondary, tertiary, unclassified, residential, living_street, service, pedestrian, footway, path, steps, track, cycleway (and links). Length is computed from way geometry. Motorways are excluded from the length figures.

| | Central Da Nang | Hoi An | Hoi An Old Town box |
|---|---|---|---|
| Walkable ways | 18,964 (3,018 km) | 3,083 (623 km) | 633 |
| Roads (trunk..living_street) | 11,233 | 2,063 | 306 |
| Roads with any `sidewalk*` tag | 225 (2.0%) | 133 (6.4%) | 32 |
| Length with `sidewalk*` | 1.8% | 7.5% | |
| Length with `surface` | 62.1% | 31.6% | 168 of 633 ways |
| Length with `lit` | 4.6% | 10.4% | 100 ways |
| Length with `smoothness` | 0.4% | 9.4% | 25 ways |
| Length with `width` | ~0% (8 ways) | 0.7% | 19 ways |
| Ways with `shade` or `covered` | 46 | 3 | 2 |
| Length with `maxspeed` | 19.1% | 4.4% | 9 ways |
| Length with `lanes` | 19.6% | 16.3% | 21 ways |
| Length with `name` | 57.9% | 30.6% | 175 ways |
| Foot-only ways (footway, pedestrian, path, steps) | 2,655 | 465 | 194 |
| `living_street` ways | 42 | 121 | 44 |
| `footway=sidewalk` (separately drawn) | 326 | 79 | 24 |
| `service=alley` | 1,326 | 65 | 34 |
| Ways named `Kiệt …` / `Hẻm …` | 1,484 | 44 | |
| `highway=crossing` nodes | 6,352 | 74 | 14 |
| Parks (`leisure=park`) | 134 | 49 | 9 |
| Tree nodes / tree rows | 937 / 93 | 387 / 0 | 103 / 0 |
| Benches / drinking water / toilets | 64 / 7 / 49 | 8 / 2 / 17 | 7 / 1 / 13 |

A denser sub-box of Hai Chau (lat 16.05–16.08, lon 108.20–108.23) gives the same picture: 1,196 roads, 43 with a sidewalk tag (3.6%), 174 ways with `lit`, 0 with `width`, but 685 `service=alley` ways and 686 crossings.

Surface values in Da Nang (ways): asphalt 6,221, concrete 2,454, paving stones 211, unpaved 192, none 9,700. A concrete alley and an asphalt arterial look the same to a router that only reads `surface`.

### What the numbers mean

- **The network is complete; its comfort attributes are not.** Way edits are recent (3,765 of 18,940 Da Nang ways were last touched in 2026, 2,485 in 2025), and Grab's mapping team (`GrabVN_*`, `Grab_*` accounts) is the last editor of 14.0% of Da Nang ways and 7.8% of Hoi An's (*measured*). Grab maps for cars and motorbikes, which explains the crossings, lanes and turn lanes (e.g. Võ Nguyên Giáp, the beach road, has `lanes`, `turn:lanes`, `cycleway:*` on most segments) and the near-absence of pedestrian comfort tags.
- **Sidewalk tags are absent even on the obvious streets.** Đường Bạch Đằng, the Han River front, has 21 segments in OSM and none carries a sidewalk tag; Lê Duẩn is the exception (19 primary segments with `sidewalk=both`) (*measured*).
- **A sidewalk tag would still mislead.** Vietnamese sidewalks are routinely taken over by parked motorbikes and vendors, pushing walkers into the road ([Ta et al., Hasselt University, Da Nang pedestrian study](https://documentserver.uhasselt.be/bitstream/1942/49889/1/main.pdf), per its introduction as surfaced by search; I couldn't load the PDF itself, so **unverified** beyond that framing). Da Nang ran a sidewalk enforcement drive in 2025–26: downtown improved, suburbs like Cam Le did not ([Tuoi Tre, 2026-04-02](https://news.tuoitre.vn/da-nang-crackdown-on-sidewalk-encroachment-sees-little-change-in-suburban-areas-103260402155225636.htm); [Tuoi Tre, 2026-04-08](https://news.tuoitre.vn/businesses-encroach-on-sidewalks-in-da-nang-despite-enforcement-push-103260408110114258.htm); [Tuoi Tre, 2026-03-20](https://news.tuoitre.vn/da-nang-bans-sidewalk-trading-placement-of-plant-pots-103260320165606805.htm)). "Has a sidewalk" and "the sidewalk is walkable" are different facts, and OSM holds neither for Da Nang.
- **Alleys are tagged inconsistently.** The OSM Vietnam guide recommends `highway=service` + `service=alley` for narrow public lanes and notes the local words (ngõ/ngách in the north, **kiệt** in central Vietnam, hẻm in the south), but mappers also use `highway=residential` for the same thing ([OSM wiki: Vietnam Mapping Guide](https://wiki.openstreetmap.org/wiki/Vietnam_Mapping_Guide); [OSM community thread](https://community.openstreetmap.org/t/should-narrow-roads-that-are-too-small-for-cars-still-be-tagged-as-minor-roads/117397/161)). In Da Nang, the `Kiệt …` name is a more reliable alley signal than the tag (1,484 named vs 1,326 tagged). Alleys are often the pleasant shortcut (quiet, shaded, local life) and sometimes a motorbike rat-run: a router has no way to tell which.
- **Hoi An's walking rules are not in the map.** Motor vehicles are kept out of the Old Town core in daily windows, but sources disagree on the hours: 9–11 and 15–21:30 (22:00 in summer) per [Viet Nam News](https://vietnamnews.vn/society/1348921/ancient-town-limits-cars-traffic-in-old-quarter-area.html), 8:30–11 and 15–21:30 per [Takeoff](https://takeoff.to/destinations/hoi-an-old-town), 9–11:30 and 15–21:30 per [Hoi An Day Trip](https://hoiandaytrip.com/hoi-an-old-town-maps/); motorbikes are also kept out on the 14th and 15th of each lunar month per [Sygic](https://guides.travel.sygic.com/production/en/Hoi_An) (**unverified** against an official source). In OSM only Lê Lợi carries a time rule (`motorcar:conditional=no @ (17:00-23:00)`) (*measured*). Whether a walk through the Old Town is lovely or a dodge between motorbikes depends on the hour, and that has to live in Issho's own data.

### OSM tags that mark pleasantness, and their state here

| Signal | OSM tags | Da Nang–Hoi An state |
|---|---|---|
| Traffic exposure | `highway` class, `lanes`, `maxspeed` | Usable: class is on every way; lanes/maxspeed on ~20% of Da Nang road length |
| Car-free | `highway=footway/pedestrian/path/steps/living_street`, `foot=designated` | Usable: 2,655 foot-only ways in Da Nang (163 km of footway), 194 in Hoi An Old Town |
| Pavement | `sidewalk*`, `footway=sidewalk` | Near-empty (1.8% / 7.5% of length) and wouldn't capture obstruction anyway |
| Surface | `surface`, `smoothness` | `surface` on 62% / 32%; `smoothness` near-empty in Da Nang |
| Night | `lit` | 4.6% / 10.4% |
| Shade | `natural=tree`, `natural=tree_row`, `shade`, `covered` | Near-empty; use canopy rasters (section 2) |
| Green and water | `leisure=park`, `landuse=grass`, `natural=water`, `waterway=riverbank`, `natural=beach` | Usable: parks and water bodies are mapped |
| Amenities on the way | `amenity=bench/drinking_water/toilets` | Sparse |
| Named routes | `type=route` + `route=hiking/foot` | **None** in the whole area |

Tag definitions: [Key:sidewalk](https://wiki.openstreetmap.org/wiki/Key:sidewalk) ("Indicates the presence or absence of a sidewalk"; values both, left, right, no, separate).

## 2. Shade and green from outside OSM

| Source | What | Resolution | Licence | Cost |
|---|---|---|---|---|
| Meta/WRI Canopy Height Maps | Tree canopy height from Maxar imagery, global | 1 m | [CC BY 4.0](https://registry.opendata.aws/dataforgood-fb-forests/) | Free (AWS open data) |
| ESA WorldCover 2021 (v200) | Land cover incl. class 10 "Tree cover" | 10 m | [CC BY 4.0](https://developers.google.com/earth-engine/datasets/catalog/ESA_WorldCover_v200) | Free |

A 10 m class is too coarse for a single street's shade; the 1 m canopy map is the right grain for "share of this street under trees". The imagery behind the canopy map is from 2009–2020 ([WRI](https://www.wri.org/outcomes/new-ai-model-maps-every-tree-earth)), so new streets in Da Nang's growth areas will be missing; a successor (CHMv2) is described in [arXiv 2603.06382](https://arxiv.org/pdf/2603.06382), licence **unverified**. Building shade (tall buildings on the sunny side) is not covered by either; it would need building heights, which OSM has sporadically (not measured).

**Unverified:** how accurate the canopy map is in Da Nang's streets specifically. Worth a spot check against a few streets the owner knows before relying on it.

## 3. Pedestrian routers: what each can weigh

| Router | Hosting | Built-in pleasantness knobs | Custom per-way score? | Cost |
|---|---|---|---|---|
| **Valhalla** | Self-host (open source) | `walkway_factor`, `sidewalk_factor`, `alley_factor`, `driveway_factor`, `service_factor`, `use_living_streets`, `use_lit`, `use_hills`, `use_tracks`, `step_penalty` | Not as a request option (would need a fork or tag preprocessing) | Free software; a server |
| **GraphHopper** | Self-host or hosted API | Custom models over encoded values: `road_class`, `road_environment`, `surface`, `smoothness`, `foot_network`, `max_speed`, `average_slope`, `urban_density`, `curvature`; `distance_influence` caps the detour | Yes, self-hosted: add an encoded value from your own data and weigh it with `priority` | Hosted: Free (500 credits/day, no flexible mode) to €479/mo |
| **openrouteservice** | Self-host or hosted (HeiGIT) | foot `green` and `quiet` weightings | Yes, self-hosted only: CSV of OSM way id → floats in [-1, 1], selected by `csv_column` | Free API with quotas; self-host free |
| **OSRM** | Self-host | None: foot profile weighs duration only, flat 5 km/h with gravel/sand slowdowns | Only by writing a Lua profile | Free software |
| **Google Routes** (WALK) | Hosted | `avoidIndoor` only; walking is beta | No | Compute Routes Essentials $5 per 1,000 after 10,000 free a month |
| **Mapbox Directions** (walking) | Hosted | `walkway_bias` (-1..1), `walking_speed`; up to 2 alternatives | No | Per request (rates on pricing page, not checked) |

Sources: Valhalla pedestrian options ([API reference](https://valhalla.github.io/valhalla/api/route/api-reference/)); GraphHopper encoded values, `priority` and `distance_influence` ("The larger `distance_influence` is, the more GraphHopper will focus on finding short routes", default 70) ([custom models doc](https://github.com/graphhopper/graphhopper/blob/master/docs/core/custom-models.md)); GraphHopper plans and the Free plan's "cannot use the flexible mode" ([pricing](https://www.graphhopper.com/pricing/)); openrouteservice `green` and `quiet` ("prefer ways through green areas over a shorter route") ([routing options](https://giscience.github.io/openrouteservice/api-reference/endpoints/directions/routing-options)) and CSV storage, "Not available on the public HeiGIT instance" ([build config](https://github.com/GIScience/openrouteservice/blob/main/docs/run-instance/configuration/engine/profiles/build.md)); OSRM ([foot.lua](https://github.com/Project-OSRM/osrm-backend/blob/master/profiles/foot.lua)); Google WALK beta warning and route modifiers ([RouteTravelMode](https://developers.google.com/maps/documentation/routes/reference/rest/v2/RouteTravelMode), [RouteModifiers](https://developers.google.com/maps/documentation/routes/reference/rest/v2/RouteModifiers), [pricing](https://developers.google.com/maps/billing-and-pricing/pricing)); Mapbox ([Directions API](https://docs.mapbox.com/api/navigation/directions/)).

Notes:

- **openrouteservice's hosted green and quiet routing was built for Germany.** Heidelberg's own posts say quiet routing was initially "only for all of Germany" and green routing "restricted to Germany only" ([GIScience blog, quiet](https://giscienceblog.uni-heidelberg.de/?p=7621); [green](https://giscienceblog.uni-heidelberg.de/?p=7497)). Whether it has since been extended is **unverified**; I'd assume it does nothing in Vietnam until tested with an API key.
- **Valhalla's knobs are the best match for the question but starve on Da Nang's tags.** `sidewalk_factor` has nothing to act on for 98% of road length and `use_lit` for 95%. `walkway_factor` (footways), `alley_factor`, `use_living_streets` and the road-class costing do work.
- **The detour cap is a first-class setting in GraphHopper** (`distance_influence`), which is exactly "a couple of minutes longer, not ten". In Valhalla the equivalent is tuning the factors; in either case Issho can also compare against Google's time and reject routes over a cap.
- **Google's terms limit mixing.** Google Maps content may be cached or stored "only as expressly permitted" in the service terms, may not be exported "for use outside the Services", and Place IDs may be stored indefinitely ([Routes policies](https://developers.google.com/maps/documentation/routes/policies)). Using Google's walking time as a baseline in-app is fine; storing Google routes as Issho's own building blocks, or drawing them on a non-Google map, is not something to assume (exact clause on non-Google maps **unverified**; the service terms page didn't load).

## 4. Popularity data (heatmaps and the like)

| Source | What | Terms for an app like Issho |
|---|---|---|
| Strava Global Heatmap | Aggregated public activities | OSM users have permission "to use the Strava heatmap data for tracing into OSM only"; full resolution needs a (free) login; since March 2025 tiles need strava.com cookies ([OSM wiki: Strava](https://wiki.openstreetmap.org/wiki/Strava), edited 2026-11-02) |
| Strava Metro | Aggregated, de-identified counts | For "organizations that plan, own, or maintain active transportation" infrastructure; no public redistribution; no competitive products; no AI/ML use without a separate agreement; free ([Metro terms](https://metro.strava.com/terms), updated 2026-09-04; [metro.strava.com](https://metro.strava.com/)) |
| Strava API | A user's own activities | "Strava Data related to other users … may not be displayed or disclosed" ([API agreement](https://www.strava.com/legal/api), effective 2026-06-01) |

Issho is not an infrastructure planner, so Strava Metro is out; the heatmap's only permitted use is improving OSM. That is still useful indirectly: an owner or volunteer could trace missing footways into OSM from the heatmap, and Issho then benefits through OSM. How much Strava walking/running activity exists in Da Nang is **unverified** (I didn't log in to view it). Other activity platforms (Komoot, AllTrails, Garmin) were not checked; their popular-route data is similarly proprietary.

## 5. Published local walks

- **Da Nang:** a riverside loop from Han Market past the Cathedral, the Han River promenade, the Cham Museum and APEC Park to the Dragon Bridge and the Love Lock Bridge, about 1.8 km ([Hoi An Day Trip](https://hoiandaytrip.com/da-nang-self-guided-walking-tour/)); a GPSmyCity "Danang Introduction Walking Tour" ([GPSmyCity](https://www.gpsmycity.com/tours/danang-introduction-walking-tour-7147.html), not opened); **Bach Dang walking street**, between the Dragon Bridge and Tran Thi Ly Bridge, opened 2024-06-07 as a pilot until 2028 per [Nomado](https://www.nomadotravel.app/en/attractions/bach-dang-walking-street) (**unverified** against an official source; the search didn't surface danangfantasticity.com).
- **Hoi An:** the Old Town itself is the walk; its timed vehicle ban (section 1) is the useful fact.
- **OSM route relations:** none for hiking, foot, walking, bicycle or MTB in lat 15.85–16.20, lon 108.13–108.36 (*measured*).

These are tourist loops, not walks between arbitrary stops. They're useful as seeds for hand-curated good segments (the Han and Tran Hung Dao riverfronts, My Khe beach front, the Old Town lanes), not as a routing source. Copying a commercial guide's route text is a licensing question; drawing the same streets ourselves is not.

## 6. Street-level imagery

| Provider | Da Nang–Hoi An coverage | Terms |
|---|---|---|
| Mapillary | **Unverified**: the API needs an access token ("All requests … must be authorized") so I couldn't count images | Images CC BY-SA per Mapillary's help centre; commercial use under its terms and Developer Usage Policy ([API docs](https://www.mapillary.com/developer/api-documentation); [Licenses](https://help.mapillary.com/hc/en-us/articles/115001770409-Licenses), 403 to my fetch). Has object detections (benches, poles, signs) but no documented "sidewalk obstructed" class |
| Panoramax | **7 images** in the whole area (*measured* via [api.panoramax.xyz](https://api.panoramax.xyz/api/search?bbox=108.13,15.85,108.36,16.12&limit=100)) | Open |
| KartaView | API didn't respond; **unverified** | |
| Google Street View | Official coverage in Vietnam reportedly published July 2025; extent in Da Nang–Hoi An **unverified** | Not licensable as data; viewing only under Google's terms |

Imagery is an aid for a human curator ("is this alley pleasant?"), not an automatic source at Pilot scale. A free Mapillary token would settle its coverage in minutes.

## 7. Rides between stops (brief)

- **Google Routes `TWO_WHEELER`** is supported in Vietnam (VN) ([coverage list](https://developers.google.com/maps/documentation/routes/coverage-two-wheeled)), beta like walking, same pricing as other Compute Routes calls.
- **Valhalla `motor_scooter`** costing has `use_primary` (willingness to use primary roads, default 0.5) and `use_hills`; `bicycle` has `use_roads` and `avoid_bad_surfaces` ([Valhalla API reference](https://valhalla.github.io/valhalla/api/route/api-reference/)).
- **Cycling data is thin**: about 4 km of `highway=cycleway` in central Da Nang (*measured*), though the beach road carries `cycleway:*` lanes. Hoi An's countryside is the place people cycle; no cycling route relations are mapped.
- **GrabMaps** (Grab's own map) is sold through Amazon Location Service, which offers car, walking and motorcycle routing ([AWS blog](https://aws.amazon.com/blogs/mobile/building-high-quality-cost-effective-apps-in-southeast-asia-with-amazon-location-service)); Vietnam coverage and price **unverified**.
- For a ride, the questions are time, cost and whether a Grab is easy to get, not the beauty of the road. A scenic exception (the Son Tra peninsula road, the coast road to Hoi An) is a curated fact, not a routing one.

## 8. What this suggests for "Routes worth walking"

Options, in rising order of effort:

1. **Google's walk, unchanged.** Free under the cap, accurate times, no pleasantness. The baseline everything else is measured against.
2. **Curated good segments.** The owner or trusted locals draw a few dozen pleasant stretches per area once (riverfronts, beach front, Old Town lanes with their hours, good alleys, parks) and plans prefer stitching through them. Small, high quality, works with any router. Fits the map's "building block" idea.
3. **A computed score on a self-hosted router.** Per OSM way: road class and lanes (traffic), foot-only or living street, park or water adjacency, alley, canopy share from the 1 m map, a curated boost or penalty. Fed to GraphHopper (custom encoded value + `distance_influence`) or openrouteservice (CSV column). One server, no per-request fee; needs a data pipeline.
4. **Contribute back to OSM.** Sidewalks, `lit`, Old Town conditional access, footways traced from the Strava heatmap. Improves every router, slowly.

The research doesn't choose; it says option 3 without option 2 will be wrong in the places that matter most (obstructed sidewalks, timed bans, which alleys are lovely), and option 2 alone doesn't scale beyond a handful of areas.

## Method

Overpass queries against `https://overpass-api.de/api/interpreter`, with `[bbox:…]` as above and `out count` per filter, e.g.

```
[out:json][bbox:15.98,108.13,16.12,108.27];
way[highway~"^(trunk|primary|secondary|tertiary|unclassified|residential|living_street)(_link)?$"][~"^sidewalk"~"."];out count;
```

Length shares came from `way[…];out meta geom;` with segment lengths summed on an equirectangular approximation; editor and year shares from the same download. The Old Town box is lat 15.872–15.884, lon 108.320–108.340. Counts move daily as people edit; Da Nang has been edited heavily through 2026.
