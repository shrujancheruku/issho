-- Venue curation (M8, 00023). A place has one row per venue category;
-- curation sets all of a place's rows together, so a place's status is the
-- same in every row (a place discovered in a new category takes it too).
-- Status is still read with rejected > approved > candidate, in case rows
-- disagree from before 00023.

-- name: ListVenueCities :many
select c.id, coalesce(c.name->>'en', c.id)::text as name, c.approved_venues_only,
       count(distinct v.google_place_id) filter (where v.status = 'candidate')::int as candidates,
       count(distinct v.google_place_id) filter (where v.status = 'approved')::int as approved,
       count(distinct v.google_place_id) filter (where v.status = 'rejected')::int as rejected
from app.cities c
left join app.venues v on v.city_id = c.id
group by c.id
order by c.id;

-- name: SetApprovedVenuesOnly :execrows
update app.cities set approved_venues_only = @approved_venues_only where id = @id;

-- A city's places in one status, optionally in one area or category, a
-- page at a time: by area, then place id (a stable order to page through).
-- plans = draft or open plans that use the place, which rejecting it
-- doesn't change. total = how many places match, for paging.
-- name: ListCurationVenues :many
with places as (
  select v.google_place_id, v.area_id,
         array_agg(v.category_id order by v.category_id)::text[] as categories,
         (case when bool_or(v.status = 'rejected') then 'rejected'
               when bool_or(v.status = 'approved') then 'approved'
               else 'candidate' end)::text as status,
         bool_or(v.added_by is not null) as added,
         max(v.reviewed_at)::timestamptz as reviewed_at
  from app.venues v
  where v.city_id = @city_id and (@area_id::text = '' or v.area_id = @area_id)
  group by v.google_place_id, v.area_id
  having @category_id::text = '' or bool_or(v.category_id = @category_id)
), used as (
  select pl.venue_place_id as place_id from app.plans pl
  where pl.city_id = @city_id and pl.status in ('draft', 'open') and pl.venue_place_id is not null
  union all
  select pl.follow_up_venue_place_id from app.plans pl
  where pl.city_id = @city_id and pl.status in ('draft', 'open') and pl.follow_up_venue_place_id is not null
), uses as (
  select place_id, count(*)::int as plans from used group by place_id
)
select p.google_place_id, p.area_id, coalesce(a.name->>'en', p.area_id)::text as area_name,
       p.categories, p.status, p.added, p.reviewed_at, coalesce(u.plans, 0)::int as plans,
       count(*) over ()::int as total
from places p
join app.areas a on a.id = p.area_id
left join uses u on u.place_id = p.google_place_id
where p.status = @status::text
order by area_name, p.google_place_id
limit @max_rows offset @skip;

-- name: SetVenueStatus :execrows
update app.venues
set status = @status, reviewed_by = @reviewed_by, reviewed_at = @at
where city_id = @city_id and google_place_id = any(@place_ids::text[]);

-- Which of these places the city already has, and their status.
-- name: ListKnownVenues :many
select v.google_place_id, v.area_id,
       array_agg(v.category_id order by v.category_id)::text[] as categories,
       (case when bool_or(v.status = 'rejected') then 'rejected'
             when bool_or(v.status = 'approved') then 'approved'
             else 'candidate' end)::text as status
from app.venues v
where v.city_id = @city_id and v.google_place_id = any(@place_ids::text[])
group by v.google_place_id, v.area_id;

-- name: ListAllVenueCategories :many
select id, coalesce(name->>'en', id)::text as name, google_types, search_radius_m
from app.venue_categories
order by id;

-- An admin adds a place, approved, in one category. If the place is
-- already there in that category it's approved instead (not if that row
-- is another city's: a place id belongs to one city).
-- name: AddVenue :exec
insert into app.venues (google_place_id, city_id, area_id, category_id, status, added_by, reviewed_by, reviewed_at)
values (@google_place_id, @city_id, @area_id, @category_id, 'approved', @added_by, @added_by, @at)
on conflict (google_place_id, category_id) do update
set status = 'approved', reviewed_by = excluded.reviewed_by, reviewed_at = excluded.reviewed_at
where app.venues.city_id = excluded.city_id;

-- name: ListCurationAreas :many
select id, city_id, coalesce(name->>'en', id)::text as name,
       st_y(centroid::geometry)::float8 as lat, st_x(centroid::geometry)::float8 as lng,
       coalesce(st_asgeojson(boundary), '')::text as boundary
from app.areas
order by city_id, name->>'en', id;
