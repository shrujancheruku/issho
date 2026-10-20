# Protocol Documentation
<a name="top"></a>

## Table of Contents

- [issho/v1/catalog.proto](#issho_v1_catalog-proto)
    - [Activity](#issho-v1-Activity)
    - [ListActivitiesRequest](#issho-v1-ListActivitiesRequest)
    - [ListActivitiesResponse](#issho-v1-ListActivitiesResponse)
  
    - [CatalogService](#issho-v1-CatalogService)
  
- [issho/v1/health.proto](#issho_v1_health-proto)
    - [CheckRequest](#issho-v1-CheckRequest)
    - [CheckResponse](#issho-v1-CheckResponse)
  
    - [HealthService](#issho-v1-HealthService)
  
- [issho/v1/identity.proto](#issho_v1_identity-proto)
    - [GetMeRequest](#issho-v1-GetMeRequest)
    - [GetMeResponse](#issho-v1-GetMeResponse)
    - [User](#issho-v1-User)
  
    - [IdentityService](#issho-v1-IdentityService)
  
- [Scalar Value Types](#scalar-value-types)



<a name="issho_v1_catalog-proto"></a>
<p align="right"><a href="#top">Top</a></p>

## issho/v1/catalog.proto



<a name="issho-v1-Activity"></a>

### Activity



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| id | [string](#string) |  | Stable slug, e.g. &#34;photo-walk&#34;. |
| name | [string](#string) |  | Localised display name (city override if set). |
| category | [string](#string) |  | Grouping for the onboarding tile grid, e.g. &#34;outdoors&#34;, &#34;food&#34;. |
| indoor | [bool](#bool) |  |  |
| energy | [int32](#int32) |  | 1 = calm, 2 = moderate, 3 = active. |
| cost_level | [int32](#int32) |  | 0 = free … 3 = expensive. |
| conversational | [bool](#bool) |  | True when people can easily talk while doing it (affects friendship formation). |
| min_group_size | [int32](#int32) |  | Smallest and largest group size this activity works for. |
| max_group_size | [int32](#int32) |  |  |






<a name="issho-v1-ListActivitiesRequest"></a>

### ListActivitiesRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| city_id | [string](#string) |  | City pack id, e.g. &#34;tokyo&#34;. Required. |
| locale | [string](#string) |  | BCP 47 language tag for display names, e.g. &#34;ja&#34; or &#34;en-GB&#34;. |






<a name="issho-v1-ListActivitiesResponse"></a>

### ListActivitiesResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| activities | [Activity](#issho-v1-Activity) | repeated | Sorted by category, then id. |





 

 

 


<a name="issho-v1-CatalogService"></a>

### CatalogService
CatalogService exposes the activity catalogue, localised per city.
Requires `Authorization: Bearer &lt;Supabase access token&gt;`.

HTTP: POST /issho.v1.CatalogService/&lt;Method&gt;

| Method Name | Request Type | Response Type | Description |
| ----------- | ------------ | ------------- | ------------|
| ListActivities | [ListActivitiesRequest](#issho-v1-ListActivitiesRequest) | [ListActivitiesResponse](#issho-v1-ListActivitiesResponse) | ListActivities returns the activities enabled in a city, with display names resolved for the requested locale (falling back to English). |

 



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





 

 

 


<a name="issho-v1-IdentityService"></a>

### IdentityService
IdentityService manages the signed-in user&#39;s app account.
Authentication itself is handled by Supabase Auth; every call must send
`Authorization: Bearer &lt;Supabase access token&gt;`.

HTTP: POST /issho.v1.IdentityService/&lt;Method&gt;

| Method Name | Request Type | Response Type | Description |
| ----------- | ------------ | ------------- | ------------|
| GetMe | [GetMeRequest](#issho-v1-GetMeRequest) | [GetMeResponse](#issho-v1-GetMeResponse) | GetMe returns the caller&#39;s account, creating the app-side user row the first time a Supabase user calls the API. |

 



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

