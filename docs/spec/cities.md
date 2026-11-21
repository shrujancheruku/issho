# Cities

From [Cities](https://github.com/shrujancheruku/issho/issues/555), with [What varies by city](https://github.com/shrujancheruku/issho/issues/545), [The stages of launch](https://github.com/shrujancheruku/issho/issues/548) and [Running Issho](https://github.com/shrujancheruku/issho/issues/557).

## Areas

- An **area** is one of the local government's units in a city. A plan's area is the area of its meeting point. An area is a label on a piece and a filter on the plans list, nothing more; nobody has a home area.
- An area has its official name in the city's language plus an English form. Area names are never machine-translated; a city's name is translated into each display language, because cities have real exonyms.
- **The person who vouches for a piece picks its area** from the city's list. Nothing is computed and no boundary is stored; the OpenStreetMap relation is kept only as a reference link.
- Every piece belongs to one of its city's areas. A place outside the list can't be used until the owner adds its area.
- The area filter on the plans list shows only areas that have an open plan.
- **When the government redraws its units**, areas follow, and the owner decides when a redraw is adopted (once the new names are what addresses use). On a merge the new area takes every piece and remembered filter of the old ones. On a split, the owner or an insider re-picks the area of each affected piece. Re-picking is a correction to a label: no re-check clock restarts and no plan closes.

## What a city is

- A city is a set of adjoining areas that passes two tests: one set of settings fits all of it ([What varies by city](city-settings.md)), and a resident of any area would reasonably go to an outing in any other, taken as about 60 minutes door to door by the city's way of getting around. The owner applies the second test by judgement; nothing computes it. Da Nang to Hoi An is about 45 minutes by motorbike, so it is one city.
- **The area list is the city's extent.** "I live here" means living in or beside those areas, self-declared.
- **A city is never split or merged.** A metro too large for the 60-minute test opens as the part that passes and grows. Insiders always cover the whole city, and at size the area filter does the narrowing.
- **A city grows only by the owner adding an area**, which needs the 60-minute test and a recorded reason. An area is removed only when no piece is in it.
- **Every place in a city's name has at least 2 of the opening 12 plans.** If Hoi An can't supply two, the city opens as "Da Nang" and is renamed later.

## A city's states

- **Planned:** the owner has announced it. It is listed, takes a waitlist, and its plans show as they are made. Only a planned city's waitlist counts toward the 150 residents below.
- **Open:** admitting people. Its stage of launch is its own ([The stages of launch](launch.md)).
- **Closed.**
- At most one city is planned at a time. The rule once there are two cities is decided after the second opens.
- A person who switches their city to a planned one joins its waitlist.

## People whose city isn't listed

They leave a **request**: an email with a typed city and country. It is not an account. It earns one email if that city becomes planned, and promises nothing.

## Choosing the next city

The owner picks; there is no formula. Three conditions must hold:

- A person who can vouch (the owner or an insider) lives there or will stay long enough to make the 12 plans.
- Google's map works there.
- The country's legal check is done, if it is the first city in that country.

Requests are evidence for the choice, not a trigger. The scarce thing is a person who can vouch. A second city waits for the first's public launch ([The stages of launch](launch.md#a-second-city)).

## What a new city costs

Before it opens:

1. A person who can vouch
2. The area list
3. The ten city settings
4. 12 plans around 12 different vouched activities, across at least three areas and every place in the city's name ([Plan knowledge](plan-knowledge.md))
5. A calendar filled in for the next 12 months
6. A lead-source list with at least the sources the 12 plans were found through
7. The app's text read by a fluent person in each of the city's languages, the 12 opening plans in each language and checked by a person, and the emails and legal texts in each ([Languages](languages.md))
8. The country's legal check, if it is the first city there
9. 150 residents on its waitlist
10. A moderator and a reader for its local language ([Running Issho](running-issho.md))
11. The owner reviewing every plan until the model judge earns trust there

While it runs: re-checks of pieces at 6 and 12 months, the calendar every 6 months, and sizing the weekly waves.

- A new city repeats plan review only. The owner approving every group and every crew offer is a one-time precaution of the first city: plan review tests local knowledge, which is new in each city, and the other two test rules that are the same everywhere.
- Nothing on the list needs code written per city.

## A city in trouble

- **Fewer than 12 plans after opening** is a warning to the owner and nothing more. The 60% rule already shrinks waves when plans are short.
- **No person able to vouch living there for 30 days** is a warning to the owner. Lapsing vouches then close plans on their own.

## Closing a city

- The owner's decision only, with 30 days' notice by email and in the app.
- From the notice: no new rounds open, and set outings still run.
- At the close: crews and their rooms end; plans and pieces are kept but not shown.
- Accounts stay, with history, standing and kept-apart pairs intact. A person can switch to another city, and otherwise sees "closed".
- A planned city that is abandoned closes the same way, with one email to its waitlist.

## Which city a visitor with no account sees

1. A shared plan or join link opens in its own city.
2. With one listed city, the visitor sees that city.
3. With several, the visitor picks from a list, with "my city isn't here" leading to a request.

The choice is remembered on the device. Nothing is guessed from location or IP address.
