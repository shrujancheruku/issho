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
  
- [issho/v1/plans.proto](#issho_v1_plans-proto)
    - [FeedPlan](#issho-v1-FeedPlan)
    - [GetPlanRequest](#issho-v1-GetPlanRequest)
    - [GetPlanResponse](#issho-v1-GetPlanResponse)
    - [ListFeedRequest](#issho-v1-ListFeedRequest)
    - [ListFeedResponse](#issho-v1-ListFeedResponse)
    - [PlanStep](#issho-v1-PlanStep)
    - [SignUpRequest](#issho-v1-SignUpRequest)
    - [SignUpResponse](#issho-v1-SignUpResponse)
    - [Signup](#issho-v1-Signup)
    - [UnavailableTime](#issho-v1-UnavailableTime)
    - [WithdrawRequest](#issho-v1-WithdrawRequest)
    - [WithdrawResponse](#issho-v1-WithdrawResponse)
  
    - [PlanClosedReason](#issho-v1-PlanClosedReason)
    - [PlanLevel](#issho-v1-PlanLevel)
  
    - [PlanService](#issho-v1-PlanService)
  
- [issho/v1/events.proto](#issho_v1_events-proto)
    - [ChatMessage](#issho-v1-ChatMessage)
    - [CheckInRequest](#issho-v1-CheckInRequest)
    - [CheckInResponse](#issho-v1-CheckInResponse)
    - [GetEventRequest](#issho-v1-GetEventRequest)
    - [GetEventResponse](#issho-v1-GetEventResponse)
    - [GetFeedbackRequest](#issho-v1-GetFeedbackRequest)
    - [GetFeedbackResponse](#issho-v1-GetFeedbackResponse)
    - [GroupMate](#issho-v1-GroupMate)
    - [ListChatMessagesRequest](#issho-v1-ListChatMessagesRequest)
    - [ListChatMessagesResponse](#issho-v1-ListChatMessagesResponse)
    - [ListMyEventsRequest](#issho-v1-ListMyEventsRequest)
    - [ListMyEventsResponse](#issho-v1-ListMyEventsResponse)
    - [MyEvent](#issho-v1-MyEvent)
    - [ReportPersonRequest](#issho-v1-ReportPersonRequest)
    - [ReportPersonResponse](#issho-v1-ReportPersonResponse)
    - [RespondToEventRequest](#issho-v1-RespondToEventRequest)
    - [RespondToEventResponse](#issho-v1-RespondToEventResponse)
    - [SendChatMessageRequest](#issho-v1-SendChatMessageRequest)
    - [SendChatMessageResponse](#issho-v1-SendChatMessageResponse)
    - [SubmitFeedbackRequest](#issho-v1-SubmitFeedbackRequest)
    - [SubmitFeedbackResponse](#issho-v1-SubmitFeedbackResponse)
  
    - [EventAction](#issho-v1-EventAction)
    - [EventStatus](#issho-v1-EventStatus)
    - [GroupStatus](#issho-v1-GroupStatus)
    - [InviteResponse](#issho-v1-InviteResponse)
    - [ReportReason](#issho-v1-ReportReason)
  
    - [EventService](#issho-v1-EventService)
  
- [issho/v1/crews.proto](#issho_v1_crews-proto)
    - [Crew](#issho-v1-Crew)
    - [CrewMember](#issho-v1-CrewMember)
    - [CrewMessage](#issho-v1-CrewMessage)
    - [GetCrewRequest](#issho-v1-GetCrewRequest)
    - [GetCrewResponse](#issho-v1-GetCrewResponse)
    - [LeaveCrewRequest](#issho-v1-LeaveCrewRequest)
    - [LeaveCrewResponse](#issho-v1-LeaveCrewResponse)
    - [ListCrewMessagesRequest](#issho-v1-ListCrewMessagesRequest)
    - [ListCrewMessagesResponse](#issho-v1-ListCrewMessagesResponse)
    - [ListMyCrewsRequest](#issho-v1-ListMyCrewsRequest)
    - [ListMyCrewsResponse](#issho-v1-ListMyCrewsResponse)
    - [ReportCrewMemberRequest](#issho-v1-ReportCrewMemberRequest)
    - [ReportCrewMemberResponse](#issho-v1-ReportCrewMemberResponse)
    - [RespondToCrewRequest](#issho-v1-RespondToCrewRequest)
    - [RespondToCrewResponse](#issho-v1-RespondToCrewResponse)
    - [SendCrewMessageRequest](#issho-v1-SendCrewMessageRequest)
    - [SendCrewMessageResponse](#issho-v1-SendCrewMessageResponse)
  
    - [CrewResponse](#issho-v1-CrewResponse)
    - [CrewStatus](#issho-v1-CrewStatus)
    - [LeaveReason](#issho-v1-LeaveReason)
  
    - [CrewService](#issho-v1-CrewService)
  
- [issho/v1/curation.proto](#issho_v1_curation-proto)
    - [AddVenueRequest](#issho-v1-AddVenueRequest)
    - [AddVenueResponse](#issho-v1-AddVenueResponse)
    - [FoundVenue](#issho-v1-FoundVenue)
    - [ListVenueCitiesRequest](#issho-v1-ListVenueCitiesRequest)
    - [ListVenueCitiesResponse](#issho-v1-ListVenueCitiesResponse)
    - [ListVenuesRequest](#issho-v1-ListVenuesRequest)
    - [ListVenuesResponse](#issho-v1-ListVenuesResponse)
    - [SearchVenuesRequest](#issho-v1-SearchVenuesRequest)
    - [SearchVenuesResponse](#issho-v1-SearchVenuesResponse)
    - [SetApprovedVenuesOnlyRequest](#issho-v1-SetApprovedVenuesOnlyRequest)
    - [SetApprovedVenuesOnlyResponse](#issho-v1-SetApprovedVenuesOnlyResponse)
    - [SetVenueStatusRequest](#issho-v1-SetVenueStatusRequest)
    - [SetVenueStatusResponse](#issho-v1-SetVenueStatusResponse)
    - [Venue](#issho-v1-Venue)
    - [VenueArea](#issho-v1-VenueArea)
    - [VenueCategory](#issho-v1-VenueCategory)
    - [VenueCity](#issho-v1-VenueCity)
  
    - [VenueStatus](#issho-v1-VenueStatus)
  
    - [CurationService](#issho-v1-CurationService)
  
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
  
- [issho/v1/metrics.proto](#issho_v1_metrics-proto)
    - [CityNow](#issho-v1-CityNow)
    - [GetCityMetricsRequest](#issho-v1-GetCityMetricsRequest)
    - [GetCityMetricsResponse](#issho-v1-GetCityMetricsResponse)
    - [WeekMetrics](#issho-v1-WeekMetrics)
  
    - [MetricsService](#issho-v1-MetricsService)
  
- [issho/v1/moderation.proto](#issho_v1_moderation-proto)
    - [CrewLeave](#issho-v1-CrewLeave)
    - [GetModeratorRequest](#issho-v1-GetModeratorRequest)
    - [GetModeratorResponse](#issho-v1-GetModeratorResponse)
    - [GetPersonRequest](#issho-v1-GetPersonRequest)
    - [GetPersonResponse](#issho-v1-GetPersonResponse)
    - [LiftSuspensionRequest](#issho-v1-LiftSuspensionRequest)
    - [LiftSuspensionResponse](#issho-v1-LiftSuspensionResponse)
    - [ListCrewLeavesRequest](#issho-v1-ListCrewLeavesRequest)
    - [ListCrewLeavesResponse](#issho-v1-ListCrewLeavesResponse)
    - [ListFlagsRequest](#issho-v1-ListFlagsRequest)
    - [ListFlagsResponse](#issho-v1-ListFlagsResponse)
    - [ListReportsRequest](#issho-v1-ListReportsRequest)
    - [ListReportsResponse](#issho-v1-ListReportsResponse)
    - [ListSuspendedRequest](#issho-v1-ListSuspendedRequest)
    - [ListSuspendedResponse](#issho-v1-ListSuspendedResponse)
    - [ListTranslationLanguagesRequest](#issho-v1-ListTranslationLanguagesRequest)
    - [ListTranslationLanguagesResponse](#issho-v1-ListTranslationLanguagesResponse)
    - [ListTranslationsRequest](#issho-v1-ListTranslationsRequest)
    - [ListTranslationsResponse](#issho-v1-ListTranslationsResponse)
    - [ModerationFlag](#issho-v1-ModerationFlag)
    - [ModerationReport](#issho-v1-ModerationReport)
    - [Person](#issho-v1-Person)
    - [PersonOuting](#issho-v1-PersonOuting)
    - [PersonSummary](#issho-v1-PersonSummary)
    - [ResolveFlagRequest](#issho-v1-ResolveFlagRequest)
    - [ResolveFlagResponse](#issho-v1-ResolveFlagResponse)
    - [ResolveReportRequest](#issho-v1-ResolveReportRequest)
    - [ResolveReportResponse](#issho-v1-ResolveReportResponse)
    - [ReviewTranslationRequest](#issho-v1-ReviewTranslationRequest)
    - [ReviewTranslationResponse](#issho-v1-ReviewTranslationResponse)
    - [SearchPeopleRequest](#issho-v1-SearchPeopleRequest)
    - [SearchPeopleResponse](#issho-v1-SearchPeopleResponse)
    - [SuspendPersonRequest](#issho-v1-SuspendPersonRequest)
    - [SuspendPersonResponse](#issho-v1-SuspendPersonResponse)
    - [SuspendedPerson](#issho-v1-SuspendedPerson)
    - [Suspension](#issho-v1-Suspension)
    - [Translation](#issho-v1-Translation)
    - [TranslationLanguage](#issho-v1-TranslationLanguage)
  
    - [FlagKind](#issho-v1-FlagKind)
    - [OutingFeedback](#issho-v1-OutingFeedback)
    - [ReportStatus](#issho-v1-ReportStatus)
  
    - [ModerationService](#issho-v1-ModerationService)
  
- [issho/v1/push.proto](#issho_v1_push-proto)
    - [RegisterPushTokenRequest](#issho-v1-RegisterPushTokenRequest)
    - [RegisterPushTokenResponse](#issho-v1-RegisterPushTokenResponse)
    - [UnregisterPushTokenRequest](#issho-v1-UnregisterPushTokenRequest)
    - [UnregisterPushTokenResponse](#issho-v1-UnregisterPushTokenResponse)
  
    - [PushPlatform](#issho-v1-PushPlatform)
  
    - [PushService](#issho-v1-PushService)
  
- [issho/v1/review.proto](#issho_v1_review-proto)
    - [ApprovePlanRequest](#issho-v1-ApprovePlanRequest)
    - [ApprovePlanResponse](#issho-v1-ApprovePlanResponse)
    - [ApproveProposalRequest](#issho-v1-ApproveProposalRequest)
    - [ApproveProposalResponse](#issho-v1-ApproveProposalResponse)
    - [CreatePlanRequest](#issho-v1-CreatePlanRequest)
    - [CreatePlanResponse](#issho-v1-CreatePlanResponse)
    - [EditPlanRequest](#issho-v1-EditPlanRequest)
    - [EditPlanResponse](#issho-v1-EditPlanResponse)
    - [FindPlanPlacesRequest](#issho-v1-FindPlanPlacesRequest)
    - [FindPlanPlacesResponse](#issho-v1-FindPlanPlacesResponse)
    - [FoundPlace](#issho-v1-FoundPlace)
    - [GetReviewEventRequest](#issho-v1-GetReviewEventRequest)
    - [GetReviewEventResponse](#issho-v1-GetReviewEventResponse)
    - [GetReviewPlanRequest](#issho-v1-GetReviewPlanRequest)
    - [GetReviewPlanResponse](#issho-v1-GetReviewPlanResponse)
    - [ListReviewCitiesRequest](#issho-v1-ListReviewCitiesRequest)
    - [ListReviewCitiesResponse](#issho-v1-ListReviewCitiesResponse)
    - [ListReviewEventsRequest](#issho-v1-ListReviewEventsRequest)
    - [ListReviewEventsResponse](#issho-v1-ListReviewEventsResponse)
    - [ListReviewPlansRequest](#issho-v1-ListReviewPlansRequest)
    - [ListReviewPlansResponse](#issho-v1-ListReviewPlansResponse)
    - [ListVenueOptionsRequest](#issho-v1-ListVenueOptionsRequest)
    - [ListVenueOptionsResponse](#issho-v1-ListVenueOptionsResponse)
    - [MoveProposalMemberRequest](#issho-v1-MoveProposalMemberRequest)
    - [MoveProposalMemberResponse](#issho-v1-MoveProposalMemberResponse)
    - [RejectPlanRequest](#issho-v1-RejectPlanRequest)
    - [RejectPlanResponse](#issho-v1-RejectPlanResponse)
    - [RejectProposalRequest](#issho-v1-RejectProposalRequest)
    - [RejectProposalResponse](#issho-v1-RejectProposalResponse)
    - [RemoveProposalMemberRequest](#issho-v1-RemoveProposalMemberRequest)
    - [RemoveProposalMemberResponse](#issho-v1-RemoveProposalMemberResponse)
    - [ReviewCity](#issho-v1-ReviewCity)
    - [ReviewEvent](#issho-v1-ReviewEvent)
    - [ReviewGroup](#issho-v1-ReviewGroup)
    - [ReviewMember](#issho-v1-ReviewMember)
    - [ReviewPlan](#issho-v1-ReviewPlan)
    - [ReviewStop](#issho-v1-ReviewStop)
    - [SetCityReviewRequest](#issho-v1-SetCityReviewRequest)
    - [SetCityReviewResponse](#issho-v1-SetCityReviewResponse)
    - [SetGroupMeetingPointRequest](#issho-v1-SetGroupMeetingPointRequest)
    - [SetGroupMeetingPointResponse](#issho-v1-SetGroupMeetingPointResponse)
    - [SetPlanStopsRequest](#issho-v1-SetPlanStopsRequest)
    - [SetPlanStopsResponse](#issho-v1-SetPlanStopsResponse)
    - [SetPlanVenueRequest](#issho-v1-SetPlanVenueRequest)
    - [SetPlanVenueResponse](#issho-v1-SetPlanVenueResponse)
    - [SetProposalStartRequest](#issho-v1-SetProposalStartRequest)
    - [SetProposalStartResponse](#issho-v1-SetProposalStartResponse)
    - [VenueOption](#issho-v1-VenueOption)
    - [WithdrawPlanRequest](#issho-v1-WithdrawPlanRequest)
    - [WithdrawPlanResponse](#issho-v1-WithdrawPlanResponse)
  
    - [ReviewEventStatus](#issho-v1-ReviewEventStatus)
    - [ReviewPlanStatus](#issho-v1-ReviewPlanStatus)
  
    - [ReviewService](#issho-v1-ReviewService)
  
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
| lat | [double](#double) |  | The area&#39;s centre (WGS 84), from the city pack: where the map of plans puts the area&#39;s pin. |
| lng | [double](#double) |  |  |






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
| SuggestActivities | [SuggestActivitiesRequest](#issho-v1-SuggestActivitiesRequest) | [SuggestActivitiesResponse](#issho-v1-SuggestActivitiesResponse) | SuggestActivities reads what someone writes about their interests, in any language, and suggests activities offered in the city, each with an interest level. Suggestions only: the app shows them and the person confirms each one. Uses Gemini on Google Cloud&#39;s Agent Platform; FAILED_PRECONDITION when the server has no GOOGLE_GEMINI_API_KEY, so the app can hide the feature. An empty list means nothing in the text matched. |

 



<a name="issho_v1_plans-proto"></a>
<p align="right"><a href="#top">Top</a></p>

## issho/v1/plans.proto



<a name="issho-v1-FeedPlan"></a>

### FeedPlan
An open plan as the caller sees it.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| id | [string](#string) |  |  |
| area_id | [string](#string) |  | Area id (CatalogService.ListCities), e.g. &#34;tokyo-kichijoji&#34;. |
| activity_id | [string](#string) |  | Activity id (CatalogService.ListActivities). |
| follow_up_activity_id | [string](#string) |  | An optional second activity, such as a meal after bouldering; &#34;&#34; = none. |
| primary_language | [string](#string) |  | The plan&#39;s languages (ISO 639-1): everyone at the event speaks the primary one, or the secondary one for a language exchange (&#34;&#34; = none). |
| secondary_language | [string](#string) |  |  |
| signups | [int32](#int32) |  | How many people are waiting for this round, including the caller. |
| level | [PlanLevel](#issho-v1-PlanLevel) |  |  |
| travel_minutes | [int32](#int32) |  | Travel time from the caller&#39;s home area; -1 = no route. |
| window_start | [string](#string) |  | When this round can happen: window_days days from window_start, a date in the city (&#34;2030-01-05&#34;). The worker can schedule it any time in the window once it&#39;s ready. |
| window_days | [int32](#int32) |  |  |
| signup_deadline | [google.protobuf.Timestamp](#google-protobuf-Timestamp) |  | Signups for this round close then; the round goes ahead with whoever can make it. |
| signed_up | [bool](#bool) |  |  |
| title | [string](#string) |  | What the outing is, as one line: &#34;Climbing and bouldering, then lunch or dinner out&#34;. In the caller&#39;s primary display language, with the secondary below it (&#34;&#34; when the same or not chosen). Area names are left out on purpose: they&#39;re proper nouns, never machine-translated, and shown with area_id&#39;s name. |
| secondary_title | [string](#string) |  |  |
| closed_reason | [PlanClosedReason](#issho-v1-PlanClosedReason) |  | Why the plan closed, for a plan closed in the last 14 days (GetPlan only; the feed lists open plans). Unspecified while it&#39;s open. |
| closed_at | [google.protobuf.Timestamp](#google-protobuf-Timestamp) |  |  |
| friends_interested | [int32](#int32) |  | How many people the caller gets on with (they picked each other after an outing) have signed up: a count, never who. Such plans rank higher. |






<a name="issho-v1-GetPlanRequest"></a>

### GetPlanRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| plan_id | [string](#string) |  |  |






<a name="issho-v1-GetPlanResponse"></a>

### GetPlanResponse
A plan that closed in the last 14 days is still returned, with
closed_reason set; older or other cities&#39; ones are NOT_FOUND.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| plan | [FeedPlan](#issho-v1-FeedPlan) |  |  |
| signup | [Signup](#issho-v1-Signup) |  | The caller&#39;s signup; unset when they haven&#39;t signed up. |
| steps | [PlanStep](#issho-v1-PlanStep) | repeated | What the outing is, in order: the activity, then any follow-up, each at its venue; or, for a walk, the meeting place, the stops on its theme and where it finishes. |
| route_polyline | [string](#string) |  | The walking route through the stops&#39; venues (for a walk, through every stop; otherwise from the first venue to the second), in Google&#39;s encoded polyline format (precision 5), for the plan&#39;s map; &#34;&#34; when there&#39;s no route or it isn&#39;t computed yet. |
| summary | [string](#string) |  | What a walk is like, in a sentence or two (or a summary an admin wrote for any plan, ReviewService.EditPlan), in the caller&#39;s primary and secondary display languages; &#34;&#34; when there&#39;s none. secondary_summary is &#34;&#34; when the same. |
| secondary_summary | [string](#string) |  |  |






<a name="issho-v1-ListFeedRequest"></a>

### ListFeedRequest







<a name="issho-v1-ListFeedResponse"></a>

### ListFeedResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| plans | [FeedPlan](#issho-v1-FeedPlan) | repeated |  |






<a name="issho-v1-PlanStep"></a>

### PlanStep
One stop of a plan.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| activity_id | [string](#string) |  | Activity id (CatalogService.ListActivities): what happens here. &#34;&#34; for the meeting point and a walk&#39;s stops, where the place is the stop. |
| venue_name | [string](#string) |  | The venue&#39;s name from Google, in the caller&#39;s primary and secondary display languages, fetched live (Maps terms: nothing but the place id is stored). Empty when the activity needs only a meeting point, no venue was found, or the server has no GOOGLE_MAPS_API_KEY. |
| venue_secondary_name | [string](#string) |  | Empty when the same as venue_name. |
| venue_address | [string](#string) |  | Short address in the primary language. |
| venue_maps_url | [string](#string) |  | Opens the venue in Google Maps. |
| walk_minutes | [int32](#int32) |  | Walk from the previous stop&#39;s venue, in minutes: Google&#39;s walking time when the route is known (GetPlanResponse.route_polyline), otherwise an estimate; 0 for the first stop or when either venue is unknown. |
| lat | [double](#double) |  | The venue&#39;s location (WGS 84), live from Google like its name, for the plan&#39;s map; both 0 when there&#39;s no venue. |
| lng | [double](#double) |  |  |
| meeting | [bool](#bool) |  | The meeting point rather than an activity (activity_id &#34;&#34;): where a walk starts. |
| finish | [bool](#bool) |  | A walk&#39;s last stop: somewhere to sit and talk. |
| minutes | [int32](#int32) |  | A walk&#39;s planned minutes at this stop; 0 for other plans. |
| note | [string](#string) |  | For a walk, why the stop is worth it and what to look at (for the meeting point, where exactly to gather), in the caller&#39;s primary and secondary display languages; &#34;&#34; for other plans. secondary_note is &#34;&#34; when the same. |
| secondary_note | [string](#string) |  |  |






<a name="issho-v1-SignUpRequest"></a>

### SignUpRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| plan_id | [string](#string) |  |  |
| signup | [Signup](#issho-v1-Signup) |  |  |






<a name="issho-v1-SignUpResponse"></a>

### SignUpResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| plan | [FeedPlan](#issho-v1-FeedPlan) |  |  |






<a name="issho-v1-Signup"></a>

### Signup
The times that suit someone for one plan. Pre-fill pattern with the
profile&#39;s availability; an empty pattern means &#34;my usual availability&#34;.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| pattern | [AvailabilitySlot](#issho-v1-AvailabilitySlot) | repeated | Weekly slots that suit them for this plan. No duplicates. |
| unavailable | [UnavailableTime](#issho-v1-UnavailableTime) | repeated | Dates or parts of a date in the plan&#39;s window they can&#39;t make. |






<a name="issho-v1-UnavailableTime"></a>

### UnavailableTime



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| date | [string](#string) |  | A date in the plan&#39;s window, &#34;2030-01-09&#34;. |
| block | [TimeBlock](#issho-v1-TimeBlock) |  | TIME_BLOCK_UNSPECIFIED = the whole day. |






<a name="issho-v1-WithdrawRequest"></a>

### WithdrawRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| plan_id | [string](#string) |  |  |






<a name="issho-v1-WithdrawResponse"></a>

### WithdrawResponse






 


<a name="issho-v1-PlanClosedReason"></a>

### PlanClosedReason


| Name | Number | Description |
| ---- | ------ | ----------- |
| PLAN_CLOSED_REASON_UNSPECIFIED | 0 |  |
| PLAN_CLOSED_REASON_NO_GROUP | 1 | The deadline passed and no group could form. |
| PLAN_CLOSED_REASON_DONE | 2 | Scheduled, with nobody left waiting. |
| PLAN_CLOSED_REASON_WITHDRAWN | 3 | Taken down (its activity or venue is no longer available). |



<a name="issho-v1-PlanLevel"></a>

### PlanLevel
How well a plan suits the caller, best last.

| Name | Number | Description |
| ---- | ------ | ----------- |
| PLAN_LEVEL_UNSPECIFIED | 0 |  |
| PLAN_LEVEL_NOT_FOR_YOU | 1 | They don&#39;t speak the plan&#39;s language(s): shown greyed out, no signup. |
| PLAN_LEVEL_LOW | 2 | No preference for the activity, or beyond their longest trip. |
| PLAN_LEVEL_MAYBE | 3 | An activity they&#39;re open to try. |
| PLAN_LEVEL_GOOD | 4 | One they like. |
| PLAN_LEVEL_TOP | 5 | One they love. |


 

 


<a name="issho-v1-PlanService"></a>

### PlanService
PlanService shows open plans to the caller and signs them up. A plan is an
outing without a date yet (an activity in an area, in a language); once
enough people sign up, the worker picks a date and forms groups.
Plans carry ids only: names come from CatalogService (ListActivities,
ListCities), which the app already has in the caller&#39;s languages.
Signup counts are shown, never who signed up.
Requires `Authorization: Bearer &lt;Supabase access token&gt;` and a saved
profile (FAILED_PRECONDITION otherwise).

HTTP: POST /issho.v1.PlanService/&lt;Method&gt;

| Method Name | Request Type | Response Type | Description |
| ----------- | ------------ | ------------- | ------------|
| ListFeed | [ListFeedRequest](#issho-v1-ListFeedRequest) | [ListFeedResponse](#issho-v1-ListFeedResponse) | ListFeed returns every open plan in the caller&#39;s city, best for them first, so scrolling far enough shows everything. Each plan&#39;s level also colours the map. |
| GetPlan | [GetPlanRequest](#issho-v1-GetPlanRequest) | [GetPlanResponse](#issho-v1-GetPlanResponse) | GetPlan returns one open plan with the caller&#39;s signup, if any. NOT_FOUND for an unknown or closed plan, or one in another city. |
| SignUp | [SignUpRequest](#issho-v1-SignUpRequest) | [SignUpResponse](#issho-v1-SignUpResponse) | SignUp signs the caller up for an open plan, or replaces the times they gave. FAILED_PRECONDITION when the plan has closed or they don&#39;t speak its language; INVALID_ARGUMENT for times outside the plan&#39;s window. |
| Withdraw | [WithdrawRequest](#issho-v1-WithdrawRequest) | [WithdrawResponse](#issho-v1-WithdrawResponse) | Withdraw removes the caller&#39;s signup. Withdrawing twice is fine. |

 



<a name="issho_v1_events-proto"></a>
<p align="right"><a href="#top">Top</a></p>

## issho/v1/events.proto



<a name="issho-v1-ChatMessage"></a>

### ChatMessage
One message in a group chat.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| id | [int64](#int64) |  |  |
| name | [string](#string) |  | The sender&#39;s first name (&#34;&#34; for a deleted account). |
| body | [string](#string) |  |  |
| at | [google.protobuf.Timestamp](#google-protobuf-Timestamp) |  |  |
| mine | [bool](#bool) |  | Sent by the caller. |






<a name="issho-v1-CheckInRequest"></a>

### CheckInRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| event_id | [string](#string) |  |  |






<a name="issho-v1-CheckInResponse"></a>

### CheckInResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| event | [MyEvent](#issho-v1-MyEvent) |  |  |






<a name="issho-v1-GetEventRequest"></a>

### GetEventRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| event_id | [string](#string) |  |  |






<a name="issho-v1-GetEventResponse"></a>

### GetEventResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| event | [MyEvent](#issho-v1-MyEvent) |  |  |
| steps | [PlanStep](#issho-v1-PlanStep) | repeated | As in GetPlanResponse.steps; the caller&#39;s group may meet at a different venue from the plan&#39;s. |
| attendees | [string](#string) | repeated | First names of the people confirmed in the caller&#39;s group, including the caller. Empty unless the caller confirmed (no profile browsing). |
| route_polyline | [string](#string) |  | As GetPlanResponse.route_polyline; &#34;&#34; when the caller&#39;s group meets somewhere other than the plan&#39;s venue. |
| summary | [string](#string) |  | As GetPlanResponse.summary. |
| secondary_summary | [string](#string) |  |  |






<a name="issho-v1-GetFeedbackRequest"></a>

### GetFeedbackRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| event_id | [string](#string) |  |  |






<a name="issho-v1-GetFeedbackResponse"></a>

### GetFeedbackResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| mates | [GroupMate](#issho-v1-GroupMate) | repeated | The others in the caller&#39;s group who confirmed (not the caller), including any who cancelled since. |
| given | [bool](#bool) |  | The caller&#39;s earlier answer, if any. |
| attended | [bool](#bool) |  |  |
| plan_rating | [int32](#int32) |  | 1–5, or 0 when not given. |
| group_rating | [int32](#int32) |  |  |
| picks | [string](#string) | repeated | GroupMate.id of the people the caller would meet again. |
| opens_at | [google.protobuf.Timestamp](#google-protobuf-Timestamp) |  | When feedback can be given; unset for someone who can&#39;t give it (they cancelled, or the group didn&#39;t go ahead). |
| closes_at | [google.protobuf.Timestamp](#google-protobuf-Timestamp) |  |  |
| open | [bool](#bool) |  |  |
| avoid | [string](#string) | repeated | GroupMate.id of the people the caller would rather not be in a group with again. |






<a name="issho-v1-GroupMate"></a>

### GroupMate
Someone else in the caller&#39;s group, for picks and reports.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| id | [string](#string) |  | Opaque; only meaningful in feedback and reports for this event. |
| name | [string](#string) |  | First name. |
| cancelled | [bool](#bool) |  | They cancelled their place: they can be reported but not picked. |






<a name="issho-v1-ListChatMessagesRequest"></a>

### ListChatMessagesRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| event_id | [string](#string) |  |  |
| after_id | [int64](#int64) |  | 0 = the latest messages. |






<a name="issho-v1-ListChatMessagesResponse"></a>

### ListChatMessagesResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| messages | [ChatMessage](#issho-v1-ChatMessage) | repeated |  |
| opens_at | [google.protobuf.Timestamp](#google-protobuf-Timestamp) |  | When it can be written in; unset when the event has no start time. |
| closes_at | [google.protobuf.Timestamp](#google-protobuf-Timestamp) |  |  |
| open | [bool](#bool) |  |  |






<a name="issho-v1-ListMyEventsRequest"></a>

### ListMyEventsRequest







<a name="issho-v1-ListMyEventsResponse"></a>

### ListMyEventsResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| events | [MyEvent](#issho-v1-MyEvent) | repeated |  |






<a name="issho-v1-MyEvent"></a>

### MyEvent
One event as the caller sees it.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| id | [string](#string) |  |  |
| plan_id | [string](#string) |  |  |
| title | [string](#string) |  | As FeedPlan.title / secondary_title. |
| secondary_title | [string](#string) |  |  |
| activity_id | [string](#string) |  |  |
| follow_up_activity_id | [string](#string) |  |  |
| area_id | [string](#string) |  |  |
| primary_language | [string](#string) |  |  |
| secondary_language | [string](#string) |  |  |
| date | [string](#string) |  | The date in the city (&#34;2030-01-05&#34;) and part of the day. |
| block | [TimeBlock](#issho-v1-TimeBlock) |  |  |
| starts_at | [google.protobuf.Timestamp](#google-protobuf-Timestamp) |  | When it starts; unset if not decided. |
| confirm_by | [google.protobuf.Timestamp](#google-protobuf-Timestamp) |  | Answer by then. |
| status | [EventStatus](#issho-v1-EventStatus) |  |  |
| group_status | [GroupStatus](#issho-v1-GroupStatus) |  |  |
| response | [InviteResponse](#issho-v1-InviteResponse) |  |  |
| confirmed | [int32](#int32) |  | How many in the caller&#39;s group have confirmed (a count, never who). |
| quorum | [int32](#int32) |  | Confirmations that make the group go ahead straight away. |
| min_attendance | [int32](#int32) |  | The fewest it can go ahead with at confirm_by. |
| meeting_point | [string](#string) |  | Exactly where to meet, once revealed to someone who confirmed; &#34;&#34; before reveal_at or when no more than the first stop&#39;s venue is needed. |
| reveal_at | [google.protobuf.Timestamp](#google-protobuf-Timestamp) |  | When the meeting point is shown (24 hours before the start). |
| checked_in | [bool](#bool) |  | The caller has checked in. |
| check_in_from | [google.protobuf.Timestamp](#google-protobuf-Timestamp) |  | When check-in is open (30 minutes before the start to 3 hours after); unset without a start time. |
| check_in_until | [google.protobuf.Timestamp](#google-protobuf-Timestamp) |  |  |
| feedback_from | [google.protobuf.Timestamp](#google-protobuf-Timestamp) |  | When the caller can give feedback (see GetFeedback); unset unless they went ahead with their group. |
| feedback_until | [google.protobuf.Timestamp](#google-protobuf-Timestamp) |  |  |
| feedback_given | [bool](#bool) |  | The caller has given feedback. |






<a name="issho-v1-ReportPersonRequest"></a>

### ReportPersonRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| event_id | [string](#string) |  |  |
| mate_id | [string](#string) |  | GroupMate.id. |
| reason | [ReportReason](#issho-v1-ReportReason) |  |  |
| details | [string](#string) |  | Optional, up to 1000 characters. |






<a name="issho-v1-ReportPersonResponse"></a>

### ReportPersonResponse







<a name="issho-v1-RespondToEventRequest"></a>

### RespondToEventRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| event_id | [string](#string) |  |  |
| action | [EventAction](#issho-v1-EventAction) |  |  |






<a name="issho-v1-RespondToEventResponse"></a>

### RespondToEventResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| event | [MyEvent](#issho-v1-MyEvent) |  |  |






<a name="issho-v1-SendChatMessageRequest"></a>

### SendChatMessageRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| event_id | [string](#string) |  |  |
| body | [string](#string) |  |  |






<a name="issho-v1-SendChatMessageResponse"></a>

### SendChatMessageResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| message | [ChatMessage](#issho-v1-ChatMessage) |  |  |






<a name="issho-v1-SubmitFeedbackRequest"></a>

### SubmitFeedbackRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| event_id | [string](#string) |  |  |
| attended | [bool](#bool) |  | Whether the caller went. When false, no ratings or picks are taken. |
| plan_rating | [int32](#int32) |  | 1–5; required when attended. |
| group_rating | [int32](#int32) |  |  |
| picks | [string](#string) | repeated | GroupMate.id of people the caller would meet again (any number, including none; only people who went ahead with the group). |
| avoid | [string](#string) | repeated | GroupMate.id of people the caller would rather not be in a group with again: they&#39;re never grouped together again, nor offered a crew together. Private; nobody in both lists. |






<a name="issho-v1-SubmitFeedbackResponse"></a>

### SubmitFeedbackResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| feedback | [GetFeedbackResponse](#issho-v1-GetFeedbackResponse) |  |  |





 


<a name="issho-v1-EventAction"></a>

### EventAction


| Name | Number | Description |
| ---- | ------ | ----------- |
| EVENT_ACTION_UNSPECIFIED | 0 |  |
| EVENT_ACTION_CONFIRM | 1 | Invited (or declined, changing their mind) → confirmed, until confirm_by. |
| EVENT_ACTION_DECLINE | 2 | Invited → declined, until confirm_by. |
| EVENT_ACTION_CANCEL | 3 | Confirmed → cancelled, until the event starts. |



<a name="issho-v1-EventStatus"></a>

### EventStatus


| Name | Number | Description |
| ---- | ------ | ----------- |
| EVENT_STATUS_UNSPECIFIED | 0 |  |
| EVENT_STATUS_SCHEDULED | 1 |  |
| EVENT_STATUS_DONE | 2 |  |
| EVENT_STATUS_CANCELLED | 3 |  |



<a name="issho-v1-GroupStatus"></a>

### GroupStatus


| Name | Number | Description |
| ---- | ------ | ----------- |
| GROUP_STATUS_UNSPECIFIED | 0 |  |
| GROUP_STATUS_INVITING | 1 | Collecting answers. |
| GROUP_STATUS_CONFIRMED | 2 | Going ahead. |
| GROUP_STATUS_CANCELLED | 3 |  |



<a name="issho-v1-InviteResponse"></a>

### InviteResponse


| Name | Number | Description |
| ---- | ------ | ----------- |
| INVITE_RESPONSE_UNSPECIFIED | 0 |  |
| INVITE_RESPONSE_INVITED | 1 |  |
| INVITE_RESPONSE_CONFIRMED | 2 |  |
| INVITE_RESPONSE_DECLINED | 3 |  |
| INVITE_RESPONSE_DROPPED | 4 | Confirmed, then cancelled. |



<a name="issho-v1-ReportReason"></a>

### ReportReason


| Name | Number | Description |
| ---- | ------ | ----------- |
| REPORT_REASON_UNSPECIFIED | 0 |  |
| REPORT_REASON_DATING | 1 | Here for dating rather than friends. |
| REPORT_REASON_HARASSMENT | 2 | Rude, harassing or made people uncomfortable. |
| REPORT_REASON_UNSAFE | 3 | Made people feel unsafe. |
| REPORT_REASON_NO_SHOW | 4 | Confirmed and didn&#39;t come. |
| REPORT_REASON_OTHER | 5 |  |


 

 


<a name="issho-v1-EventService"></a>

### EventService
EventService shows the caller the events they&#39;re invited to and records
their answers. An event is a plan round given a date; the caller is in one
group of it. A group goes ahead as soon as `quorum` people confirm, or at
`confirm_by` with at least `min_attendance`; otherwise it&#39;s cancelled and
the people who confirmed go back to waiting for the plan.
Requires `Authorization: Bearer &lt;Supabase access token&gt;` and a saved
profile (FAILED_PRECONDITION otherwise).

HTTP: POST /issho.v1.EventService/&lt;Method&gt;

| Method Name | Request Type | Response Type | Description |
| ----------- | ------------ | ------------- | ------------|
| ListMyEvents | [ListMyEventsRequest](#issho-v1-ListMyEventsRequest) | [ListMyEventsResponse](#issho-v1-ListMyEventsResponse) | ListMyEvents returns the caller&#39;s events from yesterday on, soonest first, whatever their answer, and older ones still waiting for their feedback. |
| GetEvent | [GetEventRequest](#issho-v1-GetEventRequest) | [GetEventResponse](#issho-v1-GetEventResponse) | GetEvent returns one of the caller&#39;s events with its stops and, if the caller confirmed, who else is going. NOT_FOUND unless the caller is invited to it. |
| RespondToEvent | [RespondToEventRequest](#issho-v1-RespondToEventRequest) | [RespondToEventResponse](#issho-v1-RespondToEventResponse) | RespondToEvent confirms, declines or cancels the caller&#39;s place. FAILED_PRECONDITION when it&#39;s too late, the event is cancelled or over, or the answer doesn&#39;t apply (e.g. cancelling a place not confirmed). |
| ListChatMessages | [ListChatMessagesRequest](#issho-v1-ListChatMessagesRequest) | [ListChatMessagesResponse](#issho-v1-ListChatMessagesResponse) | ListChatMessages returns the caller&#39;s group chat for an event: the latest messages, or those after after_id. Only for people who confirmed (PERMISSION_DENIED otherwise); empty until it opens 48 hours before the start. New messages also arrive over the WebSocket at GET /ws/chat (see the backend README). |
| SendChatMessage | [SendChatMessageRequest](#issho-v1-SendChatMessageRequest) | [SendChatMessageResponse](#issho-v1-SendChatMessageResponse) | SendChatMessage posts to the caller&#39;s group chat while it&#39;s open (48 hours before the start until 24 hours after; FAILED_PRECONDITION otherwise). 1–1000 characters. |
| CheckIn | [CheckInRequest](#issho-v1-CheckInRequest) | [CheckInResponse](#issho-v1-CheckInResponse) | CheckIn records that the caller has arrived: only people who confirmed (PERMISSION_DENIED otherwise), from 30 minutes before the start until 3 hours after (FAILED_PRECONDITION otherwise). Checking in again is fine. |
| GetFeedback | [GetFeedbackRequest](#issho-v1-GetFeedbackRequest) | [GetFeedbackResponse](#issho-v1-GetFeedbackResponse) | GetFeedback returns the caller&#39;s feedback form for an outing: the others in their group (first names), and what the caller already said. For anyone who confirmed, even if they cancelled since (to report someone); PERMISSION_DENIED otherwise. Feedback itself is `open` only to people who went ahead with their group, from 3 hours after the start for 14 days. |
| SubmitFeedback | [SubmitFeedbackRequest](#issho-v1-SubmitFeedbackRequest) | [SubmitFeedbackResponse](#issho-v1-SubmitFeedbackResponse) | SubmitFeedback records whether the caller went, their ratings, who they&#39;d meet again and who they&#39;d rather not, replacing anything sent before. Both are private: nobody is told who picked them; two people who picked each other are more likely to be grouped again (and may be offered a crew); &#34;rather not&#34; keeps the two apart for good. FAILED_PRECONDITION outside the window. |
| ReportPerson | [ReportPersonRequest](#issho-v1-ReportPersonRequest) | [ReportPersonResponse](#issho-v1-ReportPersonResponse) | ReportPerson reports someone from the caller&#39;s group to the moderators, from the time the caller confirmed until 30 days after the start. The two are never grouped together again. Reporting the same person about the same outing again replaces the report. |

 



<a name="issho_v1_crews-proto"></a>
<p align="right"><a href="#top">Top</a></p>

## issho/v1/crews.proto



<a name="issho-v1-Crew"></a>

### Crew
A crew, or an offer of one, as the caller sees it.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| id | [string](#string) |  |  |
| status | [CrewStatus](#issho-v1-CrewStatus) |  |  |
| activity_id | [string](#string) |  | What it&#39;s named after (exactly one is set): an activity they all like (&#34;Football crew&#34;) or the area they all live in (&#34;Kichijoji crew&#34;). The app composes the name with CatalogService names. |
| area_id | [string](#string) |  |  |
| offer_until | [google.protobuf.Timestamp](#google-protobuf-Timestamp) |  | When the offer closes. |
| response | [CrewResponse](#issho-v1-CrewResponse) |  |  |
| members | [CrewMember](#issho-v1-CrewMember) | repeated | Everyone who accepted, and while it&#39;s an offer, everyone deciding. |






<a name="issho-v1-CrewMember"></a>

### CrewMember



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| id | [string](#string) |  | Opaque; for reports. |
| name | [string](#string) |  | First name. |
| accepted | [bool](#bool) |  |  |
| me | [bool](#bool) |  | The caller. |






<a name="issho-v1-CrewMessage"></a>

### CrewMessage
A message in a crew&#39;s chat: someone&#39;s text, or a plan Issho suggested
(plan_id set; name and body empty).


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| id | [int64](#int64) |  |  |
| name | [string](#string) |  |  |
| body | [string](#string) |  |  |
| at | [google.protobuf.Timestamp](#google-protobuf-Timestamp) |  |  |
| mine | [bool](#bool) |  |  |
| plan_id | [string](#string) |  |  |






<a name="issho-v1-GetCrewRequest"></a>

### GetCrewRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| crew_id | [string](#string) |  |  |






<a name="issho-v1-GetCrewResponse"></a>

### GetCrewResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| crew | [Crew](#issho-v1-Crew) |  |  |






<a name="issho-v1-LeaveCrewRequest"></a>

### LeaveCrewRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| crew_id | [string](#string) |  |  |
| reason | [LeaveReason](#issho-v1-LeaveReason) |  |  |
| details | [string](#string) |  | Optional, up to 1000 characters. |






<a name="issho-v1-LeaveCrewResponse"></a>

### LeaveCrewResponse







<a name="issho-v1-ListCrewMessagesRequest"></a>

### ListCrewMessagesRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| crew_id | [string](#string) |  |  |
| after_id | [int64](#int64) |  | 0 = the latest messages. |






<a name="issho-v1-ListCrewMessagesResponse"></a>

### ListCrewMessagesResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| messages | [CrewMessage](#issho-v1-CrewMessage) | repeated |  |






<a name="issho-v1-ListMyCrewsRequest"></a>

### ListMyCrewsRequest







<a name="issho-v1-ListMyCrewsResponse"></a>

### ListMyCrewsResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| crews | [Crew](#issho-v1-Crew) | repeated |  |






<a name="issho-v1-ReportCrewMemberRequest"></a>

### ReportCrewMemberRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| crew_id | [string](#string) |  |  |
| member_id | [string](#string) |  | CrewMember.id. |
| reason | [ReportReason](#issho-v1-ReportReason) |  |  |
| details | [string](#string) |  | Optional, up to 1000 characters. |






<a name="issho-v1-ReportCrewMemberResponse"></a>

### ReportCrewMemberResponse







<a name="issho-v1-RespondToCrewRequest"></a>

### RespondToCrewRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| crew_id | [string](#string) |  |  |
| accept | [bool](#bool) |  |  |






<a name="issho-v1-RespondToCrewResponse"></a>

### RespondToCrewResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| crew | [Crew](#issho-v1-Crew) |  |  |






<a name="issho-v1-SendCrewMessageRequest"></a>

### SendCrewMessageRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| crew_id | [string](#string) |  |  |
| body | [string](#string) |  |  |






<a name="issho-v1-SendCrewMessageResponse"></a>

### SendCrewMessageResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| message | [CrewMessage](#issho-v1-CrewMessage) |  |  |





 


<a name="issho-v1-CrewResponse"></a>

### CrewResponse


| Name | Number | Description |
| ---- | ------ | ----------- |
| CREW_RESPONSE_UNSPECIFIED | 0 |  |
| CREW_RESPONSE_OFFERED | 1 |  |
| CREW_RESPONSE_ACCEPTED | 2 |  |



<a name="issho-v1-CrewStatus"></a>

### CrewStatus


| Name | Number | Description |
| ---- | ------ | ----------- |
| CREW_STATUS_UNSPECIFIED | 0 |  |
| CREW_STATUS_OFFERED | 1 | Offered: waiting for answers. |
| CREW_STATUS_ACTIVE | 2 |  |



<a name="issho-v1-LeaveReason"></a>

### LeaveReason


| Name | Number | Description |
| ---- | ------ | ----------- |
| LEAVE_REASON_UNSPECIFIED | 0 |  |
| LEAVE_REASON_UNCOMFORTABLE | 1 | Someone made them uncomfortable. |
| LEAVE_REASON_NOT_INTERESTED | 2 | Not interested any more. |
| LEAVE_REASON_INACTIVE | 3 | Not enough happening. |
| LEAVE_REASON_TOO_MANY_MESSAGES | 4 |  |
| LEAVE_REASON_OTHER | 5 |  |


 

 


<a name="issho-v1-CrewService"></a>

### CrewService
CrewService: crews are standing group chats for people who got on well
at outings. Issho offers one when everyone in it has met everyone else
at an outing, nobody minded anyone, each has a mutual meet-again pick in
it, and they share an activity they like or the area they live in, which
names it. It forms with the people who accept within a week (at least
three), and plans that fit are suggested in its chat every week or so.
Anyone can leave at any time.
Requires `Authorization: Bearer &lt;Supabase access token&gt;`.

HTTP: POST /issho.v1.CrewService/&lt;Method&gt;

| Method Name | Request Type | Response Type | Description |
| ----------- | ------------ | ------------- | ------------|
| ListMyCrews | [ListMyCrewsRequest](#issho-v1-ListMyCrewsRequest) | [ListMyCrewsResponse](#issho-v1-ListMyCrewsResponse) | ListMyCrews returns the caller&#39;s crews and the offers still open to them, newest first. |
| GetCrew | [GetCrewRequest](#issho-v1-GetCrewRequest) | [GetCrewResponse](#issho-v1-GetCrewResponse) | GetCrew returns one of them. NOT_FOUND otherwise. |
| RespondToCrew | [RespondToCrewRequest](#issho-v1-RespondToCrewRequest) | [RespondToCrewResponse](#issho-v1-RespondToCrewResponse) | RespondToCrew accepts or declines an offer while it&#39;s open (the answer can be changed until then). FAILED_PRECONDITION once it has closed. |
| ListCrewMessages | [ListCrewMessagesRequest](#issho-v1-ListCrewMessagesRequest) | [ListCrewMessagesResponse](#issho-v1-ListCrewMessagesResponse) | ListCrewMessages returns the crew&#39;s chat: the latest messages, or those after after_id. Members only (PERMISSION_DENIED otherwise). New messages also arrive over the WebSocket at GET /ws/chat, starting with {&#34;token&#34;, &#34;crewId&#34;, &#34;afterId&#34;} (see the backend README). |
| SendCrewMessage | [SendCrewMessageRequest](#issho-v1-SendCrewMessageRequest) | [SendCrewMessageResponse](#issho-v1-SendCrewMessageResponse) | SendCrewMessage posts 1–1000 characters to the crew&#39;s chat. |
| LeaveCrew | [LeaveCrewRequest](#issho-v1-LeaveCrewRequest) | [LeaveCrewResponse](#issho-v1-LeaveCrewResponse) | LeaveCrew takes the caller out of a crew, with why. The reason is private: the crew isn&#39;t told why, or that they left. |
| ReportCrewMember | [ReportCrewMemberRequest](#issho-v1-ReportCrewMemberRequest) | [ReportCrewMemberResponse](#issho-v1-ReportCrewMemberResponse) | ReportCrewMember reports someone in (or who was in) the caller&#39;s crew to the moderators, as EventService.ReportPerson does for an outing. |

 



<a name="issho_v1_curation-proto"></a>
<p align="right"><a href="#top">Top</a></p>

## issho/v1/curation.proto



<a name="issho-v1-AddVenueRequest"></a>

### AddVenueRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| city_id | [string](#string) |  |  |
| place_id | [string](#string) |  |  |
| category_id | [string](#string) |  |  |






<a name="issho-v1-AddVenueResponse"></a>

### AddVenueResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| area_id | [string](#string) |  |  |






<a name="issho-v1-FoundVenue"></a>

### FoundVenue



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| place_id | [string](#string) |  |  |
| name | [string](#string) |  | From Google, in English (shown, never stored). |
| address | [string](#string) |  |  |
| maps_url | [string](#string) |  |  |
| area_id | [string](#string) |  | The nearest of the city&#39;s areas. |
| area_name | [string](#string) |  |  |
| known | [bool](#bool) |  | Whether the city already has it, and in which categories. |
| status | [VenueStatus](#issho-v1-VenueStatus) |  |  |
| category_ids | [string](#string) | repeated |  |
| addable_category_ids | [string](#string) | repeated | Categories it could be added in (its Google types fit and it&#39;s near enough); empty when it&#39;s known or fits none. |
| flag | [string](#string) |  |  |
| closed | [bool](#bool) |  | Closed for good or for now: it can&#39;t be added. |






<a name="issho-v1-ListVenueCitiesRequest"></a>

### ListVenueCitiesRequest







<a name="issho-v1-ListVenueCitiesResponse"></a>

### ListVenueCitiesResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| cities | [VenueCity](#issho-v1-VenueCity) | repeated |  |
| categories | [VenueCategory](#issho-v1-VenueCategory) | repeated |  |






<a name="issho-v1-ListVenuesRequest"></a>

### ListVenuesRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| city_id | [string](#string) |  |  |
| status | [VenueStatus](#issho-v1-VenueStatus) |  |  |
| area_id | [string](#string) |  | Optional filters. |
| category_id | [string](#string) |  |  |
| offset | [int32](#int32) |  | Places to skip, for the next page. |






<a name="issho-v1-ListVenuesResponse"></a>

### ListVenuesResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| venues | [Venue](#issho-v1-Venue) | repeated |  |
| total | [int32](#int32) |  | How many places match, over all pages. |






<a name="issho-v1-SearchVenuesRequest"></a>

### SearchVenuesRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| city_id | [string](#string) |  |  |
| area_id | [string](#string) |  | &#34;&#34; = every area of the city. |
| query | [string](#string) |  | e.g. &#34;スナック&#34; or a place&#39;s name. |






<a name="issho-v1-SearchVenuesResponse"></a>

### SearchVenuesResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| venues | [FoundVenue](#issho-v1-FoundVenue) | repeated |  |
| searches | [int32](#int32) |  | Google searches made (one per area). |






<a name="issho-v1-SetApprovedVenuesOnlyRequest"></a>

### SetApprovedVenuesOnlyRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| city_id | [string](#string) |  |  |
| approved_venues_only | [bool](#bool) |  |  |






<a name="issho-v1-SetApprovedVenuesOnlyResponse"></a>

### SetApprovedVenuesOnlyResponse







<a name="issho-v1-SetVenueStatusRequest"></a>

### SetVenueStatusRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| city_id | [string](#string) |  |  |
| place_ids | [string](#string) | repeated |  |
| status | [VenueStatus](#issho-v1-VenueStatus) |  |  |






<a name="issho-v1-SetVenueStatusResponse"></a>

### SetVenueStatusResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| changed | [int32](#int32) |  | Places changed (ids not in the city are skipped). |






<a name="issho-v1-Venue"></a>

### Venue



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| place_id | [string](#string) |  |  |
| name | [string](#string) |  | From Google, in English; &#34;&#34; without a Google Maps key or if it failed. |
| address | [string](#string) |  |  |
| maps_url | [string](#string) |  |  |
| area_id | [string](#string) |  |  |
| area_name | [string](#string) |  |  |
| category_ids | [string](#string) | repeated |  |
| status | [VenueStatus](#issho-v1-VenueStatus) |  |  |
| added | [bool](#bool) |  | An admin added it (it wasn&#39;t discovered). |
| plans | [int32](#int32) |  | Draft or open plans that use it. |
| flag | [string](#string) |  | Why the name suggests a closer look, e.g. &#34;snack bar?&#34;; &#34;&#34; if nothing. |
| reviewed_at | [google.protobuf.Timestamp](#google-protobuf-Timestamp) |  |  |






<a name="issho-v1-VenueArea"></a>

### VenueArea



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| id | [string](#string) |  |  |
| name | [string](#string) |  | English name. |






<a name="issho-v1-VenueCategory"></a>

### VenueCategory



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| id | [string](#string) |  |  |
| name | [string](#string) |  | English name. |






<a name="issho-v1-VenueCity"></a>

### VenueCity



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| id | [string](#string) |  |  |
| name | [string](#string) |  | English name. |
| approved_venues_only | [bool](#bool) |  |  |
| candidates | [int32](#int32) |  | Places, not rows: a place in two categories counts once. |
| approved | [int32](#int32) |  |  |
| rejected | [int32](#int32) |  |  |
| areas | [VenueArea](#issho-v1-VenueArea) | repeated |  |





 


<a name="issho-v1-VenueStatus"></a>

### VenueStatus


| Name | Number | Description |
| ---- | ------ | ----------- |
| VENUE_STATUS_UNSPECIFIED | 0 |  |
| VENUE_STATUS_CANDIDATE | 1 |  |
| VENUE_STATUS_APPROVED | 2 |  |
| VENUE_STATUS_REJECTED | 3 |  |


 

 


<a name="issho-v1-CurationService"></a>

### CurationService
CurationService: venue curation in the admin SPA (M8). Discovery keeps
every place whose primary type fits a venue category, which lets through
some that don&#39;t suit a group outing (Japanese snack bars are typed
`bar`). Admins approve or reject places, look for places by name (to
find the snack bars, or a place discovery missed) and add them. A city
can be set to use only approved venues.
Curation applies to a place as a whole, in all its categories. Names and
addresses come live from Google, in English, and are never stored.
Admins only, like ModerationService: anyone else gets PERMISSION_DENIED.

HTTP: POST /issho.v1.CurationService/&lt;Method&gt;

| Method Name | Request Type | Response Type | Description |
| ----------- | ------------ | ------------- | ------------|
| ListVenueCities | [ListVenueCitiesRequest](#issho-v1-ListVenueCitiesRequest) | [ListVenueCitiesResponse](#issho-v1-ListVenueCitiesResponse) | ListVenueCities returns every city with its switch and how many places are in each status, and the venue categories. |
| SetApprovedVenuesOnly | [SetApprovedVenuesOnlyRequest](#issho-v1-SetApprovedVenuesOnlyRequest) | [SetApprovedVenuesOnlyResponse](#issho-v1-SetApprovedVenuesOnlyResponse) | SetApprovedVenuesOnly turns the switch on or off. On, the matcher uses only approved venues in the city, for new plans and to decide which plans are possible; plans already made keep their venues. |
| ListVenues | [ListVenuesRequest](#issho-v1-ListVenuesRequest) | [ListVenuesResponse](#issho-v1-ListVenuesResponse) | ListVenues returns a city&#39;s places in one status, 50 at a time. |
| SetVenueStatus | [SetVenueStatusRequest](#issho-v1-SetVenueStatusRequest) | [SetVenueStatusResponse](#issho-v1-SetVenueStatusResponse) | SetVenueStatus approves, rejects or resets (to candidate) up to 100 places of a city. A rejected place is never picked again; plans that already use it keep it (withdraw them on the Plans page). |
| SearchVenues | [SearchVenuesRequest](#issho-v1-SearchVenuesRequest) | [SearchVenuesResponse](#issho-v1-SearchVenuesResponse) | SearchVenues looks places up by name with Google, in one area or each area of the city (one search per area), and says which the city already has. FAILED_PRECONDITION without a Google Maps key. |
| AddVenue | [AddVenueRequest](#issho-v1-AddVenueRequest) | [AddVenueResponse](#issho-v1-AddVenueResponse) | AddVenue adds a place, approved, in one category. Its Google types must include one of the category&#39;s, and it must be within the category&#39;s search radius of one of the city&#39;s areas (it&#39;s put in the nearest). INVALID_ARGUMENT otherwise, or if it&#39;s closed. |

 



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
| suspended | [bool](#bool) |  | A moderator has suspended the account. Every other call is refused (PERMISSION_DENIED) except signing out and DeleteAccount; the app shows that it&#39;s suspended. |





 

 

 


<a name="issho-v1-IdentityService"></a>

### IdentityService
IdentityService manages the signed-in user&#39;s app account.
Authentication itself is handled by Supabase Auth; every call must send
`Authorization: Bearer &lt;Supabase access token&gt;`.

HTTP: POST /issho.v1.IdentityService/&lt;Method&gt;

| Method Name | Request Type | Response Type | Description |
| ----------- | ------------ | ------------- | ------------|
| GetMe | [GetMeRequest](#issho-v1-GetMeRequest) | [GetMeResponse](#issho-v1-GetMeResponse) | GetMe returns the caller&#39;s account, creating the app-side user row the first time a Supabase user calls the API. It works for suspended accounts too (User.suspended), so the app can say so. |
| DeleteAccount | [DeleteAccountRequest](#issho-v1-DeleteAccountRequest) | [DeleteAccountResponse](#issho-v1-DeleteAccountResponse) | DeleteAccount permanently deletes the caller&#39;s Supabase auth user and all of their app data (cascaded from auth.users). The client should sign out afterwards; existing access tokens stop working once they expire. Returns FAILED_PRECONDITION if the server has no Supabase secret key configured. |

 



<a name="issho_v1_metrics-proto"></a>
<p align="right"><a href="#top">Top</a></p>

## issho/v1/metrics.proto



<a name="issho-v1-CityNow"></a>

### CityNow



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| people | [int32](#int32) |  | People living in the city (not deleted or suspended). |
| open_plans | [int32](#int32) |  |  |
| waiting | [int32](#int32) |  | Signups waiting on open plans. |
| upcoming_outings | [int32](#int32) |  | Scheduled outings still to happen. |
| active_crews | [int32](#int32) |  |  |






<a name="issho-v1-GetCityMetricsRequest"></a>

### GetCityMetricsRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| city_id | [string](#string) |  |  |
| weeks | [int32](#int32) |  |  |






<a name="issho-v1-GetCityMetricsResponse"></a>

### GetCityMetricsResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| weeks | [WeekMetrics](#issho-v1-WeekMetrics) | repeated |  |
| now | [CityNow](#issho-v1-CityNow) |  |  |






<a name="issho-v1-WeekMetrics"></a>

### WeekMetrics



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| week | [string](#string) |  | Monday, YYYY-MM-DD. |
| attendees | [int32](#int32) |  | People who went to an outing that week. |
| repeat_attendees | [int32](#int32) |  | The north star: of those, people with at least two outings so far. |
| first_timers | [int32](#int32) |  | Of those, people at their first outing. |
| groups | [int32](#int32) |  | Groups that went ahead. |
| invited | [int32](#int32) |  | Places at those groups: invited, said they&#39;d go, went. |
| confirmed | [int32](#int32) |  |  |
| attended | [int32](#int32) |  |  |
| plan_rating | [double](#double) |  | Average feedback ratings (1 to 5); 0 = no feedback. |
| group_rating | [double](#double) |  |  |
| picks_again | [int32](#int32) |  | Meet-again picks: &#34;again&#34; out of all picks, and pairs who both said again. |
| picks | [int32](#int32) |  |  |
| mutual_pairs | [int32](#int32) |  |  |
| new_people | [int32](#int32) |  | Accounts made by people living in the city. |
| crews_formed | [int32](#int32) |  |  |
| reports | [int32](#int32) |  | Reports about people met at the city&#39;s outings. |





 

 

 


<a name="issho-v1-MetricsService"></a>

### MetricsService
MetricsService: domain metrics in the admin SPA (M8), per city and week.
The north star is weekly attendees who have been to more than one
outing (repeat_attendees). Weeks start on Monday in the city&#39;s time
zone; an outing counts in the week of its local date. &#34;Attended&#34; is
confirmed and checked in, or said so in their feedback, as matching&#39;s
reliability counts it.
Admins only, like ModerationService: anyone else gets PERMISSION_DENIED.

HTTP: POST /issho.v1.MetricsService/&lt;Method&gt;

| Method Name | Request Type | Response Type | Description |
| ----------- | ------------ | ------------- | ------------|
| GetCityMetrics | [GetCityMetricsRequest](#issho-v1-GetCityMetricsRequest) | [GetCityMetricsResponse](#issho-v1-GetCityMetricsResponse) | GetCityMetrics returns the last `weeks` weeks (1 to 52; 0 = 12), the current week last (still in progress), and the city now. NOT_FOUND for an unknown city. |

 



<a name="issho_v1_moderation-proto"></a>
<p align="right"><a href="#top">Top</a></p>

## issho/v1/moderation.proto



<a name="issho-v1-CrewLeave"></a>

### CrewLeave



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| id | [int64](#int64) |  |  |
| crew_id | [string](#string) |  |  |
| crew_name | [string](#string) |  | &#34;Football&#34; or &#34;Kichijoji&#34;: what the crew shares. |
| crew_status | [string](#string) |  | offered, active, lapsed or closed. |
| user_id | [string](#string) |  | Empty if they&#39;ve deleted their account. |
| user_name | [string](#string) |  |  |
| reason | [LeaveReason](#issho-v1-LeaveReason) |  |  |
| details | [string](#string) |  |  |
| created_at | [google.protobuf.Timestamp](#google-protobuf-Timestamp) |  |  |






<a name="issho-v1-GetModeratorRequest"></a>

### GetModeratorRequest







<a name="issho-v1-GetModeratorResponse"></a>

### GetModeratorResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| user_id | [string](#string) |  |  |
| email | [string](#string) |  |  |






<a name="issho-v1-GetPersonRequest"></a>

### GetPersonRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| user_id | [string](#string) |  |  |






<a name="issho-v1-GetPersonResponse"></a>

### GetPersonResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| person | [Person](#issho-v1-Person) |  |  |






<a name="issho-v1-LiftSuspensionRequest"></a>

### LiftSuspensionRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| user_id | [string](#string) |  |  |
| note | [string](#string) |  | Up to 2000 characters. |






<a name="issho-v1-LiftSuspensionResponse"></a>

### LiftSuspensionResponse







<a name="issho-v1-ListCrewLeavesRequest"></a>

### ListCrewLeavesRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| reason | [LeaveReason](#issho-v1-LeaveReason) |  | UNSPECIFIED = every reason. |






<a name="issho-v1-ListCrewLeavesResponse"></a>

### ListCrewLeavesResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| leaves | [CrewLeave](#issho-v1-CrewLeave) | repeated |  |






<a name="issho-v1-ListFlagsRequest"></a>

### ListFlagsRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| resolved | [bool](#bool) |  | false = open flags; true = resolved ones. |






<a name="issho-v1-ListFlagsResponse"></a>

### ListFlagsResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| flags | [ModerationFlag](#issho-v1-ModerationFlag) | repeated |  |






<a name="issho-v1-ListReportsRequest"></a>

### ListReportsRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| status | [ReportStatus](#issho-v1-ReportStatus) |  | UNSPECIFIED = every status. |
| reported_id | [string](#string) |  | Only reports about this person; empty = everyone. |






<a name="issho-v1-ListReportsResponse"></a>

### ListReportsResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| reports | [ModerationReport](#issho-v1-ModerationReport) | repeated |  |






<a name="issho-v1-ListSuspendedRequest"></a>

### ListSuspendedRequest







<a name="issho-v1-ListSuspendedResponse"></a>

### ListSuspendedResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| people | [SuspendedPerson](#issho-v1-SuspendedPerson) | repeated |  |






<a name="issho-v1-ListTranslationLanguagesRequest"></a>

### ListTranslationLanguagesRequest







<a name="issho-v1-ListTranslationLanguagesResponse"></a>

### ListTranslationLanguagesResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| languages | [TranslationLanguage](#issho-v1-TranslationLanguage) | repeated |  |






<a name="issho-v1-ListTranslationsRequest"></a>

### ListTranslationsRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| lang | [string](#string) |  |  |
| unreviewed_only | [bool](#bool) |  |  |
| search | [string](#string) |  | Matches the English source or the translation; empty = all. |
| offset | [int32](#int32) |  | How many to skip (pages of 100). |






<a name="issho-v1-ListTranslationsResponse"></a>

### ListTranslationsResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| translations | [Translation](#issho-v1-Translation) | repeated |  |






<a name="issho-v1-ModerationFlag"></a>

### ModerationFlag



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| id | [int64](#int64) |  |  |
| user_id | [string](#string) |  |  |
| user_name | [string](#string) |  |  |
| kind | [FlagKind](#issho-v1-FlagKind) |  |  |
| details_json | [string](#string) |  | The numbers behind the flag, as JSON (see FlagKind). |
| note | [string](#string) |  | The note left when it was resolved. |
| created_at | [google.protobuf.Timestamp](#google-protobuf-Timestamp) |  |  |
| updated_at | [google.protobuf.Timestamp](#google-protobuf-Timestamp) |  | The details were last refreshed (the flag is updated while open). |
| resolved_at | [google.protobuf.Timestamp](#google-protobuf-Timestamp) |  |  |
| resolved_by | [string](#string) |  |  |






<a name="issho-v1-ModerationReport"></a>

### ModerationReport



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| id | [string](#string) |  |  |
| reporter_id | [string](#string) |  | Empty if the reporter has deleted their account. |
| reporter_name | [string](#string) |  |  |
| reported_id | [string](#string) |  |  |
| reported_name | [string](#string) |  |  |
| reports_about_reported | [int32](#int32) |  | How many reports there are about the reported person in all. |
| reason | [ReportReason](#issho-v1-ReportReason) |  |  |
| details | [string](#string) |  |  |
| status | [ReportStatus](#issho-v1-ReportStatus) |  |  |
| note | [string](#string) |  | The moderator&#39;s note on the decision. |
| event_id | [string](#string) |  | Where they met: an outing (event_*) or a crew (crew_*), if still there. |
| event_title | [string](#string) |  |  |
| event_date | [string](#string) |  | The outing&#39;s date (YYYY-MM-DD, in the city&#39;s time zone) and part of the day (morning, afternoon, evening). |
| event_block | [string](#string) |  |  |
| crew_id | [string](#string) |  |  |
| crew_name | [string](#string) |  |  |
| created_at | [google.protobuf.Timestamp](#google-protobuf-Timestamp) |  |  |
| updated_at | [google.protobuf.Timestamp](#google-protobuf-Timestamp) |  | Last changed by the reporter (reporting again about the same outing or crew). Later than reviewed_at: changed since the decision. |
| reviewed_at | [google.protobuf.Timestamp](#google-protobuf-Timestamp) |  |  |
| reviewed_by | [string](#string) |  |  |






<a name="issho-v1-Person"></a>

### Person



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| user_id | [string](#string) |  |  |
| name | [string](#string) |  | Empty if they haven&#39;t finished onboarding (no profile). |
| email | [string](#string) |  |  |
| joined_at | [google.protobuf.Timestamp](#google-protobuf-Timestamp) |  |  |
| gender | [string](#string) |  | woman, man, nonbinary or unspecified; empty without a profile. |
| birth_year | [int32](#int32) |  |  |
| home_area | [string](#string) |  | English name of their home area, and its city. |
| city_id | [string](#string) |  |  |
| reports_about | [int32](#int32) |  | Reports about them (in all, and still open) and reports they made. |
| open_reports_about | [int32](#int32) |  |  |
| reports_made | [int32](#int32) |  |  |
| suspended | [bool](#bool) |  |  |
| suspensions | [Suspension](#issho-v1-Suspension) | repeated | Newest first. |
| flags | [ModerationFlag](#issho-v1-ModerationFlag) | repeated | Open ones first. |
| outings | [PersonOuting](#issho-v1-PersonOuting) | repeated | Newest first, at most 50. |






<a name="issho-v1-PersonOuting"></a>

### PersonOuting



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| event_id | [string](#string) |  |  |
| title | [string](#string) |  |  |
| date | [string](#string) |  | YYYY-MM-DD in the city&#39;s time zone, and morning, afternoon or evening. |
| block | [string](#string) |  |  |
| event_status | [string](#string) |  | scheduled, done or cancelled. |
| response | [string](#string) |  | invited, confirmed, declined or dropped (confirmed, then cancelled). |
| checked_in | [bool](#bool) |  |  |
| feedback | [OutingFeedback](#issho-v1-OutingFeedback) |  | Their feedback: SAID_WENT, SAID_DIDNT_GO, or UNSPECIFIED (none). |






<a name="issho-v1-PersonSummary"></a>

### PersonSummary



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| user_id | [string](#string) |  |  |
| name | [string](#string) |  | Empty if they haven&#39;t finished onboarding. |
| email | [string](#string) |  |  |
| suspended | [bool](#bool) |  |  |






<a name="issho-v1-ResolveFlagRequest"></a>

### ResolveFlagRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| flag_id | [int64](#int64) |  |  |
| note | [string](#string) |  | Up to 2000 characters. |






<a name="issho-v1-ResolveFlagResponse"></a>

### ResolveFlagResponse







<a name="issho-v1-ResolveReportRequest"></a>

### ResolveReportRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| report_id | [string](#string) |  |  |
| status | [ReportStatus](#issho-v1-ReportStatus) |  | ACTIONED, DISMISSED, or OPEN to reopen it. |
| note | [string](#string) |  | Up to 2000 characters. |






<a name="issho-v1-ResolveReportResponse"></a>

### ResolveReportResponse







<a name="issho-v1-ReviewTranslationRequest"></a>

### ReviewTranslationRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| source_hash | [string](#string) |  |  |
| lang | [string](#string) |  |  |
| text | [string](#string) |  | The corrected text (or the same text, to confirm it). Not empty. |






<a name="issho-v1-ReviewTranslationResponse"></a>

### ReviewTranslationResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| translation | [Translation](#issho-v1-Translation) |  |  |






<a name="issho-v1-SearchPeopleRequest"></a>

### SearchPeopleRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| query | [string](#string) |  | At least 2 characters. |






<a name="issho-v1-SearchPeopleResponse"></a>

### SearchPeopleResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| people | [PersonSummary](#issho-v1-PersonSummary) | repeated |  |






<a name="issho-v1-SuspendPersonRequest"></a>

### SuspendPersonRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| user_id | [string](#string) |  |  |
| reason | [string](#string) |  | Why, for moderators: 1-2000 characters. The person isn&#39;t shown it. |






<a name="issho-v1-SuspendPersonResponse"></a>

### SuspendPersonResponse







<a name="issho-v1-SuspendedPerson"></a>

### SuspendedPerson



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| user_id | [string](#string) |  |  |
| name | [string](#string) |  |  |
| reason | [string](#string) |  |  |
| since | [google.protobuf.Timestamp](#google-protobuf-Timestamp) |  |  |
| suspended_by | [string](#string) |  |  |






<a name="issho-v1-Suspension"></a>

### Suspension



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| id | [int64](#int64) |  |  |
| reason | [string](#string) |  |  |
| created_by | [string](#string) |  |  |
| created_at | [google.protobuf.Timestamp](#google-protobuf-Timestamp) |  |  |
| lifted_at | [google.protobuf.Timestamp](#google-protobuf-Timestamp) |  | Unset while it&#39;s in force. |
| lifted_by | [string](#string) |  |  |
| lift_note | [string](#string) |  |  |






<a name="issho-v1-Translation"></a>

### Translation



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| source_hash | [string](#string) |  | Identifies the English source and its context, with lang. |
| lang | [string](#string) |  |  |
| source | [string](#string) |  |  |
| context | [string](#string) |  | What the text is (&#34;name of a city&#34;, a UI string), as given to the translator. |
| text | [string](#string) |  |  |
| model | [string](#string) |  | Which translator wrote it. |
| reviewed | [bool](#bool) |  |  |
| reviewed_at | [google.protobuf.Timestamp](#google-protobuf-Timestamp) |  |  |
| created_at | [google.protobuf.Timestamp](#google-protobuf-Timestamp) |  |  |






<a name="issho-v1-TranslationLanguage"></a>

### TranslationLanguage



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| lang | [string](#string) |  | ISO 639-1 code, e.g. &#34;ja&#34;. |
| total | [int32](#int32) |  |  |
| unreviewed | [int32](#int32) |  |  |





 


<a name="issho-v1-FlagKind"></a>

### FlagKind


| Name | Number | Description |
| ---- | ------ | ----------- |
| FLAG_KIND_UNSPECIFIED | 0 |  |
| FLAG_KIND_PICK_GENDER_LEAN | 1 | Their meet-again picks lean heavily towards one gender, compared with who they met. Details: gender, picked (picks of that gender), picks, met (people met), met_share (that gender&#39;s share of the people they met), p_value (how likely if picks ignored gender). |



<a name="issho-v1-OutingFeedback"></a>

### OutingFeedback


| Name | Number | Description |
| ---- | ------ | ----------- |
| OUTING_FEEDBACK_UNSPECIFIED | 0 |  |
| OUTING_FEEDBACK_SAID_WENT | 1 |  |
| OUTING_FEEDBACK_SAID_DIDNT_GO | 2 |  |



<a name="issho-v1-ReportStatus"></a>

### ReportStatus


| Name | Number | Description |
| ---- | ------ | ----------- |
| REPORT_STATUS_UNSPECIFIED | 0 |  |
| REPORT_STATUS_OPEN | 1 | Waiting for a moderator. |
| REPORT_STATUS_ACTIONED | 2 | A moderator acted on it (see the note). |
| REPORT_STATUS_DISMISSED | 3 | A moderator looked and took no action. |


 

 


<a name="issho-v1-ModerationService"></a>

### ModerationService
ModerationService: the moderation queue in the admin SPA. Reports about
people, flags raised by the system (picks leaning towards one gender),
why people left crews, and corrections to machine translations.
Admins only: requires `Authorization: Bearer &lt;Supabase access token&gt;` for
an account with `app_metadata.admin = true` (set with `tool set-admin`);
anyone else gets PERMISSION_DENIED.

HTTP: POST /issho.v1.ModerationService/&lt;Method&gt;

| Method Name | Request Type | Response Type | Description |
| ----------- | ------------ | ------------- | ------------|
| GetModerator | [GetModeratorRequest](#issho-v1-GetModeratorRequest) | [GetModeratorResponse](#issho-v1-GetModeratorResponse) | GetModerator says who the caller is. The admin SPA calls it after sign-in to find out whether the account is an admin. |
| ListReports | [ListReportsRequest](#issho-v1-ListReportsRequest) | [ListReportsResponse](#issho-v1-ListReportsResponse) | ListReports returns reports, open ones first (oldest first), then the rest newest first. At most 200. |
| ResolveReport | [ResolveReportRequest](#issho-v1-ResolveReportRequest) | [ResolveReportResponse](#issho-v1-ResolveReportResponse) | ResolveReport records the decision on a report. It can be changed later (resolve again). The pair stays apart in matching either way. |
| ListFlags | [ListFlagsRequest](#issho-v1-ListFlagsRequest) | [ListFlagsResponse](#issho-v1-ListFlagsResponse) | ListFlags returns open flags (oldest first) or resolved ones (newest first). At most 200. |
| ResolveFlag | [ResolveFlagRequest](#issho-v1-ResolveFlagRequest) | [ResolveFlagResponse](#issho-v1-ResolveFlagResponse) | ResolveFlag closes an open flag with a note. FAILED_PRECONDITION if it&#39;s already resolved. A later signal opens a new flag. |
| ListCrewLeaves | [ListCrewLeavesRequest](#issho-v1-ListCrewLeavesRequest) | [ListCrewLeavesResponse](#issho-v1-ListCrewLeavesResponse) | ListCrewLeaves returns why people left crews, newest first. At most 200. |
| ListTranslationLanguages | [ListTranslationLanguagesRequest](#issho-v1-ListTranslationLanguagesRequest) | [ListTranslationLanguagesResponse](#issho-v1-ListTranslationLanguagesResponse) | ListTranslationLanguages returns the languages with machine translations, with how many still need a look. |
| ListTranslations | [ListTranslationsRequest](#issho-v1-ListTranslationsRequest) | [ListTranslationsResponse](#issho-v1-ListTranslationsResponse) | ListTranslations returns one language&#39;s translations, unreviewed ones first, 100 at a time. |
| ReviewTranslation | [ReviewTranslationRequest](#issho-v1-ReviewTranslationRequest) | [ReviewTranslationResponse](#issho-v1-ReviewTranslationResponse) | ReviewTranslation corrects (or confirms) a translation. A reviewed translation is never replaced by a machine one. INVALID_ARGUMENT if the text drops or adds a {placeholder} of the English source. |
| SearchPeople | [SearchPeopleRequest](#issho-v1-SearchPeopleRequest) | [SearchPeopleResponse](#issho-v1-SearchPeopleResponse) | SearchPeople finds people by name or email (contains, any case) or exact user id. At most 50. |
| GetPerson | [GetPersonRequest](#issho-v1-GetPersonRequest) | [GetPersonResponse](#issho-v1-GetPersonResponse) | GetPerson returns what a moderator needs to decide about someone: who they are, report counts, flags, suspensions and recent outings. NOT_FOUND if there&#39;s no such account. |
| SuspendPerson | [SuspendPersonRequest](#issho-v1-SuspendPersonRequest) | [SuspendPersonResponse](#issho-v1-SuspendPersonResponse) | SuspendPerson keeps someone out of Issho until it&#39;s lifted: the API refuses their calls (GetMe says they&#39;re suspended; signing out and deleting the account still work), the chat socket refuses them, matching leaves them out and they get no notifications. They&#39;re also taken out of their open signups, outings that haven&#39;t started, and crews. Suspending someone already suspended only repeats that removal. INVALID_ARGUMENT for the caller&#39;s own account. |
| LiftSuspension | [LiftSuspensionRequest](#issho-v1-LiftSuspensionRequest) | [LiftSuspensionResponse](#issho-v1-LiftSuspensionResponse) | LiftSuspension ends someone&#39;s suspension. They can use Issho again, but aren&#39;t put back into what they were taken out of. FAILED_PRECONDITION if they aren&#39;t suspended. |
| ListSuspended | [ListSuspendedRequest](#issho-v1-ListSuspendedRequest) | [ListSuspendedResponse](#issho-v1-ListSuspendedResponse) | ListSuspended returns everyone suspended now, most recent first. At most 200. |

 



<a name="issho_v1_push-proto"></a>
<p align="right"><a href="#top">Top</a></p>

## issho/v1/push.proto



<a name="issho-v1-RegisterPushTokenRequest"></a>

### RegisterPushTokenRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| token | [string](#string) |  | From expo-notifications&#39; getExpoPushTokenAsync: &#34;ExponentPushToken[…]&#34;. |
| platform | [PushPlatform](#issho-v1-PushPlatform) |  |  |






<a name="issho-v1-RegisterPushTokenResponse"></a>

### RegisterPushTokenResponse







<a name="issho-v1-UnregisterPushTokenRequest"></a>

### UnregisterPushTokenRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| token | [string](#string) |  |  |






<a name="issho-v1-UnregisterPushTokenResponse"></a>

### UnregisterPushTokenResponse






 


<a name="issho-v1-PushPlatform"></a>

### PushPlatform


| Name | Number | Description |
| ---- | ------ | ----------- |
| PUSH_PLATFORM_UNSPECIFIED | 0 |  |
| PUSH_PLATFORM_IOS | 1 |  |
| PUSH_PLATFORM_ANDROID | 2 |  |
| PUSH_PLATFORM_WEB | 3 |  |


 

 


<a name="issho-v1-PushService"></a>

### PushService
PushService registers the caller&#39;s devices for push notifications (a
plan you signed up for has a date; your group is going ahead or
cancelled). The app calls RegisterPushToken after sign-in and whenever its
Expo push token changes, and UnregisterPushToken when signing out.
Requires `Authorization: Bearer &lt;Supabase access token&gt;`, and the account
must exist (`IdentityService.GetMe` creates it).

HTTP: POST /issho.v1.PushService/&lt;Method&gt;

| Method Name | Request Type | Response Type | Description |
| ----------- | ------------ | ------------- | ------------|
| RegisterPushToken | [RegisterPushTokenRequest](#issho-v1-RegisterPushTokenRequest) | [RegisterPushTokenResponse](#issho-v1-RegisterPushTokenResponse) | RegisterPushToken stores the device&#39;s token for the caller. A token registered by another account moves to the caller (the last person to sign in on a device gets its notifications). |
| UnregisterPushToken | [UnregisterPushTokenRequest](#issho-v1-UnregisterPushTokenRequest) | [UnregisterPushTokenResponse](#issho-v1-UnregisterPushTokenResponse) | UnregisterPushToken stops notifications to the device for the caller. Unknown tokens are ignored. |

 



<a name="issho_v1_review-proto"></a>
<p align="right"><a href="#top">Top</a></p>

## issho/v1/review.proto



<a name="issho-v1-ApprovePlanRequest"></a>

### ApprovePlanRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| plan_id | [string](#string) |  |  |






<a name="issho-v1-ApprovePlanResponse"></a>

### ApprovePlanResponse







<a name="issho-v1-ApproveProposalRequest"></a>

### ApproveProposalRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| event_id | [string](#string) |  |  |






<a name="issho-v1-ApproveProposalResponse"></a>

### ApproveProposalResponse







<a name="issho-v1-CreatePlanRequest"></a>

### CreatePlanRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| city_id | [string](#string) |  |  |
| area_id | [string](#string) |  |  |
| activity_id | [string](#string) |  |  |
| follow_up_id | [string](#string) |  | Optional: an activity after the first (&#34;then ramen&#34;). |
| venue_place_id | [string](#string) |  | A place from ListVenueOptions; &#34;&#34; = pick one as the matcher would (none for activities that need only a meeting point). |
| follow_up_venue_place_id | [string](#string) |  |  |
| primary_language | [string](#string) |  | ISO 639-1. secondary_language &#34;&#34; = none. |
| secondary_language | [string](#string) |  |  |
| title | [string](#string) |  | As in EditPlanRequest. |
| summary | [string](#string) |  |  |






<a name="issho-v1-CreatePlanResponse"></a>

### CreatePlanResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| plan_id | [string](#string) |  |  |






<a name="issho-v1-EditPlanRequest"></a>

### EditPlanRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| plan_id | [string](#string) |  |  |
| title | [string](#string) |  | Up to 80 characters; &#34;&#34; = made from the activities. |
| summary | [string](#string) |  | Up to 500 characters; &#34;&#34; = none. |






<a name="issho-v1-EditPlanResponse"></a>

### EditPlanResponse







<a name="issho-v1-FindPlanPlacesRequest"></a>

### FindPlanPlacesRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| plan_id | [string](#string) |  |  |
| query | [string](#string) |  |  |






<a name="issho-v1-FindPlanPlacesResponse"></a>

### FindPlanPlacesResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| places | [FoundPlace](#issho-v1-FoundPlace) | repeated |  |






<a name="issho-v1-FoundPlace"></a>

### FoundPlace



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| place_id | [string](#string) |  |  |
| name | [string](#string) |  | From Google, in English (shown, never stored). |
| address | [string](#string) |  |  |
| maps_url | [string](#string) |  |  |
| primary_type | [string](#string) |  | Google&#39;s primary type, e.g. &#34;shinto_shrine&#34;. |
| meters | [int32](#int32) |  | From the area&#39;s centre. |
| closed | [bool](#bool) |  |  |






<a name="issho-v1-GetReviewEventRequest"></a>

### GetReviewEventRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| event_id | [string](#string) |  |  |






<a name="issho-v1-GetReviewEventResponse"></a>

### GetReviewEventResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| event | [ReviewEvent](#issho-v1-ReviewEvent) |  |  |
| groups | [ReviewGroup](#issho-v1-ReviewGroup) | repeated |  |






<a name="issho-v1-GetReviewPlanRequest"></a>

### GetReviewPlanRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| plan_id | [string](#string) |  |  |






<a name="issho-v1-GetReviewPlanResponse"></a>

### GetReviewPlanResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| plan | [ReviewPlan](#issho-v1-ReviewPlan) |  |  |
| steps | [PlanStep](#issho-v1-PlanStep) | repeated | As people would see them, in English. |
| stops | [ReviewStop](#issho-v1-ReviewStop) | repeated | A walk&#39;s stops as stored, for editing (SetPlanStops); empty for other plans. Same order as steps. |
| venue_place_id | [string](#string) |  | For other plans, the venues as stored (&#34;&#34; = none). |
| follow_up_venue_place_id | [string](#string) |  |  |
| upcoming_events | [int32](#int32) |  | Scheduled events of this plan still to come, whose people see edits. |






<a name="issho-v1-ListReviewCitiesRequest"></a>

### ListReviewCitiesRequest







<a name="issho-v1-ListReviewCitiesResponse"></a>

### ListReviewCitiesResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| cities | [ReviewCity](#issho-v1-ReviewCity) | repeated |  |






<a name="issho-v1-ListReviewEventsRequest"></a>

### ListReviewEventsRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| city_id | [string](#string) |  |  |
| status | [ReviewEventStatus](#issho-v1-ReviewEventStatus) |  | Required. Scheduled events are listed from today. |






<a name="issho-v1-ListReviewEventsResponse"></a>

### ListReviewEventsResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| events | [ReviewEvent](#issho-v1-ReviewEvent) | repeated |  |






<a name="issho-v1-ListReviewPlansRequest"></a>

### ListReviewPlansRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| city_id | [string](#string) |  |  |
| status | [ReviewPlanStatus](#issho-v1-ReviewPlanStatus) |  | Required. |






<a name="issho-v1-ListReviewPlansResponse"></a>

### ListReviewPlansResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| plans | [ReviewPlan](#issho-v1-ReviewPlan) | repeated |  |






<a name="issho-v1-ListVenueOptionsRequest"></a>

### ListVenueOptionsRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| area_id | [string](#string) |  |  |
| activity_id | [string](#string) |  |  |






<a name="issho-v1-ListVenueOptionsResponse"></a>

### ListVenueOptionsResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| venues | [VenueOption](#issho-v1-VenueOption) | repeated |  |






<a name="issho-v1-MoveProposalMemberRequest"></a>

### MoveProposalMemberRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| event_id | [string](#string) |  |  |
| user_id | [string](#string) |  |  |
| group_id | [string](#string) |  |  |






<a name="issho-v1-MoveProposalMemberResponse"></a>

### MoveProposalMemberResponse







<a name="issho-v1-RejectPlanRequest"></a>

### RejectPlanRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| plan_id | [string](#string) |  |  |






<a name="issho-v1-RejectPlanResponse"></a>

### RejectPlanResponse







<a name="issho-v1-RejectProposalRequest"></a>

### RejectProposalRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| event_id | [string](#string) |  |  |






<a name="issho-v1-RejectProposalResponse"></a>

### RejectProposalResponse







<a name="issho-v1-RemoveProposalMemberRequest"></a>

### RemoveProposalMemberRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| event_id | [string](#string) |  |  |
| user_id | [string](#string) |  |  |






<a name="issho-v1-RemoveProposalMemberResponse"></a>

### RemoveProposalMemberResponse







<a name="issho-v1-ReviewCity"></a>

### ReviewCity



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| id | [string](#string) |  |  |
| name | [string](#string) |  | English name. |
| review | [bool](#bool) |  |  |
| drafts | [int32](#int32) |  |  |
| open | [int32](#int32) |  |  |






<a name="issho-v1-ReviewEvent"></a>

### ReviewEvent



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| id | [string](#string) |  |  |
| plan_id | [string](#string) |  |  |
| status | [ReviewEventStatus](#issho-v1-ReviewEventStatus) |  |  |
| title | [string](#string) |  | English: the plan&#39;s stored title, or made from its activities. |
| area_name | [string](#string) |  |  |
| date | [string](#string) |  | Local date (YYYY-MM-DD) and part of the day; the start time. |
| block | [string](#string) |  |  |
| starts_at | [google.protobuf.Timestamp](#google-protobuf-Timestamp) |  |  |
| confirm_by | [google.protobuf.Timestamp](#google-protobuf-Timestamp) |  | Invitees answer by then. |
| forced | [bool](#bool) |  | Scheduled at the signup deadline with whoever could go, rather than because a group reached its target size. |
| groups | [int32](#int32) |  |  |
| people | [int32](#int32) |  | Invited or going. |
| created_at | [google.protobuf.Timestamp](#google-protobuf-Timestamp) |  |  |
| time_zone | [string](#string) |  | The city&#39;s time zone (IANA), for showing and setting local times. |






<a name="issho-v1-ReviewGroup"></a>

### ReviewGroup



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| id | [string](#string) |  |  |
| quorum | [int32](#int32) |  | Confirmed straight away at quorum; at the deadline it goes ahead with at least min_attendance. |
| min_attendance | [int32](#int32) |  |  |
| score | [float](#float) |  |  |
| status | [string](#string) |  | inviting, confirmed or cancelled. |
| meeting_point | [string](#string) |  | English; &#34;&#34; for none. |
| members | [ReviewMember](#issho-v1-ReviewMember) | repeated |  |






<a name="issho-v1-ReviewMember"></a>

### ReviewMember



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| user_id | [string](#string) |  |  |
| name | [string](#string) |  |  |
| gender | [string](#string) |  | woman, man, nonbinary or unspecified. |
| birth_year | [int32](#int32) |  |  |
| response | [string](#string) |  | invited, confirmed, declined or dropped. |
| signed_up | [bool](#bool) |  | Still signed up for the plan: in a proposal, someone who withdrew since (false) is left out when it&#39;s approved. |






<a name="issho-v1-ReviewPlan"></a>

### ReviewPlan



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| id | [string](#string) |  |  |
| city_id | [string](#string) |  |  |
| status | [ReviewPlanStatus](#issho-v1-ReviewPlanStatus) |  |  |
| closed_reason | [string](#string) |  | done, no_group, withdrawn or rejected, for a closed plan. |
| title | [string](#string) |  | The English title people see: the stored one, or made from the activities. stored_title is &#34;&#34; when it&#39;s made from the activities. |
| stored_title | [string](#string) |  |  |
| summary | [string](#string) |  | English summary; &#34;&#34; for none. |
| area_id | [string](#string) |  |  |
| area_name | [string](#string) |  |  |
| activity_id | [string](#string) |  |  |
| follow_up_id | [string](#string) |  |  |
| primary_language | [string](#string) |  |  |
| secondary_language | [string](#string) |  |  |
| walk | [bool](#bool) |  | A composed walk (its stops are in GetReviewPlanResponse). |
| audience | [int32](#int32) |  | From the matcher: how many people it suits, and its score; 0 for a plan made by hand. |
| score | [float](#float) |  |  |
| signups | [int32](#int32) |  |  |
| window_start | [string](#string) |  | The current round&#39;s window (YYYY-MM-DD, local) and signup deadline. |
| window_days | [int32](#int32) |  |  |
| signup_deadline | [google.protobuf.Timestamp](#google-protobuf-Timestamp) |  |  |
| created_at | [google.protobuf.Timestamp](#google-protobuf-Timestamp) |  |  |
| created_by | [string](#string) |  | The admin who made it by hand; &#34;&#34; = the matcher. |
| reviewed_by | [string](#string) |  |  |
| reviewed_at | [google.protobuf.Timestamp](#google-protobuf-Timestamp) |  |  |






<a name="issho-v1-ReviewStop"></a>

### ReviewStop
A walk&#39;s stop. The role follows the position: &#34;meet&#34; first, &#34;finish&#34;
last, &#34;stop&#34; between.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| place_id | [string](#string) |  |  |
| role | [string](#string) |  |  |
| minutes | [int32](#int32) |  | Time there: 0 to 120. |
| note | [string](#string) |  | English, at most 300 characters; people see it translated. |






<a name="issho-v1-SetCityReviewRequest"></a>

### SetCityReviewRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| city_id | [string](#string) |  |  |
| review | [bool](#bool) |  |  |






<a name="issho-v1-SetCityReviewResponse"></a>

### SetCityReviewResponse







<a name="issho-v1-SetGroupMeetingPointRequest"></a>

### SetGroupMeetingPointRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| group_id | [string](#string) |  |  |
| text | [string](#string) |  | Up to 300 characters; &#34;&#34; clears it. |






<a name="issho-v1-SetGroupMeetingPointResponse"></a>

### SetGroupMeetingPointResponse







<a name="issho-v1-SetPlanStopsRequest"></a>

### SetPlanStopsRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| plan_id | [string](#string) |  |  |
| stops | [ReviewStop](#issho-v1-ReviewStop) | repeated |  |






<a name="issho-v1-SetPlanStopsResponse"></a>

### SetPlanStopsResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| warnings | [string](#string) | repeated | Rules the walk breaks, in words; empty when it fits. |
| minutes | [int32](#int32) |  | The whole outing in minutes (at the stops and walking); 0 when the route couldn&#39;t be worked out. |






<a name="issho-v1-SetPlanVenueRequest"></a>

### SetPlanVenueRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| plan_id | [string](#string) |  |  |
| follow_up | [bool](#bool) |  | The follow-up&#39;s venue rather than the first activity&#39;s. |
| place_id | [string](#string) |  |  |






<a name="issho-v1-SetPlanVenueResponse"></a>

### SetPlanVenueResponse







<a name="issho-v1-SetProposalStartRequest"></a>

### SetProposalStartRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| event_id | [string](#string) |  |  |
| date | [string](#string) |  | Local to the city: YYYY-MM-DD and HH:MM. |
| time | [string](#string) |  |  |






<a name="issho-v1-SetProposalStartResponse"></a>

### SetProposalStartResponse







<a name="issho-v1-VenueOption"></a>

### VenueOption



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| place_id | [string](#string) |  |  |
| name | [string](#string) |  | From Google, in English; &#34;&#34; without GOOGLE_MAPS_API_KEY or if it failed. |
| address | [string](#string) |  |  |
| maps_url | [string](#string) |  |  |
| category_id | [string](#string) |  | Our venue category, and candidate or approved. |
| status | [string](#string) |  |  |






<a name="issho-v1-WithdrawPlanRequest"></a>

### WithdrawPlanRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| plan_id | [string](#string) |  |  |






<a name="issho-v1-WithdrawPlanResponse"></a>

### WithdrawPlanResponse






 


<a name="issho-v1-ReviewEventStatus"></a>

### ReviewEventStatus


| Name | Number | Description |
| ---- | ------ | ----------- |
| REVIEW_EVENT_STATUS_UNSPECIFIED | 0 |  |
| REVIEW_EVENT_STATUS_PROPOSED | 1 | Waiting for an admin. |
| REVIEW_EVENT_STATUS_SCHEDULED | 2 | Invitations sent. |



<a name="issho-v1-ReviewPlanStatus"></a>

### ReviewPlanStatus


| Name | Number | Description |
| ---- | ------ | ----------- |
| REVIEW_PLAN_STATUS_UNSPECIFIED | 0 |  |
| REVIEW_PLAN_STATUS_DRAFT | 1 |  |
| REVIEW_PLAN_STATUS_OPEN | 2 |  |
| REVIEW_PLAN_STATUS_CLOSED | 3 |  |


 

 


<a name="issho-v1-ReviewService"></a>

### ReviewService
ReviewService: plan review in the admin SPA (M8), for launching a city
by hand. A city can be put under review: its suggested plans (walks
included) then wait as drafts, seen by nobody, until an admin approves
or rejects them. Admins can also edit a plan&#39;s English title and summary,
withdraw an open plan, and create plans from scratch in any city.
Text is English, as stored; people see it translated on demand.
Admins only, like ModerationService: anyone else gets PERMISSION_DENIED.

HTTP: POST /issho.v1.ReviewService/&lt;Method&gt;

| Method Name | Request Type | Response Type | Description |
| ----------- | ------------ | ------------- | ------------|
| ListReviewCities | [ListReviewCitiesRequest](#issho-v1-ListReviewCitiesRequest) | [ListReviewCitiesResponse](#issho-v1-ListReviewCitiesResponse) | ListReviewCities returns every city with its review switch and plan counts. |
| SetCityReview | [SetCityReviewRequest](#issho-v1-SetCityReviewRequest) | [SetCityReviewResponse](#issho-v1-SetCityReviewResponse) | SetCityReview turns review on or off for a city. Turning it off doesn&#39;t open the drafts already waiting: approve or reject them. |
| ListReviewPlans | [ListReviewPlansRequest](#issho-v1-ListReviewPlansRequest) | [ListReviewPlansResponse](#issho-v1-ListReviewPlansResponse) | ListReviewPlans returns a city&#39;s plans in one state: drafts oldest first, open and closed plans newest first. At most 200. |
| GetReviewPlan | [GetReviewPlanRequest](#issho-v1-GetReviewPlanRequest) | [GetReviewPlanResponse](#issho-v1-GetReviewPlanResponse) | GetReviewPlan returns one plan with its stops (venue names from Google, in English). NOT_FOUND if there&#39;s no such plan. |
| ApprovePlan | [ApprovePlanRequest](#issho-v1-ApprovePlanRequest) | [ApprovePlanResponse](#issho-v1-ApprovePlanResponse) | ApprovePlan opens a draft, with a fresh signup window from today. FAILED_PRECONDITION if it isn&#39;t a draft. |
| RejectPlan | [RejectPlanRequest](#issho-v1-RejectPlanRequest) | [RejectPlanResponse](#issho-v1-RejectPlanResponse) | RejectPlan turns a draft down. The matcher won&#39;t suggest the same plan again until the cool-down passes (two weeks). FAILED_PRECONDITION if it isn&#39;t a draft. |
| EditPlan | [EditPlanRequest](#issho-v1-EditPlanRequest) | [EditPlanResponse](#issho-v1-EditPlanResponse) | EditPlan replaces a draft&#39;s or open plan&#39;s English title and summary. FAILED_PRECONDITION for a closed plan. |
| WithdrawPlan | [WithdrawPlanRequest](#issho-v1-WithdrawPlanRequest) | [WithdrawPlanResponse](#issho-v1-WithdrawPlanResponse) | WithdrawPlan closes an open plan (&#34;withdrawn&#34;). People who signed up see it closed; nobody is notified. Groups already scheduled go ahead. FAILED_PRECONDITION if it isn&#39;t open. |
| ListVenueOptions | [ListVenueOptionsRequest](#issho-v1-ListVenueOptionsRequest) | [ListVenueOptionsResponse](#issho-v1-ListVenueOptionsResponse) | ListVenueOptions returns the venues found in an area for an activity&#39;s venue categories (not rejected ones), approved first, with names from Google in English. At most 20. Empty for activities that need no venue. |
| CreatePlan | [CreatePlanRequest](#issho-v1-CreatePlanRequest) | [CreatePlanResponse](#issho-v1-CreatePlanResponse) | CreatePlan opens a plan made by hand, with a signup window from today. It opens straight away, also in a city under review. INVALID_ARGUMENT for an unknown area or activity, an area or activity not in the city, or a venue not found for the activity there. |
| FindPlanPlaces | [FindPlanPlacesRequest](#issho-v1-FindPlanPlacesRequest) | [FindPlanPlacesResponse](#issho-v1-FindPlanPlacesResponse) | Editing a plan&#39;s stops (draft or open plans; an event already scheduled shows the change to the people going). FindPlanPlaces looks places up by name with Google within 3 km of the plan&#39;s area. FAILED_PRECONDITION without a Google Maps key. |
| SetPlanStops | [SetPlanStopsRequest](#issho-v1-SetPlanStopsRequest) | [SetPlanStopsResponse](#issho-v1-SetPlanStopsResponse) | SetPlanStops replaces a walk&#39;s stops, in walking order: the first is where people meet, the last where they sit down at the end. Each place is checked with Google (it exists, isn&#39;t closed, is within 3 km of the area&#39;s centre, appears once) and the walking route is worked out again in this order. A walk outside the composer&#39;s rules (2 to 4 hours, no walk between stops over 20 minutes, 100 minutes of walking at most) is saved anyway, with warnings. INVALID_ARGUMENT for a plan that isn&#39;t a walk, fewer than 3 or more than 10 places, or a place that fails. |
| SetPlanVenue | [SetPlanVenueRequest](#issho-v1-SetPlanVenueRequest) | [SetPlanVenueResponse](#issho-v1-SetPlanVenueResponse) | SetPlanVenue changes the venue of a plan that isn&#39;t a walk, or of its follow-up: one of the venues found for the activity in the area (ListVenueOptions), or &#34;&#34; to let the matcher pick. The walk between the two is worked out again. |
| ListReviewEvents | [ListReviewEventsRequest](#issho-v1-ListReviewEventsRequest) | [ListReviewEventsResponse](#issho-v1-ListReviewEventsResponse) | ListReviewEvents returns a city&#39;s proposals, or its upcoming scheduled events (to set meeting points), soonest first. At most 200. |
| GetReviewEvent | [GetReviewEventRequest](#issho-v1-GetReviewEventRequest) | [GetReviewEventResponse](#issho-v1-GetReviewEventResponse) | GetReviewEvent returns one with its groups and who&#39;s in them. NOT_FOUND if there&#39;s no such event. |
| MoveProposalMember | [MoveProposalMemberRequest](#issho-v1-MoveProposalMemberRequest) | [MoveProposalMemberResponse](#issho-v1-MoveProposalMemberResponse) | MoveProposalMember moves someone to another group of the proposal. FAILED_PRECONDITION if it&#39;s no longer a proposal; NOT_FOUND for a group not in it or someone not in it. |
| RemoveProposalMember | [RemoveProposalMemberRequest](#issho-v1-RemoveProposalMemberRequest) | [RemoveProposalMemberResponse](#issho-v1-RemoveProposalMemberResponse) | RemoveProposalMember takes someone out of the proposal; they stay signed up for the plan&#39;s next round. A group left empty goes. |
| SetProposalStart | [SetProposalStartRequest](#issho-v1-SetProposalStartRequest) | [SetProposalStartResponse](#issho-v1-SetProposalStartResponse) | SetProposalStart moves the proposal to another local date and time in the city (its part of the day follows the time). FAILED_PRECONDITION if the day before has already begun: people need until then to answer. |
| SetGroupMeetingPoint | [SetGroupMeetingPointRequest](#issho-v1-SetGroupMeetingPointRequest) | [SetGroupMeetingPointResponse](#issho-v1-SetGroupMeetingPointResponse) | SetGroupMeetingPoint sets where exactly a group meets, in English (&#34;&#34; clears it), for a proposal or a scheduled event; stored in the event&#39;s languages too when the server can translate. People who confirmed see it from 24 hours before the start. |
| ApproveProposal | [ApproveProposalRequest](#issho-v1-ApproveProposalRequest) | [ApproveProposalResponse](#issho-v1-ApproveProposalResponse) | ApproveProposal sends the invitations, as if just scheduled: people who withdrew (or were suspended) since are left out, everyone else is invited and told, and the plan moves to its next round for whoever is still waiting, or closes. FAILED_PRECONDITION if the plan moved on, answering would no longer be possible in time (set a later start), or a group has fewer people than its minimum. |
| RejectProposal | [RejectProposalRequest](#issho-v1-RejectProposalRequest) | [RejectProposalResponse](#issho-v1-RejectProposalResponse) | RejectProposal deletes it. Everyone stays signed up, and the plan isn&#39;t scheduled again for a day. |

 



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

