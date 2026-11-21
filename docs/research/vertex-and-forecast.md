# Models on Vertex AI, and forecast sources

Research for [#572](https://github.com/shrujancheruku/issho/issues/572) (Architecture and stack). The owner has chosen Google Cloud products, with models called through Vertex AI so they can be swapped and tested. This note checks what that choice gives the four model uses and the weather lookup in the v2 spec ([docs/spec/](../spec/README.md)). It decides nothing.

- **Sources checked:** 2026-11-21, unless a line says otherwise. Each claim links its primary source.
- **Unverified:** marked **(unverified)**, and gathered at the end.
- **Not legal advice:** the terms-of-service parts are research, not legal advice.
- **Earlier notes:** this one replaces the first-party Anthropic API assumption in [v2-stack-asks](https://github.com/shrujancheruku/issho/blob/research/v2-stack-asks/docs/research/v2-stack-asks.md) (2026-11-20). [scraping-terms-and-law](https://github.com/shrujancheruku/issho/blob/research/scraping-terms-and-law/docs/research/scraping-terms-and-law.md) (2026-11-12) covers Google Maps Platform terms and the Gemini Developer API grounding terms. [vietnam-phone-verification](https://github.com/shrujancheruku/issho/blob/research/vietnam-phone-verification/docs/research/vietnam-phone-verification.md) (2026-11-07) covers SMS in Vietnam.

**Vertex AI has been renamed.** At Cloud Next '26 (2026-04-22), Vertex AI became the **Gemini Enterprise Agent Platform**. Google says "all Vertex AI services and roadmap evolutions will be delivered exclusively through the Agent Platform" ([blog](https://cloud.google.com/blog/products/ai-machine-learning/introducing-gemini-enterprise-agent-platform)). Other names that changed:
- The Vertex AI API is now the Agent Platform API.
- Vertex AI Search is now Agent Search.
- Model Garden keeps its name.

The API host is still `aiplatform.googleapis.com`, and old doc links redirect. This note says "Vertex" for short.

**The four model uses in the spec:**

| # | Use | Personal data? | What the spec requires |
|---|---|---|---|
| 1 | Plan judge | No | none beyond the general rules |
| 2 | Plan translation, vi↔en, stored at approval | No | none beyond the general rules |
| 3 | Lead search: a web search; Issho keeps only its own note, the source links and a Google place ID | No | The search tool's terms must allow keeping that. The spec says this rules out "Grounding with Google Search". |
| 4 | Moderator's translation of reports, chat and feedback | **Yes** | One service only, under terms that forbid training on the data |

Two general rules also apply: no other personal data goes to a model, and nothing stored may be derived from Google Maps content.

---

## Summary

### A. Models on Vertex AI

| Question | Decisive fact | Source |
|---|---|---|
| What can be called | Gemini (4 Neon, 4 Hydrogen, 4 Carbon, 4 Argon, 3.8, 3.7 and 3.6 Flash, 3.5 Flash and Flash-Lite, and older). Claude (Fable 5.x, Opus 5.5, Sonnet 5.5, Haiku 5.5 and older). Also Grok, Mistral, and open models as a service (DeepSeek, Gemma 4, gpt-oss, Llama 4, Qwen, GLM). | [models](https://docs.cloud.google.com/gemini-enterprise-agent-platform/models) |
| One API for all? | **No.** Gemini uses `generateContent` (Gen AI SDK for Go). Claude uses `rawPredict` with the Anthropic Messages body (Anthropic Go SDK, `vertex` package). The OpenAI-compatible endpoint covers Gemini and self-deployed or open models, not Claude. | [openai overview](https://docs.cloud.google.com/gemini-enterprise-agent-platform/models/migrate/openai/overview), [Claude on Google Cloud](https://platform.claude.com/docs/en/build-with-claude/claude-models-on-google-cloud/google-managed) |
| Claude: two offerings | **Google managed:** Model Garden, billed and governed by Google. Its web search is basic (`web_search_20250305` only); it has no web fetch and no Message Batches API. **Anthropic managed:** `claude.googleapis.com`, sold through Marketplace. It has the full Claude API, and "Anthropic is the data processor". | [overview](https://platform.claude.com/docs/en/build-with-claude/claude-models-on-google-cloud/overview) |
| Training | SST §18: "Google will not use Customer Data to train or fine-tune any AI/ML models without Customer's prior permission or instruction." This applies to "all managed models on Gemini Enterprise Agent Platform". Claude on Vertex is also governed by Anthropic's Commercial Terms, which say "Anthropic may not train models on Customer Content". | [SST](https://cloud.google.com/terms/service-terms), [ZDR page](https://docs.cloud.google.com/gemini-enterprise-agent-platform/resources/zero-data-retention), [abuse monitoring](https://docs.cloud.google.com/gemini-enterprise-agent-platform/models/abuse-monitoring) |
| Retention you cannot switch off (online-terms customer) | Gemini: prompts flagged by abuse classifiers are logged for up to 90 days, with human review. Gemini 4 Carbon and the cyber models: all prompts and responses logged for 30 days. Claude Fable 5 and Mythos 5: logged for 30 days and shared with Anthropic. Grounding with Google Search: queries logged for 3 days. Maps grounding: 30 days. | same |
| Lead search, Google grounding | Grounding with Google Search, Web Grounding for Enterprise and Maps grounding forbid caching, storing (except for chat history or evaluation) and "using Links to build an index". **None fits use (3).** | [SST §20(k)–(m)](https://cloud.google.com/terms/service-terms) |
| Lead search, other tools | Anthropic web search on Vertex: $10 per 1,000 searches. The docs require only citations when shown to end users; no storage limit was found. Brave: no storing "other than transient", unless on a custom plan with "storage rights". Parallel: output "shall not be copied, cached, stored, or made available to other End Customers". Tavily: no storage clause found. Exa: terms not read **(unverified)**. | below |
| Asia regions | Singapore (`asia-southeast1`) serves Gemini 4 Argon and 3.5 Flash. For Claude it serves only Opus 4.6 and 4.5, Sonnet 4.6 and 4.5, and Claude 3 Haiku. Everything else is on `global` (no residency) or `us`/`eu`. | [locations](https://docs.cloud.google.com/gemini-enterprise-agent-platform/resources/locations), [data residency](https://docs.cloud.google.com/gemini-enterprise-agent-platform/resources/data-residency) |
| Translation | Cloud Translation's data FAQ says it "does not use the content you send to train and improve our Google Translation features" and "text is held briefly in-memory". It supports vi↔en, with NMT and the Translation LLM, at $20 or $10+$10 per 1M characters. It has only global, EU and US endpoints. | [data usage](https://docs.cloud.google.com/translate/data-usage), [pricing](https://cloud.google.com/products/translate/pricing) |
| Prices (per 1M tokens, in/out, global) | Gemini 3.5 Flash-Lite $0.30/$2.50. 3.8 Flash $0.75/$3.75 until 2026-12-31, then $1.50/$7.50. 4 Hydrogen $1.50/$7.50. 4 Argon $4/$20 from 2026-12-01. Claude Haiku 5.5 $0.10/$0.50 (as listed). Sonnet 5.5 $2/$10. Opus 5.5 $4/$20. Google Search grounding: 5,000 free per month, then $14 per 1,000 queries. | [Gemini pricing](https://cloud.google.com/vertex-ai/generative-ai/pricing), [partner pricing](https://cloud.google.com/model-garden/partner-models/pricing) |

### B. Weather

| Question | Decisive fact | Source |
|---|---|---|
| Google Weather API in Vietnam | **No forecasts for Vietnam.** The coverage table marks Vietnam "---" (unsupported) for current conditions and for daily, hourly and history forecasts. It marks only weather alerts as supported, sourced from Vietnam's NCHMF. | [coverage](https://developers.google.com/maps/documentation/weather/coverage), [alerts](https://developers.google.com/maps/documentation/weather/weather-alerts) |
| Google Weather API terms (where it works) | Hourly forecast values may be cached for 1 hour, daily values for 24 hours, and alerts for 1 hour. "May not use … to recreate a Google service". Price: 10,000 free events per month, then $0.15 per 1,000. | [Maps SST §21](https://cloud.google.com/maps-platform/terms/maps-service-terms), [pricing](https://developers.google.com/maps/billing-and-pricing/pricing) |
| Open-Meteo | Up to 16 days of forecast, with hourly `precipitation_probability`. Data is under CC BY 4.0, and the server is AGPL-3.0, so it can be self-hosted. The free API is for non-commercial use only; commercial use needs a paid plan (1M calls per month on Standard; price not shown). | [docs](https://open-meteo.com/en/docs), [terms](https://open-meteo.com/en/terms), [pricing](https://open-meteo.com/en/pricing) |
| Vietnam's national service | No public developer API found **(unverified)**. Its alerts reach developers through Google's alert endpoint. | search, [alerts](https://developers.google.com/maps/documentation/weather/weather-alerts) |
| OpenWeather One Call 3.0 | Hourly forecast for 48 hours and daily for 8 days, with `pop` (probability of precipitation) and government alerts. 1,000 free calls per day. **Daily stops at 8 days**, short of about 9. | [one call 3](https://openweathermap.org/api/one-call-3) |

### C. Google equivalents

| v1 part | Google equivalent | Gain or loss |
|---|---|---|
| Supabase Postgres | Cloud SQL for PostgreSQL (or AlloyDB) in `asia-southeast1` | Same engine, so goose and sqlc carry over. Loses Supabase's bundled auth and dashboard. AlloyDB is overkill at this size. |
| Supabase Auth (email OTP) | Identity Platform / Firebase Auth | **No email-code sign-in**; Firebase offers an email *link*. A 6-digit email code would need custom tokens and our own mailer. 50,000 MAU free per month. |
| Resend email | No Google transactional email product | Keep Resend (or another provider). Firebase sends only its own auth emails. |
| Expo push | FCM direct (APNs for iOS through FCM) | Removes Expo's relay. Needs native FCM setup in the Expo app. Free. |
| Sentry (EU) | Error Reporting plus Cloud Logging | Same project and IAM. Weaker mobile crash grouping and release tracking than Sentry; Firebase Crashlytics covers native crashes. |
| Phone codes | Identity Platform / Firebase phone auth | Vietnam is listed at **$0.13 per SMS**. Delivery quality in Vietnam is not confirmed here **(unverified)**; see the phone-verification note. |

---

## A. Models on Vertex AI

### A1. Which models, and how they are called

**Model Garden** lists "more than 200" models ([models](https://docs.cloud.google.com/gemini-enterprise-agent-platform/models), [Google models](https://docs.cloud.google.com/gemini-enterprise-agent-platform/models/google-models)).

| Family | On Vertex | Called by |
|---|---|---|
| Gemini (Google) | GA: 4 Neon, 4 Hydrogen, 4 Carbon, 4 Argon, 3.8 Flash, 3.7 Flash, 3.6 Flash, 3.5 Flash, 3.5 Flash-Lite, 3.1 Flash-Lite, 2.5 Pro/Flash/Flash-Lite. Preview: 3.1 Pro, 3 Flash. Vietnamese is in the supported-language list. | `generateContent`; Interactions API; OpenAI-compatible Chat Completions |
| Claude (Anthropic, partner) | Fable 5.2, 5.1, 5; Opus 5.5, 5, 4.8, 4.7, 4.6, 4.5; Sonnet 5.5, 5, 4.6; Haiku 5.5, 4.5. Mythos 5 is limited. | `rawPredict` / `streamRawPredict` with an Anthropic Messages body |
| Other partners | Grok; Mistral (Medium 3, Small 3.1, OCR, Codestral 2) | their own surfaces |
| Open models as a service | DeepSeek-V4.1-Flash, GLM 5.2, gpt-oss 120B, Llama 4 Maverick/Scout, Gemma 4, Qwen | mostly Chat Completions |

**How Gemini and Claude calls differ:**

| | Gemini | Claude (Google managed) |
|---|---|---|
| Endpoint | `https://aiplatform.googleapis.com/v1/projects/P/locations/global/publishers/google/models/M:generateContent` | `…/publishers/anthropic/models/claude-opus-5-5:rawPredict` |
| Go SDK | Google Gen AI SDK, `client.Models.GenerateContent(ctx, model, contents, config)` | `github.com/anthropics/anthropic-sdk-go/v2`, with `vertex.WithGoogleAuth(ctx, "global", "PROJECT")` |
| Request shape | Gemini `contents` / `generationConfig` | The Anthropic Messages body. The `model` goes in the URL and `anthropic_version` in the body (`vertex-2023-10-16` or `2026-10-21`). |
| Auth | Application Default Credentials; `roles/aiplatform.user` | the same (`aiplatform.endpoints.predict`) |
| Structured output | `responseSchema` / JSON mode | structured outputs supported ([Claude on Google Cloud](https://platform.claude.com/docs/en/build-with-claude/claude-models-on-google-cloud/google-managed)) |
| Quotas | Google Cloud quotas | Google Cloud quotas (Google managed). Anthropic sets the limits on the Anthropic-managed offering. |
| Enabling | on by default with the API | Enable the model card in Model Garden. Partner models are a "Separate Offering" under SST §19 ([Claude page](https://docs.cloud.google.com/gemini-enterprise-agent-platform/models/partner-models/claude)). |

**Swapping models.** No single API surface covers both families. The Chat Completions endpoint supports "both Gemini models and select self-deployed models from Model Garden", plus some open models as a service, but not Claude ([openai overview](https://docs.cloud.google.com/gemini-enterprise-agent-platform/models/migrate/openai/overview)). Swapping Gemini for Claude therefore means a thin interface in Issho's own code with two adapters. Both adapters use the same Google credentials and billing.

**Claude's two Google offerings** ([overview](https://platform.claude.com/docs/en/build-with-claude/claude-models-on-google-cloud/overview)):

| | Google managed (Model Garden) | Anthropic managed |
|---|---|---|
| Host | `aiplatform.googleapis.com` | `claude.googleapis.com/v1/projects/{p}/locations/global/workspaces/{ws}/invoke/v1/messages` |
| Setup | enable the model card | Subscribe on Marketplace. Create an Anthropic organisation, giving name, "entity type", country and intended use. Accept Anthropic's Commercial Terms. |
| Data | "Google Cloud governs data handling" | "Anthropic is the data processor … Some supporting services, such as Anthropic's trust and safety systems, process data outside Google Cloud." |
| Web search | basic `web_search_20250305` only | all versions, plus web fetch |
| Batch, Files API, code execution | Message Batches API, Files API and code execution are not supported. Vertex batch prediction pricing is listed. | supported |
| Regions | global, `us`/`eu`, and some regional endpoints | `global` only, with `inference_geo: "us"` as an option |
| Price | Google SKU; multi-region and regional endpoints "cost 10 percent more than global" | Claude Consumption Units at $0.01 each |

**Individual or company.** Google's Claude page says nothing about whether an individual's billing account can enable partner models. It doesn't say they can't **(unverified)**. The Anthropic-managed offering asks for an "entity type", and whether an individual can be one is not stated **(unverified)**.

### A2. Web search for the lead search (use 3)

**What use (3) needs.** Issho stores its own note, the source links and a Google place ID, and keeps them as a lead that later becomes a plan seen by many people.

#### Google's own grounding (Service Specific Terms, last modified 2026-11-18)

**Grounding with Google Search**, SST §20(k)(i) ([SST](https://cloud.google.com/terms/service-terms)):
- (1): Customer "will only display the Grounded Results with the associated Search Suggestion(s) to the End User who submitted the prompt".
- (2): Customer "will not … cache, frame, syndicate, resell, analyze, train on, or otherwise learn from Grounded Results or Search Suggestions." It also says: "it is a violation of these terms to use Grounding with Google Search to extract or collect one or more of these components for another purpose (for example, using programmatic or automated means to collect Links, using Links to build an index, or using Links to identify destination pages for crawling or scraping)."
- (3): no "copy, store" except:
  - for up to 2 years, "only to evaluate and optimize the display", or "in the chat history of an End User … only for the purpose of allowing that End User to view their chat history";
  - "for the minimum time necessary to comply with applicable law";
  - end users may save single results, but not so they are "used to create a database".
- (4): no modifying or interspersing other content with Grounded Results.
- **§20(k)(ii):** Google logs "queries derived from End User prompts" for up to 3 days for debugging. The ZDR page says: "There is no way to disable the storage of this information" ([ZDR](https://docs.cloud.google.com/gemini-enterprise-agent-platform/resources/zero-data-retention)).
- **Display rules** ([grounding page](https://docs.cloud.google.com/gemini-enterprise-agent-platform/models/grounding/grounding-with-google-search)):
  - Search Suggestions are shown "exactly as provided".
  - "Whenever a grounded response is shown, its corresponding Search Suggestion should remain visible."
  - Tapping a suggestion goes straight to Google's results page.
  - Pricing lets a customer skip Search Suggestions "at standard pricing for under 1 million Grounding Prompts per day" ([pricing](https://cloud.google.com/vertex-ai/generative-ai/pricing)).

**Vertex compared with the Gemini Developer API.** The core restrictions match the Developer API terms quoted in the scraping note (show only to the prompter, no cache, no index, 2-year storage for evaluation and chat history). Vertex adds three things: a carve-out for legal compliance, the 3-day debug log, and the Gemini Enterprise UI variant (§20(k)(v)). Neither allows a stored lead database.

**Web Grounding for Enterprise**, SST §20(l): "Section 20(k) (Grounding with Google Search) also applies to Web Grounding for Enterprise". The only exceptions are the names, "GCP's web index", and that the 3-day debug log (k)(ii) "is deleted". The page says "The service doesn't log customer data" ([page](https://docs.cloud.google.com/gemini-enterprise-agent-platform/models/grounding/web-grounding-enterprise)). The storage limits are the same, so this fails use (3) for the same reason.

**Grounding with Google Maps**, SST §20(m):
- Results are shown "to the End User who initiated the prompt".
- No caching except for up to 90 days to tune display, or up to 6 months in that user's chat history.
- "Google and its content providers retain all rights to Google Maps Data".
- Google stores prompts and output for 30 days.

It also conflicts with the spec rule against storing anything derived from Maps content.

#### Other tools Vertex offers for Gemini

| Tool | What it is | Terms for storing a note plus links |
|---|---|---|
| Grounding with Parallel | A "Separate Offering". Google sends "queries derived and rewritten from the original user prompt" to Parallel. Subscribe through Marketplace, or bring your own key. A zero-data-retention listing exists. ([page](https://docs.cloud.google.com/gemini-enterprise-agent-platform/models/grounding/grounding-with-parallel)) | Parallel Customer Terms (effective 2026-11-10): "Customer Output generated from one query shall be primarily for the use of one End Customer only, and shall not be copied, cached, stored, or made available to other End Customers or other third parties". Output may not be used to "create databases". ([terms](https://parallel.ai/customer-terms)) **Conflicts** with a lead shown to many people, though "primarily" leaves room. |
| Grounding with Exa | A "Separate Offering" ([page](https://docs.cloud.google.com/gemini-enterprise-agent-platform/models/grounding/grounding-with-exa)) | Exa's terms are a PDF that could not be read here **(unverified)**. |
| Grounding with your search API | Gemini calls an endpoint you run, and your API returns snippets ([page](https://docs.cloud.google.com/gemini-enterprise-agent-platform/models/grounding/grounding-with-your-search-api)) | The terms are whatever your search provider's are. Pricing lists "Grounding with your data" at $2.50 per 1,000 prompts; whether that applies to this tool is **(unverified)**. |
| Function tool backed by a search API | Your code calls a search API and passes the results to Gemini or Claude as tool results | The provider's terms apply; see the next table. |

#### Search APIs behind a function tool

| Provider | Status and price | Storage terms |
|---|---|---|
| Google Programmable Search (Custom Search JSON API) | "closed to new customers"; existing users must move by 2027-01-01 ([overview](https://developers.google.com/custom-search/v1/overview)) | not available |
| Brave Search API | $5 per 1,000 requests, with $5 of free credit per month ([page](https://brave.com/search/api/)) | Terms (2026-09-01): no "store, cache, or create a database of Search Results, in whole or in part, other than transient storage" ([terms](https://api-dashboard.search.brave.com/terms-of-service)). FAQ: "Can I store results …? Yes, with a custom plan … search with storage rights." |
| Tavily | ([terms](https://www.tavily.com/terms), last updated 2026-05-04) | No storage or caching limit found. Tavily may use Customer Input "to provide and improve Tavily's services". |
| Exa | — | **(unverified)** |

#### Anthropic's web search on Vertex (Claude, Google managed)

- **Version:** only `web_search_20250305`, "without dynamic filtering" ([tool doc](https://platform.claude.com/docs/en/agents-and-tools/tool-use/web-search-tool)).
- **Price on Vertex:** "Web Search Request: $10 per 1000 searches", plus tokens ([partner pricing](https://cloud.google.com/model-garden/partner-models/pricing)).
- **Display:** "When displaying API outputs directly to end users, citations must be included to the original source." If outputs are modified or combined with other material, display citations "as appropriate based on consultation with your legal team."
- **Storage:** no storage, caching or index limit found in the tool doc. Use is governed by Anthropic's Commercial Terms ([abuse monitoring](https://docs.cloud.google.com/gemini-enterprise-agent-platform/models/abuse-monitoring)).
- **Search provider:** Anthropic doesn't name it **(unverified)**.
- **Where queries run:** whether search queries made from Google-managed Claude leave Google Cloud is not stated **(unverified)**.

**Options left open (A2):**
1. Claude on Vertex with Anthropic's web search. The only Vertex-native option whose docs set no storage limit.
2. Gemini or Claude with a function tool over a search API whose terms allow storage: Tavily as read, or Brave on a custom plan with storage rights.
3. Gemini grounding with Parallel or Exa. Parallel's one-end-customer clause needs a reading or a written exception; Exa needs its terms read.
4. Google Search grounding, Web Grounding for Enterprise or Maps grounding. These fail use (3) as worded.

### A3. Data terms

**Training.**
- Google: SST §18, "Google will not use Customer Data to train or fine-tune any AI/ML models without Customer's prior permission or instruction." It applies "to all managed models on Gemini Enterprise Agent Platform, including GA and pre-GA models" ([ZDR](https://docs.cloud.google.com/gemini-enterprise-agent-platform/resources/zero-data-retention)).
- Outside the account: SST §20(h), Google "will not store outside Customer's Account (i) Customer Data prompted to a Generative AI Service for longer than is reasonably necessary to create the Generated Output, or (ii) the Generated Output."
- Partner models: "Use of Anthropic models on Gemini Enterprise Agent Platform is governed by Anthropic's Commercial Terms of Service" ([abuse monitoring](https://docs.cloud.google.com/gemini-enterprise-agent-platform/models/abuse-monitoring)). Those terms say "Anthropic may not train models on Customer Content from Services." Partner models are a "Separate Offering": "subject to separate terms and conditions", and Google "disclaims all liability" for them (SST §19).
- Cloud Translation: "Google does not use the content you send to train and improve our Google Translation features" ([data usage](https://docs.cloud.google.com/translate/data-usage)).

**Does Anthropic receive the data?**
- Google managed: "Turning on this [logging] service doesn't give Google or Anthropic any access to your content" ([Claude on Google Cloud](https://platform.claude.com/docs/en/build-with-claude/claude-models-on-google-cloud/google-managed)).
- The exception: "For … Claude Fable 5 on Google Cloud and Mythos 5, prompts and responses are retained for up to 30 days … you must enable sharing this data with Anthropic for abuse monitoring" ([abuse monitoring](https://docs.cloud.google.com/gemini-enterprise-agent-platform/models/abuse-monitoring)).
- Anthropic managed: Anthropic processes the data, and some of it outside Google Cloud.

**Retention and logging:**

| What | Who is in scope | How long | Can it be turned off? |
|---|---|---|---|
| Abuse logging, Gemini (most models) | customers on the online Google Cloud Platform Terms; Master Agreement customers are exempt | Prompts flagged by classifiers, up to 90 days; Google staff may review them | Not by an online-terms customer, per the page |
| "Google Frontier Models" (Gemini 4 Carbon, the cyber models) | all | Prompts and responses, 30 days, kept in the customer's environment; human review for online-terms customers | no |
| "Advanced AI" partner models (Claude Fable, Mythos; Opus ≥4.7 and Sonnet ≥5 only in the cyber programme) | all | Prompts and responses, 30 days | "may not be possible" |
| Grounding with Google Search | all | Queries, 3 days | "no way to disable" |
| Grounding with Google Maps | all | Prompts and output, 30 days | "no way to disable" |
| Implicit caching (Gemini) | all | In memory and durable, deleted "no later than 24 hours"; "never used to train" | yes, per project (`cacheConfig`) |
| Request-response logging | opt-in | your BigQuery | off by default |
| Interactions API `store` | — | stored | defaults to `true`; set `false` |

Sources: [abuse monitoring](https://docs.cloud.google.com/gemini-enterprise-agent-platform/models/abuse-monitoring), [ZDR](https://docs.cloud.google.com/gemini-enterprise-agent-platform/resources/zero-data-retention).

**Is "terms that forbid training" enough for use (4)?** The spec asks for one service under no-training terms, and both Gemini and Cloud Translation meet that wording. What the wording leaves out:
- **Abuse logs.** Gemini prompts may be kept for up to 90 days and read by Google staff. Issho is an online-terms customer, so it is in scope. Cloud Translation's FAQ instead says the text is "held briefly in-memory".
- **Global endpoints.** A `global` endpoint gives no data residency (see A4).
- **Chosen model.** Picking Claude Fable would send the data to Anthropic as well.

Whether these matter is a privacy-policy question for #572.

**Options left open (A3):**
1. Use (4) on Cloud Translation. Its no-training and in-memory terms come from a non-generative API.
2. Use (4) on Gemini (Translation LLM or a prompt). The data is not used for training, but it may sit in abuse logs.
3. Ask whether a Master Agreement or abuse-logging exception is open to an individual **(unverified)**.

### A4. Regions

**What each endpoint promises** ([data residency](https://docs.cloud.google.com/gemini-enterprise-agent-platform/resources/data-residency)):
- Global: "Requests submitted to a `global` endpoint may be processed in any Google Cloud location around the world, and therefore don't provide any data residency guarantees."
- Regional and multi-regional: they keep ML processing in that jurisdiction (SST §16).
- Data at rest stays in the chosen location whichever endpoint is used.

**Availability** (from the [locations](https://docs.cloud.google.com/gemini-enterprise-agent-platform/resources/locations) tables):

| Model | global | `us` / `eu` | Singapore `asia-southeast1` | Other Asia-Pacific |
|---|---|---|---|---|
| Gemini 4 Argon | yes | yes | **yes** | — |
| Gemini 4 Neon / Hydrogen / Carbon | yes | mostly | no | — |
| Gemini 3.8 Flash | yes | yes | no | Tokyo |
| Gemini 3.5 Flash | yes | yes | **yes** | Mumbai, Tokyo, Sydney |
| Gemini 3.5 Flash-Lite | yes | yes | no | Mumbai |
| Claude 5.x (Opus, Sonnet, Haiku, Fable) | yes | yes | **no** on the locations page | — |
| Claude Opus 4.6, 4.5; Sonnet 4.6, 4.5; Claude 3 Haiku | yes | 4.7 and later only | **yes** | — |

**Pages disagree.** The data-residency page's ML-processing table also ticks Singapore for Opus 5.5, Sonnet 5.5, Sonnet 5, Opus 5, Fable 5.1 and Fable 5. That conflicts with the locations page and with Anthropic's "Regional endpoints serve Claude Sonnet 4.6 and earlier models only" **(unverified which is right)**.

**Cost and limits:**
- Non-global Gemini costs 10% more, for example 3.5 Flash $1.65 instead of $1.50 input ([pricing](https://cloud.google.com/vertex-ai/generative-ai/pricing)).
- Claude multi-region and regional endpoints also cost 10% more.
- The Anthropic-managed offering is `global` only, with `inference_geo: "us"` as an option.

**Options left open (A4):**
1. Global endpoints: cheapest and newest models, no residency.
2. Singapore: Gemini 4 Argon or 3.5 Flash, or Claude 4.6 and earlier.
3. The `us` or `eu` multi-region.

### A5. Translation (uses 2 and 4)

| | Cloud Translation NMT | Cloud Translation, Translation LLM | Prompting Gemini |
|---|---|---|---|
| vi↔en | yes ([languages](https://docs.cloud.google.com/translate/docs/languages)) | yes, "Official Support" | yes (Gemini language list) |
| Price | $20 per 1M input characters; $10 credit per month free (about 500k characters) | $10 per 1M input + $10 per 1M output characters ([pricing](https://cloud.google.com/products/translate/pricing)) | tokens; for example Flash-Lite $0.30/$2.50 per 1M |
| Data terms | no training; "held briefly in-memory" ([data usage](https://docs.cloud.google.com/translate/data-usage)) | Same FAQ **(unverified that it covers the LLM path)**. Also listed on Agent Platform pricing as "Translation LLM 002", called through the Agent Platform API. | SST §18 no training; abuse logging as in A3 |
| Regions | `translate.googleapis.com` (global), `translate-eu`, `translate-us`; **no Asia endpoint** ([endpoints](https://docs.cloud.google.com/translate/docs/advanced/endpoints)) | same; multi-region endpoints don't serve preview features | as in A4 |
| Control over tone and glossary | glossaries | glossaries; adaptive translation | full prompt control |

**Use (2), plan translation.** Plans contain no personal data, and every option can do vi↔en. A prompt can follow the plan's own voice, which a translation API can't.

**Use (4), moderator translation.** This is personal data for one service. The choice is between Cloud Translation's in-memory terms and Gemini's no-training terms plus abuse logs.

**Live speech.** "Gemini 3.5 Live Translate" exists as a preview, for real-time speech only; the spec doesn't need it.

**Options left open (A5):**
1. Cloud Translation for both uses.
2. Gemini for (2) and Cloud Translation for (4).
3. Gemini for both.

### A6. Rough prices (global endpoint, USD per 1M tokens, input/output)

| Model | Price | Note |
|---|---|---|
| Gemini 3.1 Flash-Lite | $0.25 / — | |
| Gemini 3.5 Flash-Lite | $0.30 / $2.50 | |
| Gemini 3.8 Flash | $0.75 / $3.75 | introductory price until 2026-12-31, then $1.50 / $7.50 |
| Gemini 3.5 Flash | $1.50 / $9.00 | |
| Gemini 4 Hydrogen | $1.50 / $7.50 | |
| Gemini 4 Argon | $2 / $10 until 2026-11-30; $4 / $20 from 2026-12-01 | |
| Gemini 3.1 Pro (preview) | $2 / $12 | |
| Claude Haiku 5.5 | $0.10 / $0.50 | shown in the "≤100K input" columns; looks low **(unverified)** |
| Claude Sonnet 5.5 | $2 / $10 | |
| Claude Opus 5.5 | $4 / $20 | |
| Claude Fable 5.2 | $8 / $40 | |

Non-global endpoints cost 10% more for Gemini 3 and later, and for Claude Sonnet 4.5 and later.

| Tool | Price |
|---|---|
| Grounding with Google Search, Web Grounding for Enterprise | 5,000 queries per month free (Gemini 3 and 4), then $14 per 1,000 queries |
| Grounding with Google Maps | 5,000 per month free, then $14 per 1,000 |
| Claude web search on Vertex | $10 per 1,000 searches |
| Brave Search API | $5 per 1,000 |

Sources: [Gemini pricing](https://cloud.google.com/vertex-ai/generative-ai/pricing), [partner pricing](https://cloud.google.com/model-garden/partner-models/pricing).

---

## B. Weather

### What the spec needs

From [outings.md](../spec/outings.md), for plans that need dry weather:
- **Before the outing is set:** a date with rain forecast in the plan's hours is not offered.
- **At 24 hours before:** a check raises a decision for a person.
- **Severe-weather warning:** cancels the outing by rule.

These need an hourly precipitation probability out to about 9 days, plus official warnings, for Da Nang and Hoi An.

### B7. Google Weather API (Maps Platform)

| Question | Answer | Source |
|---|---|---|
| Horizon | Hourly forecast: "Up to 240 hours". Daily forecast: "Up to 10 days". | [overview](https://developers.google.com/maps/documentation/weather/overview) |
| Precipitation fields | Hourly "Probability, percentage, and type of precipitation" (`precipitation.probability.percent`, `qpf`) and `thunderstormProbability` | [hourly](https://developers.google.com/maps/documentation/weather/hourly-forecast) |
| **Vietnam coverage** | The table reads `VN | Vietnam | --- | --- | --- | --- | ⬤`. "---" means "unsupported" for current conditions and for daily, hourly and history forecasts; only weather alerts are supported. Vietnam is one of six countries with no forecast at all, alongside China, Cuba, Iran, North Korea and Syria. | [coverage](https://developers.google.com/maps/documentation/weather/coverage) |
| Alerts in Vietnam | Source: "National Center for Hydrometeorological Forecasting", in `en-VN` and `vi-VN`. "Instructions and safety recommendations are returned in the original language published by the agency." | [alerts](https://developers.google.com/maps/documentation/weather/weather-alerts) |
| Price | "Weather Usage" SKU: 10,000 free per month, then $0.15 per 1,000 (falling with volume). Every endpoint is billed, alerts included. | [pricing](https://developers.google.com/maps/billing-and-pricing/pricing), [SKU](https://developers.google.com/maps/billing-and-pricing/sku-details/environment) |
| Caching | Maps SST §21.2.1 (last modified 2026-10-28): "Hourly Forecast values one hour", "Daily Forecast values 24 hours", "Today's Forecast values 30 consecutive calendar days", "Hourly history 240 hours", "Public Alert values one hour", after which "Customer must delete the cached Google Maps Content". | [Maps SST](https://cloud.google.com/maps-platform/terms/maps-service-terms) |
| Use limits | §21.1: "Customers may not use Google Maps Content retrieved from Weather API to recreate a Google service or product (e.g. … a weather app …)" | same |
| Attribution | Google Maps attribution is required when the content is displayed ([policies](https://developers.google.com/maps/documentation/weather/policies)). | same |

**May Issho store "the forecast said rain, yes or no, at time T for date D"?**
- Forecast values may be cached for 1 hour (hourly) or 24 hours (daily).
- The Maps ToS forbids content "based on" Google Maps Content (§3.2.3(c), quoted in the scraping note).
- A stored yes/no derived from the forecast would therefore look like derived Maps content, which the spec also rules out. That reading is **(unverified)** by any explicit term.
- The point is moot for forecasts, since Vietnam has none. It still applies to alerts: Issho may keep "an alert cancelled this outing", but not the alert's content beyond 1 hour **(unverified)**.

### B8. Alternatives

| Source | Horizon and fields | Vietnam | Licence and commercial terms | Storing our own yes/no |
|---|---|---|---|---|
| **Open-Meteo** ([docs](https://open-meteo.com/en/docs), [terms](https://open-meteo.com/en/terms), [pricing](https://open-meteo.com/en/pricing)) | "up to 16 days"; hourly `precipitation_probability`, "based on ensemble weather models with 0.25° (~27 km) resolution"; daily `precipitation_probability_max` | Global models (ECMWF, GFS, ICON and others); no Vietnam-specific model | Free API: "You may only use the free API services for non-commercial purposes", and "Websites or apps that have subscriptions or display advertisements" count as commercial. Paid plans: Standard 1M calls per month, Professional 5M, Enterprise 50M+; prices not shown on the page **(unverified)**. Data "provided under the terms of the CC-BY 4.0 licence". The server is AGPL-3.0 ([repo](https://github.com/open-meteo/open-meteo)) and can be self-hosted. | CC BY 4.0 allows storing with attribution. |
| **Vietnam national service** (VNMHA / NCHMF) | Its "Thoi tiet Viet Nam KTTV" app gives 10-day forecasts and alerts | official source | No public developer API found **(unverified)**. Its alerts reach developers through Google's alert endpoint above. | — |
| **OpenWeather One Call 3.0** ([doc](https://openweathermap.org/api/one-call-3)) | "hourly forecast for 48 hours, daily forecast for 8 days", `pop` (probability of precipitation), "government weather alerts" | global | 1,000 calls per day free, then pay per call | Terms not read **(unverified)**. Daily stops at 8 days. |

Tomorrow.io was not checked.

**Options left open (B):**
1. A paid Open-Meteo plan, or self-hosted Open-Meteo, for forecasts, plus Google's alert endpoint for official warnings.
2. A commercial provider for both forecasts and alerts (OpenWeather is 8 days; others not checked).
3. Ask VNMHA for data access.

Google's Weather API cannot provide forecasts in Vietnam today.

---

## C. Google equivalents

| v1 part | Google equivalent | Gain or loss |
|---|---|---|
| Supabase Postgres (Singapore) | Cloud SQL for PostgreSQL in `asia-southeast1`; AlloyDB for larger scale | Gain: one cloud, IAM and private networking. Loss: Supabase's dashboard and bundled auth. goose and sqlc are unaffected. |
| Supabase Auth, email OTP | Identity Platform / Firebase Auth | Firebase's guides list **email-link** passwordless sign-in, phone SMS codes and TOTP for MFA. They show **no email-code (OTP) sign-in** ([Firebase auth index](https://firebase.google.com/docs/auth/llms.txt), [email link](https://firebase.google.com/docs/auth/web/email-link-auth)). Keeping a 6-digit email code means sending it ourselves and minting custom tokens. Price: email, phone and social users are free up to 50,000 MAU, then $0.0055 per MAU ([pricing](https://cloud.google.com/identity-platform/pricing)). |
| Resend email | none | Google Cloud has no transactional email product; keep Resend or similar. |
| Expo push | Firebase Cloud Messaging direct (FCM sends to APNs for iOS) | Gain: one fewer relay. Loss: Expo's single token and receipts; needs native config. Free. |
| Sentry (EU) | Cloud Error Reporting plus Cloud Logging; Firebase Crashlytics for native crashes | Gain: same project. Loss: Sentry's JS and React Native source-map grouping, releases and session replay **(unverified)**. |
| Phone codes | Identity Platform / Firebase phone auth | Priced per SMS by country: **"Viet Nam (VN) $0.13 / 1 count"** ([pricing](https://cloud.google.com/identity-platform/pricing)). Vietnam delivery and sender-ID rules: see the phone-verification note **(unverified for Firebase specifically)**. |

**Options left open (C):** for each row, move to the Google product or keep the v1 tool.

---

## Unverified

1. Whether an individual (not a company) can enable Claude in Model Garden, or set up the Anthropic-managed offering's "entity type".
2. Which page is right about Claude 5.x in Singapore: the data-residency table (yes) or the locations table and Anthropic's docs (no).
3. Who provides the search behind Anthropic's web search tool, and whether queries leave Google Cloud on the Google-managed offering.
4. Exa's terms on storing results.
5. Whether "Grounding with your data" ($2.50 per 1,000 prompts) is the SKU for "Grounding with your search API".
6. Whether an online-terms customer can opt out of Gemini abuse logging, or get a Master Agreement.
7. Whether Cloud Translation's in-memory and no-training FAQ covers the Translation LLM path.
8. The Claude Haiku 5.5 price as listed ($0.10 / $0.50, in "≤100K input" columns).
9. Whether a stored rain yes/no counts as content "based on" Google Maps Content, and how long an alert-driven cancellation may keep alert content.
10. Open-Meteo paid plan prices.
11. That Vietnam's national service has no public API.
12. OpenWeather's terms on storing data.
13. Sentry features that Error Reporting lacks.
14. Firebase SMS delivery in Vietnam.
