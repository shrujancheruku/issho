# Research: two-wheeler routing on Google Maps in Vietnam

Issue [#394](https://github.com/shrujancheruku/issho/issues/394), part of map [#334](https://github.com/shrujancheruku/issho/issues/334); it feeds the grilling ticket [#393](https://github.com/shrujancheruku/issho/issues/393) "Rides between stops in Vietnam". Researched 2026-11-05 against main at `6986d0b`. This branch is never merged.

Starting point: the walk composer (`backend/internal/walks/`) asks Routes `computeRoutes` for one walk through a walk's stops, `travelMode: WALK`, with `optimizeWaypointOrder` when the order may change (`WalkThrough` in `backend/internal/maps/google/google.go`). `build-matrix` asks Compute Route Matrix for area-to-area WALK and DRIVE times (`docs/google-maps.md`). Nothing here was tested against the live API; every claim comes from the documents linked next to it.

## Answer in brief

- **Vietnam is covered.** Google's list of countries supported for two-wheeled routes includes "VN Vietnam". `TWO_WHEELER` is "Two-wheeled, motorized vehicle. For example, motorcycle", and is in **beta**, like WALK.
- **Waypoint optimisation: documented as mode-independent, but not tested.** Neither the `computeRoutes` reference nor the optimisation guide restricts `optimizeWaypointOrder` to any travel mode; the only conditions are no `via` waypoints and no `TRAFFIC_AWARE_OPTIMAL`. Up to 25 intermediates. That it actually works with `TWO_WHEELER` in Vietnam is **unconfirmed** until one real request is made.
- **Cost: every two-wheeler request is Enterprise.** Two-wheeled routing is an Enterprise trigger for both Compute Routes and Compute Route Matrix: **15 USD per 1,000** (first tier) with **1,000 free a month**, against WALK with optimisation (Pro) at **10 USD** with 5,000 free, or plain WALK (Essentials) at **5 USD** with 10,000 free. One SKU is billed per request, the highest one triggered, so two-wheeler + optimisation is Enterprise, not Enterprise + Pro.
- **Missing or different:** the same beta caveat and the **same must-display warning** as walking routes; traffic models are DRIVE-only (`trafficModel` needs DRIVE + `TRAFFIC_AWARE_OPTIMAL`); `routingPreference` is allowed (DRIVE and TWO_WHEELER only), and any `TRAFFIC_AWARE*` value would add nothing to the price since Enterprise is already the top SKU. One `travelMode` per request, so an outing that mixes walking and riding legs needs separate requests per mode. Google does not document which response fields, if any, are absent for TWO_WHEELER (**unconfirmed**).
- **Terms: nothing specific to two-wheelers or Vietnam.** The Routes API rules that already apply to walks apply unchanged: Routes content may be shown without a Google map but **never with a non-Google map**; only lat/lng may be cached, for up to 30 days; no use in apps embedded in a vehicle; the beta warning must be displayed. Issho shows a route to plan, not turn-by-turn navigation, so the navigation restrictions don't bite.
- **Grab for a group of 3 to 6: each person rides a separate GrabBike, or the group takes one car.** Vietnamese law lets a motorbike carry one passenger (with narrow exceptions), so 4 people means 4 bikes booked separately. Grab's "group rides" booking is a shared single vehicle for up to 3 users, piloted only in Hanoi and Ho Chi Minh City. Grab's July 2026 fare table for Da Nang: GrabBike **12,273 VND** for the first 2 km then **4,222 VND/km**, GrabCar 4-seat 27,000 then 12,175/km, 6-seat 33,382 then 14,335/km, plus a time component, platform fee and surge.
- **For #393:** a 3 km hop costs a group of 4 roughly 4 × 20,000 = 80,000 VND on bikes, against roughly 40,000 to 55,000 VND for one 4- or 6-seat car (estimates from the table, before the time component, surge and car fees). For groups of 3 to 6, a car leg is cheaper and keeps the group together, and a car leg is a **DRIVE** route (Essentials or Pro), cheaper to compute than TWO_WHEELER. TWO_WHEELER matters if people ride their own motorbikes, or as the more accurate time for a GrabBike leg.

## Google Routes API

### Coverage

- [Countries and regions supported for two-wheeled vehicles](https://developers.google.com/maps/documentation/routes/coverage-two-wheeled): "Two-wheeled vehicle routes are currently supported in the following countries/regions", including the row "VN | Vietnam". The page has no last-updated date.
- [RouteTravelMode](https://developers.google.com/maps/documentation/routes/reference/rest/v2/RouteTravelMode): `TWO_WHEELER` is "Two-wheeled, motorized vehicle. For example, motorcycle. Note that this differs from the `BICYCLE` travel mode which covers human-powered mode." Like `WALK` and `BICYCLE` it is beta: routes "might sometimes be missing clear sidewalks, pedestrian paths, or bicycling paths".
- [Get a two-wheeled vehicle route](https://developers.google.com/maps/documentation/routes/route_two_wheel): set `"travelMode":"TWO_WHEELER"` in `computeRoutes`. "You must display this warning to the user for all walking, bicycling, and two-wheel routes that you display in your app." "Requests using a travel mode of TWO_WHEELER are billed at a higher rate."
- [Available route matrix options](https://developers.google.com/maps/documentation/routes/options-rm): Compute Route Matrix lists "two-wheeled vehicle" among its travel modes, also in beta.

### Waypoint optimisation

- [computeRoutes reference](https://developers.google.com/maps/documentation/routes/reference/rest/v2/TopLevel/computeRoutes), `optimizeWaypointOrder`: "the service attempts to minimize the overall cost of the route by re-ordering the specified intermediate waypoints. The request fails if any of the intermediate waypoints is a `via` waypoint." No travel-mode restriction is stated. `intermediates`: "Up to 25 intermediate waypoints are supported."
- [Optimize the order of stops on your route](https://developers.google.com/maps/documentation/routes/opt-way): conditions are no `via` waypoints and `routingPreference` not `TRAFFIC_AWARE_OPTIMAL`. The only example uses DRIVE; the page doesn't list supported modes.
- Issho already runs `optimizeWaypointOrder` with WALK, so it isn't DRIVE-only in practice. Whether it works with TWO_WHEELER in Vietnam: **unconfirmed** (no mode restriction is documented; no request was made for this note).

### Other fields

From the [computeRoutes reference](https://developers.google.com/maps/documentation/routes/reference/rest/v2/TopLevel/computeRoutes):

- `travelMode` is one per request; intermediates have no mode of their own.
- `routingPreference`: "You can specify this option only when the `travelMode` is `DRIVE` or `TWO_WHEELER`, otherwise the request fails."
- `trafficModel` is only available with `TRAFFIC_AWARE_OPTIMAL` and DRIVE, so not with TWO_WHEELER.
- `computeAlternativeRoutes`: "No alternative routes are returned for requests that have intermediate waypoints" (any mode).
- [Route modifiers](https://developers.google.com/maps/documentation/routes/route-modifiers) (`avoidTolls`, `avoidHighways`, `avoidFerries`, `avoidIndoor`) carry no mode restriction on that page; avoidance isn't supported by Compute Route Matrix.
- No Google page lists response fields that are missing for TWO_WHEELER. The fields Issho's mask asks for (duration, distance, polyline, legs, optimised index) are generic route fields; that they come back for TWO_WHEELER is **unconfirmed** until tested.

### Pricing

[Routes SKU details](https://developers.google.com/maps/billing-and-pricing/sku-details/routes):

- **Compute Routes Pro** is triggered by 11 to 25 intermediates, `"optimizeWaypointOrder": "true"`, `TRAFFIC_AWARE`/`TRAFFIC_AWARE_OPTIMAL`, or a location modifier (side of the road, heading, vehicle stopover).
- **Compute Routes Enterprise** is triggered by two-wheeled vehicle routing, toll calculation, traffic information on polylines or large vehicle routing.
- **Compute Route Matrix Enterprise**: two-wheeled vehicle routing, toll calculation or large vehicle routing elements. Waypoint optimisation doesn't apply to the matrix.
- "If you request any features from a higher-priced SKU, then your request is billed at the higher rate." "You are only charged for one SKU per request." [Usage and billing](https://developers.google.com/maps/documentation/routes/usage-and-billing) says the same: Enterprise is "Billed for requests that use an enterprise feature, such as two-wheel routing".

[Global price list](https://developers.google.com/maps/billing-and-pricing/pricing), first paid tier (to 100,000 a month):

| SKU | Free a month | USD per 1,000 |
|---|---|---|
| Compute Routes Essentials (plain WALK) | 10,000 | 5.00 |
| Compute Routes Pro (WALK + optimisation) | 5,000 | 10.00 |
| Compute Routes Enterprise (any TWO_WHEELER) | 1,000 | 15.00 |
| Compute Route Matrix Essentials / Pro / Enterprise (per element) | 10,000 / 5,000 / 1,000 | 5.00 / 10.00 / 15.00 |

The old names ("Basic", "Advanced", "Preferred") map to Essentials, Pro and Enterprise; the code comment on `WalkThrough` still says "Advanced SKU".

Scale check: a TWO_WHEELER layer on the Da Nang–Hoi An matrix would be at most the 242 elements of today's per-city build every 25 days, inside the 1,000 free Enterprise elements. Composing walks is the cost to watch: each attempt with a ride is one Enterprise request, and the free 1,000 go 5 times faster than Pro's.

## Google Maps Platform terms

Read 2026-11-05 from the [Maps Platform Terms of Service](https://cloud.google.com/maps-platform/terms) and the [Service Specific Terms](https://cloud.google.com/maps-platform/terms/maps-service-terms) (whose change log's latest entry is 2026-10-28). Neither mentions two-wheelers, motorcycles or Vietnam.

- Service Specific Terms 19.1: "Customer may use Google Maps Content from the Routes API in Customer Applications without a corresponding Google Map."
- 19.2: "Customer must not use Google Maps Content from the Routes API in conjunction with a non-Google map."
- 19.3: "Customer may temporarily cache latitude (lat) and longitude (lng) values from the Routes API for up to 30 consecutive calendar days". The general "may cache place_id from ... Routes API" also applies. Durations and polylines are Routes content too; Issho's existing 30-day policy (`docs/google-maps.md`) covers them the same way for any mode.
- Terms 3.2.3(b): "Customer will not cache Google Maps Content except as expressly permitted under the Maps Service Specific Terms."
- Terms 3.2.3(f), no use in embedded vehicle systems: a phone app a rider carries isn't one.
- Terms 3.2.3(d)(iv) forbids building "real-time navigation functionality substantially similar to" Google Maps for Android from Directions, Geolocation and the Android SDK; 19.4.1's safety requirements apply to real-time navigation or apps "used during real-time driving". Showing a planned route with times isn't either, as long as the app doesn't become turn-by-turn guidance for the rider.
- The beta warning (see Coverage) is a documentation requirement that applies to every walking and two-wheeler route Issho displays.

## Grab in Da Nang and Hoi An

### Availability

- [Grab locations](https://www.grab.com/vn/en/locations/) lists Da Nang, Quang Nam and Hoi An; the page doesn't break services down by city.
- [GrabBike (Vietnam)](https://www.grab.com/vn/en/transport/bike/) is an "On-demand Motorcycle Ride Taxi Service" with "fixed and transparent fares through the app"; the price shown in the app is final.
- The [fare table](https://www.grab.com/vn/en/blog/bang-thong-tin-cac-dich-vu-tren-ung-dung-grab/) has a GrabBike row for Da Nang but **none for Quang Nam / Hoi An** (GrabCar has rows for both). That GrabBike is bookable in Hoi An, and at what price, is **unconfirmed** from Grab's pages.

### A group of 3 to 6

- **One passenger per bike.** Vietnam's Law on Road Traffic Order and Safety 2024 (36/2024/QH15), Article 33(1), lets a motorbike rider carry one person, two only when one is a sick person going to emergency care, a person under escort, a child under 12, or an elderly or disabled person ([Ha Tinh provincial government summary](https://hatinh.gov.vn/vi/bai-viet/4-truong-hop-duoc-phep-cho-2-nguoi-tren-xe-may-khong-bi-phat-tu-112025); the law's own text wasn't read). Grab's own one-passenger rule for GrabBike Vietnam is **unconfirmed**: the [consumer code of conduct](https://www.grab.com/vn/en/terms-policies/code-of-conduct-consumer) says only "Wear a helmet if travelling on a bike or scooter" and suggests a bigger car for "more than 4 passengers".
- **So a group of n rides n GrabBikes.** Grab's pages describe no way to book several bikes at once, so each person (or one person, several times) books a separate ride; drivers arrive at different times and the group regroups at the next stop. That a phone can only hold one active booking at a time is **unconfirmed**.
- **Grab group rides don't fit.** [Đặt xe nhóm](https://www.grab.com/vn/en/blog/dat-xe-nhom/) (a pilot from 2026-06-03 in Hanoi and 2026-07-27 in Ho Chi Minh City; Da Nang isn't named) lets "tối đa 03" users, with different pickups and drop-offs, share **one** vehicle, with a group leader who can pay for everyone or split the fare (split is cashless only). [Its terms](https://www.grab.com/vn/terms-policies/group-rides-terms-of-service/) took effect 2026-06-11 and don't name the eligible services.
- **One car is the group option.** GrabCar 4-seat and 6-seat cars take a group in one vehicle. Whether "4-seat" means 4 passenger seats or 4 seats including the driver (so 3 passengers) is **unconfirmed** from Grab's pages; a group of 6 may need the 6-seat or two cars.

### Fares

From Grab's [service fare table](https://www.grab.com/vn/en/blog/bang-thong-tin-cac-dich-vu-tren-ung-dung-grab/), "Cập nhật lần cuối: 08/07/2026":

| Da Nang | First 2 km (minimum) | Each km after | Time component (after 2 km) |
|---|---|---|---|
| GrabBike | 12,273 VND | 4,222 VND | 344 |
| GrabBike Plus | 15,709 VND | 5,204 VND | 363 |
| GrabCar 4-seat | 27,000 VND | 12,175 VND | 442 |
| GrabCar 6-seat | 33,382 VND | 14,335 VND | 864 |

- Quang Nam's GrabCar 4- and 6-seat rows match Da Nang's.
- The time column ("Giá cước tính theo thời gian di chuyển (sau 2km đầu tiên)") gives no unit; per minute is a guess (**unconfirmed**).
- On top: a GrabBike platform fee of 3,000 VND, night, waiting and weather surcharges, and prices that "may be adjusted based on supply and demand".
- Worked example (estimate, before the time component and surge): a 3 km hop is about 12,273 + 4,222 + 3,000 ≈ 20,000 VND per GrabBike, 80,000 for 4 people or 120,000 for 6; one GrabCar 4-seat is about 39,000 VND plus fees, a 6-seat about 48,000.

## What this means for #393 (input, not a decision)

- TWO_WHEELER can compute ride legs in Da Nang–Hoi An, and probably optimise stop order, at 1.5 times the per-request price of WALK with optimisation and a fifth of the free allowance. One real request would settle optimisation and the returned fields.
- A ride leg for a group is more likely a car than bikes: cheaper for 4 or more, keeps the group together, and routed with DRIVE, which is already in `travelMode()` and costs less than TWO_WHEELER.
- Mixing walking and riding legs in one outing takes one request per leg (or per run of same-mode legs): Routes has one `travelMode` per request, so WALK's stop optimisation can't optimise across a ride.
- Whatever is shown needs the beta warning, already required for walks.
