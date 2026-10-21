# Protocol Documentation
<a name="top"></a>

## Table of Contents

- [issho/v1/profile.proto](#issho_v1_profile-proto)
    - [ActivityPreference](#issho-v1-ActivityPreference)
    - [AvailabilitySlot](#issho-v1-AvailabilitySlot)
    - [GetProfileRequest](#issho-v1-GetProfileRequest)
    - [GetProfileResponse](#issho-v1-GetProfileResponse)
    - [Profile](#issho-v1-Profile)
    - [SaveProfileRequest](#issho-v1-SaveProfileRequest)
    - [SaveProfileResponse](#issho-v1-SaveProfileResponse)
    - [SpokenLanguage](#issho-v1-SpokenLanguage)
  
    - [Gender](#issho-v1-Gender)
    - [GroupSize](#issho-v1-GroupSize)
    - [Interest](#issho-v1-Interest)
    - [TimeBlock](#issho-v1-TimeBlock)
  
    - [ProfileService](#issho-v1-ProfileService)
  
- [issho/v1/catalog.proto](#issho_v1_catalog-proto)
    - [Activity](#issho-v1-Activity)
    - [Area](#issho-v1-Area)
    - [City](#issho-v1-City)
    - [Language](#issho-v1-Language)
    - [ListActivitiesRequest](#issho-v1-ListActivitiesRequest)
    - [ListActivitiesResponse](#issho-v1-ListActivitiesResponse)
    - [ListCitiesRequest](#issho-v1-ListCitiesRequest)
    - [ListCitiesResponse](#issho-v1-ListCitiesResponse)
    - [ListLanguagesRequest](#issho-v1-ListLanguagesRequest)
    - [ListLanguagesResponse](#issho-v1-ListLanguagesResponse)
    - [SuggestActivitiesRequest](#issho-v1-SuggestActivitiesRequest)
    - [SuggestActivitiesResponse](#issho-v1-SuggestActivitiesResponse)
  
    - [CatalogService](#issho-v1-CatalogService)
  
- [issho/v1/health.proto](#issho_v1_health-proto)
    - [CheckRequest](#issho-v1-CheckRequest)
    - [CheckResponse](#issho-v1-CheckResponse)
  
    - [HealthService](#issho-v1-HealthService)
  
- [issho/v1/identity.proto](#issho_v1_identity-proto)
    - [DeleteAccountRequest](#issho-v1-DeleteAccountRequest)
    - [DeleteAccountResponse](#issho-v1-DeleteAccountResponse)
    - [GetMeRequest](#issho-v1-GetMeRequest)
    - [GetMeResponse](#issho-v1-GetMeResponse)
    - [User](#issho-v1-User)
  
    - [IdentityService](#issho-v1-IdentityService)
  
- [issho/v1/ui.proto](#issho_v1_ui-proto)
    - [GetStringsRequest](#issho-v1-GetStringsRequest)
    - [GetStringsResponse](#issho-v1-GetStringsResponse)
    - [GetStringsResponse.StringsEntry](#issho-v1-GetStringsResponse-StringsEntry)
  
    - [UIService](#issho-v1-UIService)
  
- [Scalar Value Types](#scalar-value-types)



<a name="issho_v1_profile-proto"></a>
<p align="right"><a href="#top">Top</a></p>

## issho/v1/profile.proto



<a name="issho-v1-ActivityPreference"></a>

### ActivityPreference



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| activity_id | [string](#string) |  |  |
| interest | [Interest](#issho-v1-Interest) |  |  |






<a name="issho-v1-AvailabilitySlot"></a>

### AvailabilitySlot



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| weekday | [int32](#int32) |  | 0 = Sunday … 6 = Saturday. |
| block | [TimeBlock](#issho-v1-TimeBlock) |  |  |






<a name="issho-v1-GetProfileRequest"></a>

### GetProfileRequest







<a name="issho-v1-GetProfileResponse"></a>

### GetProfileResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| profile | [Profile](#issho-v1-Profile) |  |  |
| current_conduct_version | [int32](#int32) |  | The code-of-conduct version the app must show and send back. When it&#39;s newer than profile.conduct_version, the person has to accept it again. |






<a name="issho-v1-Profile"></a>

### Profile



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| display_name | [string](#string) |  | First name or nickname, shown only to confirmed attendees. 1–40 characters. |
| birth_year | [int32](#int32) |  | Must make the person at least 18 this year. |
| gender | [Gender](#issho-v1-Gender) |  |  |
| same_gender_only | [bool](#bool) |  | Only be grouped with people of the same gender. Needs gender woman, man or nonbinary. |
| city_id | [string](#string) |  | Read-only: the city of home_area_id. |
| home_area_id | [string](#string) |  | Area id from CatalogService.ListCities, e.g. &#34;tokyo-kichijoji&#34;. |
| max_travel_min | [int32](#int32) |  | Longest trip from home_area_id they&#39;ll make, 5–180 minutes. |
| spoken_languages | [SpokenLanguage](#issho-v1-SpokenLanguage) | repeated | Languages they can speak at an event (used for matching). At least one fluent; no duplicates. |
| primary_language | [string](#string) |  | Display languages (what they read): ISO 639-1 codes. primary_language is required; secondary_language is optional and must differ. |
| secondary_language | [string](#string) |  |  |
| availability | [AvailabilitySlot](#issho-v1-AvailabilitySlot) | repeated | When they&#39;re usually free. The default availability for every signup. At least one slot; no duplicates. |
| activities | [ActivityPreference](#issho-v1-ActivityPreference) | repeated | Activities they&#39;d do, from CatalogService.ListActivities for their city. Activities not listed have no preference. At least three. |
| group_size | [GroupSize](#issho-v1-GroupSize) |  |  |
| energy | [int32](#int32) |  | 1 = calm, 2 = moderate, 3 = active. |
| budget | [int32](#int32) |  | Most they&#39;re happy to spend: 0 = free … 3 = expensive. |
| conduct_version | [int32](#int32) |  | The code-of-conduct version they accepted. On save it must equal GetProfileResponse.current_conduct_version. |






<a name="issho-v1-SaveProfileRequest"></a>

### SaveProfileRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| profile | [Profile](#issho-v1-Profile) |  |  |






<a name="issho-v1-SaveProfileResponse"></a>

### SaveProfileResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| profile | [Profile](#issho-v1-Profile) |  |  |






<a name="issho-v1-SpokenLanguage"></a>

### SpokenLanguage



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| code | [string](#string) |  | ISO 639-1 code, e.g. &#34;ja&#34;. |
| fluent | [bool](#bool) |  | False = still learning (then level is required). |
| level | [int32](#int32) |  | For learners: 1 beginner, 2 intermediate, 3 advanced. 0 when fluent. |





 


<a name="issho-v1-Gender"></a>

### Gender


| Name | Number | Description |
| ---- | ------ | ----------- |
| GENDER_UNSPECIFIED | 0 |  |
| GENDER_WOMAN | 1 |  |
| GENDER_MAN | 2 |  |
| GENDER_NONBINARY | 3 |  |
| GENDER_PREFER_NOT_TO_SAY | 4 | An answer, unlike GENDER_UNSPECIFIED (not sent), which is rejected. |



<a name="issho-v1-GroupSize"></a>

### GroupSize


| Name | Number | Description |
| ---- | ------ | ----------- |
| GROUP_SIZE_UNSPECIFIED | 0 |  |
| GROUP_SIZE_SMALL | 1 | Around 4 people. |
| GROUP_SIZE_MEDIUM | 2 | Around 6. |
| GROUP_SIZE_LARGE | 3 | Around 8. |



<a name="issho-v1-Interest"></a>

### Interest


| Name | Number | Description |
| ---- | ------ | ----------- |
| INTEREST_UNSPECIFIED | 0 |  |
| INTEREST_OPEN_TO_TRY | 1 |  |
| INTEREST_LIKE | 2 |  |
| INTEREST_LOVE | 3 |  |



<a name="issho-v1-TimeBlock"></a>

### TimeBlock


| Name | Number | Description |
| ---- | ------ | ----------- |
| TIME_BLOCK_UNSPECIFIED | 0 |  |
| TIME_BLOCK_MORNING | 1 |  |
| TIME_BLOCK_AFTERNOON | 2 |  |
| TIME_BLOCK_EVENING | 3 |  |


 

 


<a name="issho-v1-ProfileService"></a>

### ProfileService
ProfileService reads and writes the caller&#39;s own profile: what matching
needs to know about them. Nobody can read another person&#39;s profile (there&#39;s
no profile browsing).
Requires `Authorization: Bearer &lt;Supabase access token&gt;`, and the account
must already exist (`IdentityService.GetMe` creates it).

HTTP: POST /issho.v1.ProfileService/&lt;Method&gt;

| Method Name | Request Type | Response Type | Description |
| ----------- | ------------ | ------------- | ------------|
| GetProfile | [GetProfileRequest](#issho-v1-GetProfileRequest) | [GetProfileResponse](#issho-v1-GetProfileResponse) | GetProfile returns the caller&#39;s profile. NOT_FOUND until onboarding has saved one. |
| SaveProfile | [SaveProfileRequest](#issho-v1-SaveProfileRequest) | [SaveProfileResponse](#issho-v1-SaveProfileResponse) | SaveProfile creates or replaces the caller&#39;s whole profile, including languages, availability and activity preferences, in one transaction. Saving it the first time completes onboarding (`User.onboarded`). INVALID_ARGUMENT names the first field that&#39;s wrong. |

 



<a name="issho_v1_catalog-proto"></a>
<p align="right"><a href="#top">Top</a></p>

## issho/v1/catalog.proto



<a name="issho-v1-Activity"></a>

### Activity



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| id | [string](#string) |  | Stable slug, e.g. &#34;photo-walk&#34;. |
| name | [string](#string) |  | Name in the primary language (the city&#39;s override if set). If there&#39;s no translation in the primary language, the secondary one is used instead; failing both, English. |
| category | [string](#string) |  | Grouping for the onboarding tile grid, e.g. &#34;outdoors&#34;, &#34;food&#34;. |
| indoor | [bool](#bool) |  |  |
| energy | [int32](#int32) |  | 1 = calm, 2 = moderate, 3 = active. |
| cost_level | [int32](#int32) |  | 0 = free … 3 = expensive. |
| conversational | [bool](#bool) |  | True when people can easily talk while doing it (affects friendship formation). |
| min_group_size | [int32](#int32) |  | Smallest and largest group size this activity works for. |
| max_group_size | [int32](#int32) |  |  |
| secondary_name | [string](#string) |  | Name in the secondary language. Empty when none was requested, there&#39;s no translation, or it&#39;s identical to name, so clients can render &#34;name · secondary_name&#34; without showing the same words twice. |






<a name="issho-v1-Area"></a>

### Area



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| id | [string](#string) |  | e.g. &#34;tokyo-kichijoji&#34;. |
| name | [string](#string) |  |  |
| secondary_name | [string](#string) |  |  |






<a name="issho-v1-City"></a>

### City



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| id | [string](#string) |  | City pack id, e.g. &#34;tokyo&#34;. |
| name | [string](#string) |  |  |
| secondary_name | [string](#string) |  | Empty when not requested, unavailable or identical to name. |
| languages | [string](#string) | repeated | Languages commonly used in the city, most common first. Suggestions only: any language is allowed. |
| areas | [Area](#issho-v1-Area) | repeated | Sorted by id. |






<a name="issho-v1-Language"></a>

### Language



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| code | [string](#string) |  | ISO 639-1 code, e.g. &#34;es&#34;. |
| autonym | [string](#string) |  | The language&#39;s name for itself (&#34;Español&#34;), or its English name where none is known. |
| name | [string](#string) |  | Its name in the requested locale (&#34;スペイン語&#34;); falls back to English. |






<a name="issho-v1-ListActivitiesRequest"></a>

### ListActivitiesRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| city_id | [string](#string) |  | City pack id, e.g. &#34;tokyo&#34;. Required. |
| locale | [string](#string) |  | Primary display language: a language code such as &#34;ja&#34; or &#34;es&#34; (a region, as in &#34;ja-JP&#34;, is ignored). Clients pass the user&#39;s primary language. Required. |
| secondary_language | [string](#string) |  | Optional secondary display language, same format. Clients pass the user&#39;s secondary language (outside events) or the second language chosen for an event screen. |






<a name="issho-v1-ListActivitiesResponse"></a>

### ListActivitiesResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| activities | [Activity](#issho-v1-Activity) | repeated | Sorted by category, then id. |






<a name="issho-v1-ListCitiesRequest"></a>

### ListCitiesRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| locale | [string](#string) |  | Primary and optional secondary display language, as in ListActivitiesRequest. |
| secondary_language | [string](#string) |  |  |






<a name="issho-v1-ListCitiesResponse"></a>

### ListCitiesResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| cities | [City](#issho-v1-City) | repeated | Sorted by id. |






<a name="issho-v1-ListLanguagesRequest"></a>

### ListLanguagesRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| locale | [string](#string) |  | Language to name the languages in, e.g. &#34;ja&#34; gives &#34;スペイン語&#34; for es. |






<a name="issho-v1-ListLanguagesResponse"></a>

### ListLanguagesResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| languages | [Language](#issho-v1-Language) | repeated | Sorted by code. |






<a name="issho-v1-SuggestActivitiesRequest"></a>

### SuggestActivitiesRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| city_id | [string](#string) |  | City pack id; only activities offered there are suggested. Required. |
| text | [string](#string) |  | What they enjoy, in their own words and any language. 1–500 characters. |






<a name="issho-v1-SuggestActivitiesResponse"></a>

### SuggestActivitiesResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| suggestions | [ActivityPreference](#issho-v1-ActivityPreference) | repeated | In the order the text mentions them; at most one per activity. |





 

 

 


<a name="issho-v1-CatalogService"></a>

### CatalogService
CatalogService exposes reference data for onboarding and settings: the
activity catalogue (localised per city), cities with their areas, and the
languages people can choose.

Names come back in two languages: a primary and an optional secondary, for
display side by side. Any ISO 639-1 language works: names are stored in
English (plus real local names such as 吉祥寺) and translated on demand.
Requires `Authorization: Bearer &lt;Supabase access token&gt;`.

HTTP: POST /issho.v1.CatalogService/&lt;Method&gt;

| Method Name | Request Type | Response Type | Description |
| ----------- | ------------ | ------------- | ------------|
| ListActivities | [ListActivitiesRequest](#issho-v1-ListActivitiesRequest) | [ListActivitiesResponse](#issho-v1-ListActivitiesResponse) | ListActivities returns the activities enabled in a city, with names in the requested primary and secondary languages. |
| ListCities | [ListCitiesRequest](#issho-v1-ListCitiesRequest) | [ListCitiesResponse](#issho-v1-ListCitiesResponse) | ListCities returns every city with its areas (the &#34;home&#34; choices), with names in the requested primary and secondary languages. |
| ListLanguages | [ListLanguagesRequest](#issho-v1-ListLanguagesRequest) | [ListLanguagesResponse](#issho-v1-ListLanguagesResponse) | ListLanguages returns every language a person can choose to speak or read (ISO 639-1), each named in itself and in the requested language. |
| SuggestActivities | [SuggestActivitiesRequest](#issho-v1-SuggestActivitiesRequest) | [SuggestActivitiesResponse](#issho-v1-SuggestActivitiesResponse) | SuggestActivities reads what someone writes about their interests, in any language, and suggests activities offered in the city, each with an interest level. Suggestions only: the app shows them and the person confirms each one. Uses Claude; FAILED_PRECONDITION when the server has no ANTHROPIC_API_KEY, so the app can hide the feature. An empty list means nothing in the text matched. |

 



<a name="issho_v1_health-proto"></a>
<p align="right"><a href="#top">Top</a></p>

## issho/v1/health.proto



<a name="issho-v1-CheckRequest"></a>

### CheckRequest







<a name="issho-v1-CheckResponse"></a>

### CheckResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| database_ok | [bool](#bool) |  | True when the API could ping Postgres during this request. |
| version | [string](#string) |  | Build version of the API binary (&#34;dev&#34; for local builds). |





 

 

 


<a name="issho-v1-HealthService"></a>

### HealthService
HealthService reports whether the API and its dependencies are reachable.
Public: no Authorization header required. Used by load balancers and the app&#39;s status row.

HTTP: POST /issho.v1.HealthService/Check

| Method Name | Request Type | Response Type | Description |
| ----------- | ------------ | ------------- | ------------|
| Check | [CheckRequest](#issho-v1-CheckRequest) | [CheckResponse](#issho-v1-CheckResponse) | Check pings the database and returns the running build version. |

 



<a name="issho_v1_identity-proto"></a>
<p align="right"><a href="#top">Top</a></p>

## issho/v1/identity.proto



<a name="issho-v1-DeleteAccountRequest"></a>

### DeleteAccountRequest







<a name="issho-v1-DeleteAccountResponse"></a>

### DeleteAccountResponse







<a name="issho-v1-GetMeRequest"></a>

### GetMeRequest







<a name="issho-v1-GetMeResponse"></a>

### GetMeResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| user | [User](#issho-v1-User) |  |  |






<a name="issho-v1-User"></a>

### User



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| id | [string](#string) |  | Same UUID as the Supabase auth user (auth.users.id). |
| email | [string](#string) |  | Email from the Supabase token; empty for providers that hide it. |
| onboarded | [bool](#bool) |  | True once the user has completed onboarding (has a profile). |
| primary_language | [string](#string) |  | Display languages from the profile (empty until onboarded), so the app can show itself in the right languages straight after GetMe. |
| secondary_language | [string](#string) |  |  |





 

 

 


<a name="issho-v1-IdentityService"></a>

### IdentityService
IdentityService manages the signed-in user&#39;s app account.
Authentication itself is handled by Supabase Auth; every call must send
`Authorization: Bearer &lt;Supabase access token&gt;`.

HTTP: POST /issho.v1.IdentityService/&lt;Method&gt;

| Method Name | Request Type | Response Type | Description |
| ----------- | ------------ | ------------- | ------------|
| GetMe | [GetMeRequest](#issho-v1-GetMeRequest) | [GetMeResponse](#issho-v1-GetMeResponse) | GetMe returns the caller&#39;s account, creating the app-side user row the first time a Supabase user calls the API. |
| DeleteAccount | [DeleteAccountRequest](#issho-v1-DeleteAccountRequest) | [DeleteAccountResponse](#issho-v1-DeleteAccountResponse) | DeleteAccount permanently deletes the caller&#39;s Supabase auth user and all of their app data (cascaded from auth.users). The client should sign out afterwards; existing access tokens stop working once they expire. Returns FAILED_PRECONDITION if the server has no Supabase secret key configured. |

 



<a name="issho_v1_ui-proto"></a>
<p align="right"><a href="#top">Top</a></p>

## issho/v1/ui.proto



<a name="issho-v1-GetStringsRequest"></a>

### GetStringsRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| language | [string](#string) |  | ISO 639-1 code; a region (&#34;pt-BR&#34;) is ignored. Required. |






<a name="issho-v1-GetStringsResponse"></a>

### GetStringsResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| strings | [GetStringsResponse.StringsEntry](#issho-v1-GetStringsResponse-StringsEntry) | repeated | Key → text. Keys whose translation isn&#39;t ready (or came back with broken {placeholders}) are left out; show the English text for those. |
| complete | [bool](#bool) |  | False while some translations are still running: cache what came back, but ask again later. |
| catalog_version | [string](#string) |  | Changes whenever the English catalogue changes. |






<a name="issho-v1-GetStringsResponse-StringsEntry"></a>

### GetStringsResponse.StringsEntry



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| key | [string](#string) |  |  |
| value | [string](#string) |  |  |





 

 

 


<a name="issho-v1-UIService"></a>

### UIService
UIService translates the app&#39;s own UI text. The app is written in English
(apps/mobile/src/i18n/en.json, copied into the server by `make gen`); other
languages are machine-translated on demand, cached on the server for
everyone and on the device.

Public (no token): the sign-in screens need it before anyone is signed in.
Only the server&#39;s copy of the catalogue is ever translated, so callers can&#39;t
make the server pay for arbitrary text.

HTTP: POST /issho.v1.UIService/&lt;Method&gt;

| Method Name | Request Type | Response Type | Description |
| ----------- | ------------ | ------------- | ------------|
| GetStrings | [GetStringsRequest](#issho-v1-GetStringsRequest) | [GetStringsResponse](#issho-v1-GetStringsResponse) | GetStrings returns the UI catalogue in one language. |

 



## Scalar Value Types

| .proto Type | Notes | C++ | Java | Python | Go | C# | PHP | Ruby |
| ----------- | ----- | --- | ---- | ------ | -- | -- | --- | ---- |
| <a name="double" /> double |  | double | double | float | float64 | double | float | Float |
| <a name="float" /> float |  | float | float | float | float32 | float | float | Float |
| <a name="int32" /> int32 | Uses variable-length encoding. Inefficient for encoding negative numbers – if your field is likely to have negative values, use sint32 instead. | int32 | int | int | int32 | int | integer | Bignum or Fixnum (as required) |
| <a name="int64" /> int64 | Uses variable-length encoding. Inefficient for encoding negative numbers – if your field is likely to have negative values, use sint64 instead. | int64 | long | int/long | int64 | long | integer/string | Bignum |
| <a name="uint32" /> uint32 | Uses variable-length encoding. | uint32 | int | int/long | uint32 | uint | integer | Bignum or Fixnum (as required) |
| <a name="uint64" /> uint64 | Uses variable-length encoding. | uint64 | long | int/long | uint64 | ulong | integer/string | Bignum or Fixnum (as required) |
| <a name="sint32" /> sint32 | Uses variable-length encoding. Signed int value. These more efficiently encode negative numbers than regular int32s. | int32 | int | int | int32 | int | integer | Bignum or Fixnum (as required) |
| <a name="sint64" /> sint64 | Uses variable-length encoding. Signed int value. These more efficiently encode negative numbers than regular int64s. | int64 | long | int/long | int64 | long | integer/string | Bignum |
| <a name="fixed32" /> fixed32 | Always four bytes. More efficient than uint32 if values are often greater than 2^28. | uint32 | int | int | uint32 | uint | integer | Bignum or Fixnum (as required) |
| <a name="fixed64" /> fixed64 | Always eight bytes. More efficient than uint64 if values are often greater than 2^56. | uint64 | long | int/long | uint64 | ulong | integer/string | Bignum |
| <a name="sfixed32" /> sfixed32 | Always four bytes. | int32 | int | int | int32 | int | integer | Bignum or Fixnum (as required) |
| <a name="sfixed64" /> sfixed64 | Always eight bytes. | int64 | long | int/long | int64 | long | integer/string | Bignum |
| <a name="bool" /> bool |  | bool | boolean | boolean | bool | bool | boolean | TrueClass/FalseClass |
| <a name="string" /> string | A string must always contain UTF-8 encoded or 7-bit ASCII text. | string | String | str/unicode | string | string | string | String (UTF-8) |
| <a name="bytes" /> bytes | May contain any arbitrary sequence of bytes. | string | ByteString | str | []byte | ByteString | string | String (ASCII-8BIT) |

