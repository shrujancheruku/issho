# Research: terms and law on scraping and reuse

Ticket: [#537](https://github.com/shrujancheruku/issho/issues/537), on the v2 map [#532](https://github.com/shrujancheruku/issho/issues/532). Feeds "Where plan knowledge comes from". Read 2026-11-12.

**This is research, not legal advice.** It reads the published terms and statutes as written. A Vietnamese lawyer should confirm the Vietnam parts before Issho relies on them commercially. Anything marked **unverified** was not read at its primary source.

## Answer in brief

- **Google Maps Platform.** Store place IDs for ever. Store latitude and longitude for up to 30 days. Everything else (names, addresses, hours, ratings, reviews, photos, and also **route durations and distances**) may only be fetched live and shown, never kept. Nothing may be *derived* from Google content: Google's own example of a banned derivation is running Places coordinates through a point-in-polygon test, which is what v1 does to assign venues to areas. Google content may not go into a model's training, testing or validation.
- **Review sites and social platforms** (Foody, Tabelog, Yelp, Tripadvisor, Facebook, TikTok) all forbid copying or automated collection in their terms. Most also forbid commercial use of what they show. The only lawful routes to keep their content are their paid APIs (Yelp, Tripadvisor) under those APIs' caching rules, or a written licence.
- **Open data can be stored.** OpenStreetMap (ODbL: attribute, and share alike any derived *database* you publish), Overture Maps Places (CDLA-Permissive-2.0 or Apache-2.0) and Foursquare OS Places (Apache-2.0).
- **Search-and-cite and scrape-and-keep are different things, in the terms and in law.** Reading a page and linking to it is what every site invites. Keeping a copy is what the terms forbid, and what database rights (in the EU) and copyright (everywhere) protect.
- **Facts are free in law, but not always under contract.** Vietnam's IP law excludes "data" from copyright and has no database right. The US does the same (*Feist*). Japan protects only creative databases. The EU adds a database right against extracting a *substantial* part. So "this café has big tables and stays quiet in the afternoon", written in Issho's own words from someone's observation, can be kept. The source's text, photos and ratings can't. The exception is Google: its contract forbids creating content "based on" Google Maps Content, whatever the law says about facts.
- **Language models with web search.** Anthropic's web search tool requires citations when its output is shown to users. It says nothing about storing results; what Issho may keep is then governed by the source sites and the law above. Gemini's Grounding with Google Search lets results be shown only to the person who asked and forbids building a database from them. Grounding with Google Maps (Gemini API) allows 90 days for display tuning and six months in a user's chat. Google's newer "Maps Experience Agent Tools" allow 30 days and two years in chat history. None of these lets Issho build a stored catalogue of recommendations from their answers.

## Google Maps Platform

Sources: the [Terms of Service](https://cloud.google.com/maps-platform/terms) (last modified 2026-08-26), the [Service Specific Terms](https://cloud.google.com/maps-platform/terms/maps-service-terms) (last modified 2026-10-28), the [Places policies](https://developers.google.com/maps/documentation/places/web-service/policies) and the [Routes policies](https://developers.google.com/maps/documentation/routes/policies).

### What may be stored

| Content | Rule | Source |
|---|---|---|
| Place IDs (and other Google IDs) | Indefinitely: "Customer may cache (a) place_id from Places API, Directions API, Geolocation API and Routes API" | Service Specific Terms §3 (Google ID Caching) |
| Latitude/longitude from Places | 30 days: "Customer may temporarily cache latitude and longitude values from the Places API for up to 30 consecutive calendar days" | SST §14.3 |
| Latitude/longitude from Routes | 30 days, same wording | SST §19.3 |
| Route durations, distances, polylines | **No caching permission at all.** §19 (Routes) and §4 (Directions) permit only lat/lng; §5 (Distance Matrix) has no caching clause. The default applies: "Customer will not cache Google Maps Content except as expressly permitted under the Maps Service Specific Terms" | ToS §3.2.3(b); SST §§4, 5, 19 |
| Names, addresses, reviews | Never: "copy and save business names, addresses, or user reviews" is a listed example of scraping | ToS §3.2.3(a)(iii) |
| Anything derived | "Customer will not create content based on Google Maps Content", with examples including "(iv) use latitude/longitude values from the Places API as an input for point-in-polygon analysis" and "(vii) use Google Maps Content to improve machine learning and artificial intelligence models, including to train, test, validate or fine-tune the models" | ToS §3.2.3(c) |

Other clauses that matter for a planner:

- **No directory.** Customer will not "(iii) use the Google Maps Core Services in a listings or directory service" (ToS §3.2.3(d)). "Google Maps Core Services" means every Maps Platform service (ToS definitions). A browsable catalogue of venues built on Places could be read as a directory; a plan that names its venues is less clearly one. **Not tested; a judgement call.**
- **No non-Google map.** Places and Routes content must not be used "in conjunction with a non-Google map" (SST §§14.2, 19.2). Places content can be shown without any map (§14.1). So OSM base maps and Google venue names can't share a screen.
- **The policies page says the same about IDs:** "You can therefore store place ID values indefinitely" ([Places policies](https://developers.google.com/maps/documentation/places/web-service/policies)); "Store exempt identifiers such as the place ID to fetch fresh details" ([Routes policies](https://developers.google.com/maps/documentation/routes/policies)).

### Checking v1's reading in `docs/google-maps.md` §5

| v1 says | Current terms | Verdict |
|---|---|---|
| Place IDs may be stored indefinitely | SST §3 | **Still right** |
| Names, addresses, hours, ratings, photos not stored, fetched live | ToS §3.2.3(a)(iii), (b) | **Still right** |
| Place locations "used in memory to assign each venue to its area (the one whose boundary contains it, or the nearest), and are never written to the database" | ToS §3.2.3(c)(iv) names "point-in-polygon analysis" on Places lat/lng as forbidden *content creation*, whether or not the coordinates are stored. The stored area is content based on Google Maps Content | **Wrong.** The in-memory detail doesn't help. v2 needs area assignment from a non-Google source (OSM, Overture or Foursquare coordinates for the same place), or from people |
| Travel times (Routes content) cached for at most 30 days | The SST permits caching only lat/lng from Routes. Durations and distances have no caching permission | **Wrong as the terms stand now.** v1 may have followed an older version (not checked). The matrix and the plan routes (`app.plans.route_*`, `app.plan_stops.walk_seconds`) would have to be fetched live, or come from a non-Google router (e.g. OSRM or Valhalla on OSM data) |
| Nothing from Google given to the model; benchmark keeps only its outcome; ML clause | ToS §3.2.3(c)(vii) unchanged | **Still right.** Note that comparing the model's place name with Google's in memory (walks) is close to "create content based on"; v1 judged it a check, not content |
| Gemini Grounding with Google Maps: shown only to the asker, not stored beyond 90 days | [Gemini API Additional Terms](https://aistudio.google.com/docs/terms) (effective 2026-03-23): "for up to ninety (90) days, only to evaluate and optimize the display", plus "in the chat history of an end user in your application for up to six (6) months" | **Right, and there is now a six-month chat-history allowance** |
| Attribution: text "Google Maps", 12sp, #5E5E5E on light, white on dark, "the only colours Google allows" | "Attribution should take the form of the Google Maps logo whenever possible"; text when space is limited; colours "White, black (#1F1F1F), or gray (#5E5E5E)"; don't translate (`translate="no"`) ([Places policies](https://developers.google.com/maps/documentation/places/web-service/policies)) | **Mostly right.** The logo is preferred, black #1F1F1F is also allowed, and the 12sp size wasn't found in the current page (**unverified**) |
| Reviews/photos | "You must always credit the author when displaying photos or reviews" (same page) | v1 showed neither. Still applies if v2 shows them |

### New since v1: Maps Experience Agent Tools

SST §10 adds a Google-sanctioned way to ground any LLM on Maps:

- Permitted: "Customer may use Maps Experience Agent Tools to ground a LLM to generate and display Grounded Output to End Users if (i) Customer complies with the Generative AI Prohibited Use Policy … and (ii) Customer includes associated Google Maps source links with the Grounded Output" (§10.2.1).
- Storage: "Customer may cache Grounded Output: (1) for up to thirty (30) consecutive days for the purpose of evaluating and optimizing the performance or display … or (2) in the chat history of an End User … for up to two (2) years" (§10.2.2).
- Banned: "attempt to extract or otherwise separate Google Maps Content from the Grounded Output", "use Grounded Output to train, develop, or improve any machine learning models or artificial intelligence systems", and "modify or intersperse content with the Grounded Output" (§10.3).

So it can make a live answer for one person, not a stored plan for many.

## Other sources

| Source | What its terms say | May Issho keep it? |
|---|---|---|
| **Foody.vn** ([terms](https://www.foody.vn/dieu-khoan-su-dung)) | Location information, comments and images "đều thuộc quyền sở hữu của Foody" ("all belong to Foody"). "Trừ phi được sự đồng ý của FOODY, bạn không được phép tải lên, gửi, xuất bản, tái sản xuất" ("unless Foody consents, you may not upload, send, publish, reproduce"). No commercial use without Foody's prior written permission. No clause names bots or scraping | No, without written consent. No public API ([local recommendation sources](https://github.com/shrujancheruku/issho/blob/research/local-recommendation-sources/docs/research/local-recommendation-sources.md)) |
| **Facebook / Meta** ([terms](https://www.facebook.com/legal/terms?locale=en_US), effective 2025-01-01; [Automated Data Collection Terms](https://www.facebook.com/apps/site_scraping_tos_terms.php), effective 2024-10-07) | "You may not access or collect data from our Products using automated means (without our prior permission)" (§3.2). Even with permission: "The Collected Data shall only be Used for the purpose of (i) providing results for your Search Engine" | No. Meta's permission is for search engines only |
| **TikTok** ([terms, rest of world](https://www.tiktok.com/legal/page/row/terms-of-service/en), updated 2025-12-01) | You may not "use automated scripts to collect information from or otherwise interact with the Services" or "bypass any measures we may use to prevent or restrict access" (§5) | No |
| **Zalo** | The terms page ([zalo.me/zalo/dieukhoan](https://zalo.me/zalo/dieukhoan/)) rendered only its heading. **Unverified.** Recommendations there travel in private chats and groups, which no API exposes | No route found |
| **Tabelog** ([rules](https://tabelog.com/help/rules/), revised 2026-02-17) | 「食べログへ投稿された口コミを無断転載・無断利用することは禁止します。」 ("reposting or using reviews posted to Tabelog without permission is prohibited", §6(1)); no copying, reproduction, distribution or resale of information without prior consent (§9(1)); no access or use for commercial purposes (§6(1)) | No |
| **Yelp** ([Terms of Service](https://terms.yelp.com/tos/en_us/20200101_en_us/)) | Don't "use any robot, spider, Service search/retrieval application, or other automated device … to access, retrieve, copy, scrape, or index any portion of the Service", or index it "for purposes of constructing or populating a searchable database" (§7.2). Via the paid API: "You may cache Yelp Fusion API content for a maximum of 24 hours", business IDs indefinitely ([Fusion FAQ](https://yelp-developers.readme.io/docs/fusion-faq)) | Only IDs; content for 24 hours via the paid API. A ban on feeding Yelp content to generative AI is reported (**unverified**, [third-party summary](https://vorplabs.com/agent-tools/yelp-ai-api)) |
| **Tripadvisor Content API** ([master terms](https://tripadvisor-content-api.readme.io/reference/terms-of-use), 2022-12-12, titled "Old") | "Customer may only cache the Licensed Content as explicitly permitted by the Tripadvisor Caching Policy" (§3.4.3); no displaying it "alongside or in conjunction with other user-generated content" (§3.5.1); no use "for … any purpose other than the display of the Licensed Content" (§6.4(a)); displays must be "non-indexable by search engines" (§3.4.3). The caching policy itself wasn't read (**unverified**) | Display only. The ban on mixing with other user content clashes with a planner that blends sources |
| **OpenStreetMap** ([ODbL 1.0](https://opendatacommons.org/licenses/odbl/1-0/), [attribution guidelines](https://osmfoundation.org/wiki/Licence/Attribution_Guidelines), edited 2026-09-10) | Store and use freely. Publicly using a **Produced Work** (a plan, a map, a text) needs a notice that content came from the database (§4.3): "Attribution must be to "OpenStreetMap"", ideally linked to openstreetmap.org/copyright. A **Derivative Database** that is publicly used (including through a Produced Work made from it) must be under the ODbL (§4.4) and offered in machine-readable form (§4.6). Search results and routing instructions need not carry attribution "as long as it does not form a Derivative Database" | Yes. Keep Issho's own data in a separate database from OSM, or accept share-alike for the merged one |
| **Overture Maps Places** ([guide](https://docs.overturemaps.org/guides/places/)) | "CDLA Permissive 2.0 and Apache 2.0 licenses, depending on the source"; no OSM data. Joining it with OSM "may need to carry the Open Database License (ODbL) if it is a derivative database" | Yes, with per-source attribution |

## Search-and-cite versus scrape-and-keep

**In the terms.** Every review and social site above draws the line at *copying, collecting by automated means, and commercial reuse*, not at reading. A link and a short reference to what a page says is the use the open web invites. Two exceptions push the other way: Tripadvisor allows only display under its rules, and Google's Maps terms bind Issho as an API customer whether or not anything is stored.

**In law, Vietnam first.**

- **Facts and data are outside copyright.** [Law on Intellectual Property No. 50/2005/QH11, Art. 15](https://dulieuphapluat.vn/van-ban/so-huu-tri-tue-van-ban/law-on-intellectual-property-2005-998276.html): not protected are "News of the day as mere items of information" and "Processes, systems, operational methods, concepts, principles and data."
- **Compilations are protected only for creative selection or arrangement, and never the data itself.** Art. 22(2): "Data collection means a set of data selected or arranged in a creative way"; "Copyright protection of data collections shall not extend to protection of the data itself." No sui generis database right was found in the law (an inference from its structure; **confirm with a Vietnamese lawyer**). The English text read is the 2005 original; amendments of 2009, 2019 and 2022 weren't compared article by article (**unverified** that Arts. 15 and 22 are unchanged).
- **Fair-use exceptions are narrow and listed** (Art. 25(1): one copy for research or teaching, quotation for comment, news, teaching, libraries, and so on). Commercial copying of review text is not among them.
- **AI and text-and-data mining.** Law No. 131/2025/QH15, passed 2025-12-10 and in force from 2026-04-01, adds a clause 5 to Art. 7 allowing use of lawfully published, publicly accessible texts and data for research, testing and AI training, "với điều kiện việc sử dụng này không ảnh hưởng bất hợp lý đến quyền và lợi ích hợp pháp của tác giả" ("provided this does not unreasonably affect the authors' legitimate rights") ([Luật Việt An](https://luatvietan.vn/luat-sua-doi-luat-so-huu-tri-tue-2025.html)). Decree 134/2026 reportedly limits it to "non‑commercial purposes at the point of use", requires lawful access without circumventing protection measures, lets rights holders opt out, and may impose royalties when a trained system is exploited commercially ([Baker McKenzie](https://www.bakermckenzie.com/en/insight/publications/2026/05/vietnam-redefining-copyright-for-ai)). **Unverified at the official text** (thuvienphapluat returned 403). Issho fetches recommendations to make plans, it doesn't train models, so this exception probably doesn't apply either way.
- **Personal data.** Reviews and posts carry their authors' names and faces. [Personal Data Protection Law No. 91/2025/QH15](https://english.luatvietnam.vn/dan-su/law-on-personal-data-protection-law-no-91-2025-qh15-405135-d1.html) (in force 2026-01-01): "Personal data may be collected with the prior consent of a personal data subject, unless otherwise prescribed by law" (Art. 11(1)); others may analyse personal data only from sources the law permits (Art. 11(3)); "Buying and selling personal data" is prohibited (Art. 7(6)). Publicly posted is not the same as consented. Keeping reviewer names or profiles is the riskiest thing scraping could do in Vietnam; keeping anonymous facts about a *place* is not personal data.
- **Contract.** Whether Vietnamese courts enforce a site's browse-wrap terms against a visitor who never signed in was not researched (**unverified**).

**EU in outline.**

- **Database right.** The maker of a database with "a substantial investment in either the obtaining, verification or presentation" of its contents may prevent "extraction and/or re-utilization of the whole or of a substantial part"; and "The repeated and systematic extraction and/or re-utilization of insubstantial parts … shall not be permitted" where it conflicts with normal exploitation ([Directive 96/9/EC, Art. 7(1), (5)](https://www.legislation.gov.uk/eudr/1996/9/article/7)). A review site's database is the textbook case.
- **Search engines can infringe it.** In *CV-Online Latvia v Melons* (C-762/19, 2021-06-03), a specialist search engine copying and indexing a freely accessible job database could infringe where it harms the maker's investment ([IPPT copy](https://www.ippt.eu/sites/ippt/files/2021/IPPT20210603_CJEU_CV-Online_Latvia_v_Melons.pdf); CURIA not read).
- **Contract fills the gap.** In *Ryanair v PR Aviation* (C-30/14, 2015-01-15), the maker of a database protected by neither copyright nor the database right may still restrict its use by contract (paraphrase, via [IPPT](https://www.ippt.eu/sites/ippt/files/2015/IPPT20150115_ECJ_Ryanair_v_PR_Aviation.pdf) and commentary; CURIA not read).
- **Text and data mining** is allowed for "reproductions and extractions of lawfully accessible works" unless rights holders reserve it "in an appropriate manner, such as machine-readable means" ([Directive 2019/790, Art. 4](https://www.legislation.gov.uk/eudr/2019/790/article/4)). It covers analysis, not keeping or republishing the text.

**Japan in outline.** A database is a work only if "by reason of the selection or systematic construction of information contained therein, constitutes a creation" ([Copyright Act Art. 12-2](https://www.cric.or.jp/english/clj/cl2.html)); there is no separate database right. Use for "data analysis" is allowed unless it "would unreasonably prejudice the interests of the copyright owner" (Art. 30-4), and a search or analysis service may show minor parts of works with its results (Art. 47-5). Data held behind access limits is protected as "shared data with limited access" under the [Unfair Competition Prevention Act](https://www.japaneselawtranslation.go.jp/en/laws/view/4709/en); open data is outside it (from METI summaries, **unverified** at the article). Tabelog's contract terms still bind its users.

**US in outline.** Facts are free: "In no event may copyright extend to the facts themselves", and "originality, not 'sweat of the brow,' is the touchstone" ([*Feist v. Rural*, 499 U.S. 340 (1991)](https://www.law.cornell.edu/supremecourt/text/499/340)). Scraping a public site without a login is not "without authorization" under the CFAA ([*hiQ v. LinkedIn*, 9th Cir. 2022](https://law.justia.com/cases/federal/appellate-courts/ca9/17-16783/17-16783-2022-04-18.html), from summaries). Meta's terms "do not apply to and prohibit Bright Data's scraping of publicly available data while logged off" (*Meta v. Bright Data*, N.D. Cal. 2024-01-23, a district court, via [Farella Braun](https://www.fbm.com/data-analytics/publications/major-decision-affects-law-of-scraping-and-online-data-collection-meta-platforms-v-bright-data/); opinion not read). Copyright in review text and photos stays with their authors either way.

**What this means.** Searching, reading and linking is safe nearly everywhere. Keeping copies of review text, photos or ratings is not, by contract and (for text and photos) by copyright. Bulk-extracting a review site's database is a database-right problem in the EU even when every item is a fact.

## Attribution and display

- **Google:** logo, or "Google Maps" text untranslated in white, #1F1F1F or #5E5E5E; third-party data needs "Google Maps" plus the provider's name; author credit (avatar, name, profile link) on every review or photo; a notice explaining how reviews are ordered; a flag on machine-translated reviews ([Places policies](https://developers.google.com/maps/documentation/places/web-service/policies), [Routes policies](https://developers.google.com/maps/documentation/routes/policies)).
- **OpenStreetMap:** "OpenStreetMap", linked to its copyright page, wherever a Produced Work is publicly used (ODbL §4.3).
- **Tripadvisor:** its mark must accompany every display, logos served from Tripadvisor, pages not indexable.
- **Anthropic web search:** "When displaying API outputs directly to end users, citations must be included to the original source" ([web search tool](https://platform.claude.com/docs/en/agents-and-tools/tool-use/web-search-tool)). The same note, with "include", is on the [web fetch tool](https://platform.claude.com/docs/en/agents-and-tools/tool-use/web-fetch-tool).
- **Gemini grounding:** show Search Suggestions or Google Maps Links with the answer, unmodified ([Gemini API terms](https://aistudio.google.com/docs/terms)).

## Facts drawn from a source

| Kind of fact | Keep it? | Why |
|---|---|---|
| A place's existence, name and location, from open data (OSM, Overture) | Yes | Licence allows; attribute |
| Same, from Google Places | ID only | ToS §3.2.3(a)(iii), SST §14.3 |
| "Good for groups", "quiet in the afternoon", "the path along the river is pleasant", written by Issho from a person's observation (the owner, a trusted local, a participant's feedback) | Yes | Issho's own content |
| The same fact, summarised from blogs, Foody or Facebook posts that Issho *read* (by a person or a model) and recorded in Issho's own words, with a link | Yes in law (facts and data are outside copyright in Vietnam and the US; one fact is not a substantial part of an EU database). Contract risk is low when nothing was copied or collected in bulk, but each site's ban on "commercial use" of its information is drafted widely enough to argue (**unresolved**) | Vietnam IP Art. 15(3); *Feist*; Directive 96/9 Art. 7 |
| The same fact derived from **Google** reviews, ratings or attributes | No | "Customer will not create content based on Google Maps Content" (ToS §3.2.3(c)) |
| Systematically derived from a whole review site (e.g. a score for every venue computed from Foody ratings) | No | Repeated, systematic extraction (EU Art. 7(5)); Foody's ban on reproduction and commercial exploitation |
| Review text, photos, star ratings, reviewer names | No | Copyright; terms; personal data (PDPL Art. 11) |

## Language models with web search

- **Anthropic** ([web search](https://platform.claude.com/docs/en/agents-and-tools/tool-use/web-search-tool), [web fetch](https://platform.claude.com/docs/en/agents-and-tools/tool-use/web-fetch-tool), [Commercial Terms](https://www.anthropic.com/legal/commercial-terms), effective 2025-06-17). Citations are "always enabled for web search" and required when output is shown to users. The customer "owns its Outputs". Anthropic's terms set no storage limit on search results, but owning the output doesn't clear the rights in what it quotes: the source sites' terms and copyright still apply to quoted text. Web fetch refuses URLs blocked by "Anthropic-side restrictions, such as private addresses, `robots.txt`", so it is a well-behaved fetcher, not a licence. Claude's own words summarising facts, with the source's URL, are the cleanest thing to keep.
- **Gemini, Grounding with Google Search** ([terms](https://aistudio.google.com/docs/terms)): show results only "to the end user who submitted the prompt", with Search Suggestions; don't "cache, frame, syndicate, resell, analyze, train on, or otherwise learn from Grounded Results"; text may be stored up to two years for evaluation and chat history, but not "used to build a database", and links may not be used "to build an index". This rules it out for building a shared catalogue.
- **Gemini, Grounding with Google Maps** and **Maps Experience Agent Tools**: per-person answers only, as above.
- **OpenAI** and others: not checked (**unverified**).

The pattern that fits all of them: a model searches and reads at planning time, and Issho keeps only its own words (facts, judgements) plus the source URL. It never keeps the retrieved text, and never anything from Google beyond place IDs.

## For "Where plan knowledge comes from"

1. **v1's area assignment and travel-time cache break the current Google terms** (point-in-polygon, ToS §3.2.3(c)(iv); no caching of Routes durations, SST §19.3). v2 needs coordinates and travel times from open data (OSM or Overture plus an open router) for anything it stores, and Google only for live display.
2. **Google Places is a live lookup and an ID, nothing more.** It can't seed stored building blocks: their facts must come from people or open data.
3. **Review sites have no lawful bulk route in Vietnam.** Foody has no API and forbids commercial reuse; Facebook, TikTok and Zalo have no read API. Local recommendations can enter Issho only through people (the owner, trusted locals, participants) or as search-and-cite at planning time.
4. **Tripadvisor's API can't be blended** with other user content, which suits a planner badly.
5. **Keep Issho's own data apart from OSM's** if v2 wants to avoid share-alike on its catalogue.
6. **Don't store reviewer identities.** Vietnam's PDPL makes that the main legal risk of any collection.

## Not verified here

- The official Vietnamese text of Law 131/2025/QH15 and Decree 134/2026 (403s); whether IP Law Arts. 15, 22 and 25 were changed by the 2009–2022 amendments.
- Enforceability of browse-wrap terms in Vietnam; criminal-law exposure for scraping (e.g. unauthorised access offences); Vietnam's Cybersecurity Law.
- Zalo's terms; Tripadvisor's Caching Policy and Display Requirements; Yelp's current API terms (the 2024 URL tried returned 404) and its reported AI ban; OpenAI's web search terms.
- The judgments in *CV-Online*, *Ryanair*, *hiQ* and *Meta v. Bright Data* (read through summaries); Japan's Unfair Competition Prevention Act article text.
- Whether Google's consumer terms bind a person who reads Google Maps in a browser and then writes their own note (relevant to trusted locals who use it).
- The 12sp minimum size for Google's text attribution.
