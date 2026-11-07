# Vietnam: verifying accounts by phone number

Research note, 2026-11-07, for [#505](https://github.com/shrujancheruku/issho/issues/505). Not legal advice. It asks how Issho can meet Decree 333/2026 Art. 16.2. That article requires verifying an account by a Vietnamese mobile number, or by ID number for someone without one. The note also covers what each option costs and what Supabase Auth supports. Background: `docs/research/vietnam-data-localization.md` on branch `research/vietnam-data-localization`, section 1, "Duties that apply now".

Today: sign-in is Supabase Auth email OTP and the Go API verifies the JWT. Phone sign-up is off in `supabase/config.toml` (`[auth.sms] enable_signup = false`, `[auth.sms.twilio] enabled = false`). The operator is a US individual with no company until the public launch.

**Verified** means read in the primary source: the gazette PDF, the vendor's own docs or pricing page, or Supabase Auth source code. **Mirror** means a full-text republication of a law, not the gazette. **Unverified** means it comes from secondary commentary, from a vendor claim that is not specific to Vietnam, or from our own reading.

## Answer in short

- **Recommended:** keep email OTP for sign-in, and add a **Vietnamese phone number as a second, verified attribute**:
  - The client calls `supabase.auth.updateUser({ phone })`, then `verifyOtp({ type: 'phone_change' })`.
  - Supabase sends the code through **Twilio Verify**.
  - The Go API lets an account post or use interactive features only when the JWT's `phone` claim is a confirmed `+84` number (Art. 16.2(d)).
- **Cost:** about **US$0.34 per successful verification** through Twilio Verify (US$0.05 plus US$0.2852 for each SMS to Vietnam). That is roughly **US$170 for 500 verified users** and **US$670 for 2,000**, or about 30% more with retries.
- **Zalo ZNS** costs far less (300 VND per OTP, about US$0.01). But it needs a **verified Zalo Official Account**, and that appears to require a business registration and a +84 number. A US individual cannot get one. Revisit it once there is an entity.
- **Foreign visitors** have no lawful fallback that Issho can check. The decree's fallback is a personal identification number, which only Vietnamese citizens have. Its other fallback is an e-ID such as VNeID, which needs a residence card. So a visitor's practical path is a **local SIM**, registered with a passport. Without one they can browse but not take part.
- **PDPL Art. 29.2** (no ID images) does not limit any option here. No option collects an image.

## 1. What the law requires (verified)

Source: Decree 333/2026/NĐ-CP, [Công báo 499, 2026-09-04](https://congbaocdn.chinhphu.vn/180507251028987904/2026/9/4/470335-1787890927_v1_1788486330_signed.pdf), Art. 16.2, pp. 66–67.

- **(a)** "Thực hiện xác thực thông tin người sử dụng tại thời điểm đăng ký tài khoản số theo quy định của pháp luật" (verify user information at the time a digital account is registered).
- **(b)** "Thực hiện xác thực tài khoản bằng số điện thoại di động tại Việt Nam; trường hợp người sử dụng không có số điện thoại di động tại Việt Nam thì thực hiện xác thực bằng số định danh cá nhân hoặc phương thức định danh điện tử hợp pháp khác theo quy định của pháp luật về định danh và xác thực điện tử" (verify the account by a mobile phone number in Vietnam; if the user has none, by personal identification number or another lawful e-ID method under the law on electronic identification and authentication).
- **(c)** Commercial livestreaming must be verified by personal identification number. This does not apply to Issho.
- **(d)** "chỉ cho phép các tài khoản đã được xác thực thực hiện việc đăng tải, chia sẻ thông tin và sử dụng các tính năng tương tác" (only verified accounts may post, share information or use interactive features).

What follows from the text:

- **Verified:** the decree does not say *how* to verify a phone number. It names no OTP, carrier lookup or provider. It does not mention passports or "người nước ngoài" (foreigners) anywhere in Art. 16. A text search of the whole decree confirms this.
- **Verified:** Art. 16.2(d) gates posting, sharing and interactive features, not account creation or reading. An unverified account may exist and browse.
- **Verified:** Art. 19.1(b) lists "số điện thoại đăng ký được gắn với tài khoản" (the registered phone number attached to the account) among the data to be stored in Vietnam. For a foreign provider this applies only after an MPS decision (Art. 19.3; see the localisation note). Collecting the number adds it to that data set.
- **Unverified (our reading):** a phone number that has passed an SMS OTP counts as "xác thực tài khoản bằng số điện thoại di động". This is how Vietnamese platforms have done it since Decree 147/2024. It works because every Vietnamese SIM is registered to a verified identity (section 5).
- **Unverified:** whether a **foreign passport number** counts as "phương thức định danh điện tử hợp pháp khác". It is not an e-ID method under the e-ID law, and the decree does not name it. Ask counsel.

## 2. SMS OTP to Vietnam

### Carrier rules (verified, vendor docs)

- **[Twilio, Vietnam SMS guidelines](https://www.twilio.com/en-us/guidelines/vn/sms):**
  - "Vietnam mobile operators require pre-registration of Alphanumeric Sender IDs and message templates", for both international and domestic traffic.
  - Dynamic sender IDs, domestic long codes and short codes are not supported. Numeric senders are "overwritten with generic Alphanumeric Sender IDs", best effort only.
  - Complete blocking of unregistered sender IDs began **2025-08-25**.
  - Registration takes about **5 weeks**. It needs "a colored copy of the business registration certificate", plus proof of the right to the brand if the sender ID is not the company's name.
  - The body must contain the brand or app name. It must not contain URLs, chat links or phone numbers. Only transactional traffic is allowed.
- **[AWS End User Messaging, Vietnam registration](https://docs.aws.amazon.com/sms-voice/latest/userguide/registrations-vietnam.html):**
  - A non-Vietnamese company files a BM04 letter of authorisation "on company letterhead, stamped, and signed with company seal on all pages".
  - It also files company registration documents, a tax ID (EIN or VAT) and proof of its connection to the sender ID.
  - "One-time passwords" is a listed use case, and promotional content is not allowed.
- **Verified, both sources:** registering an own sender ID needs a **company**. A US individual cannot register one through Twilio or AWS.

### Vietnamese law on sender names (mirror)

- **Mirror:** Decree 91/2020/NĐ-CP on spam, read on [hethongphapluat.com, Chapter 3](https://hethongphapluat.com/nghi-dinh-91-2020-nd-cp-ve-chong-tin-nhan-rac-thu-dien-tu-rac-cuoc-goi-rac/chuong-3):
  - Art. 13.8: only messages that **advertise** need a registered "tên định danh" (brand name).
  - Art. 23.2: "Mọi tổ chức, cá nhân đều có quyền đăng ký" (any organisation or individual may register).
  - Art. 24.2: an individual files a certified copy of an ID card or **passport**, plus Mẫu 01.
- **Unverified (our reading):** the law does not oblige anyone to register a brand name for OTP messages. That requirement comes from carrier policy, which the aggregators enforce. Registering as an individual with the ministry would not help: Twilio and AWS still ask for company documents.
- **Unverified:** the Decree 91 text was read in a mirror, not the gazette. The registry at `tendinhdanh.ais.gov.vn` was not checked. The ministry may have moved after the 2025 MIC–MOST merger.

### Twilio Verify: the route that needs no own registration

- **Verified:**
  - Twilio's [error 30018 page](https://www.twilio.com/docs/api/errors/30018) ("destination carrier requires sender ID pre-registration") recommends Twilio Verify, "which will automatically take care of all applicable registrations".
  - [Verify pricing](https://www.twilio.com/en-us/verify/pricing): "$0.05 per successful verification plus standard channel fees". SMS attempts "are always charged", even if not delivered.
  - [SMS pricing, Vietnam](https://www.twilio.com/en-us/sms/pricing/vn): **US$0.2852 per segment** ("current as of October 2026"). The page gives no per-carrier split.
- **Unverified:** that Verify's managed sender is registered **for Vietnam specifically**. The claim on Twilio's error page is generic. Also unverified: Verify's delivery rate to Viettel, Mobifone and Vinaphone. Before launch, send a test to a Vietnamese SIM on each of the three carriers.
- **Unverified:** whether Twilio accepts an individual's account for Verify traffic to Vietnam. Twilio accounts can be personal, but this was not confirmed for Vietnam.
- **Not checked:** Vonage's Vietnam page returned HTTP 403. Bird (MessageBird) and Vonage Verify pricing for Vietnam were not read. Vietnamese aggregators (eSMS, VietGuys) sell domestic brand-name SMS for a few hundred VND per message, but they contract with Vietnamese entities. Both points are unverified.

### Cost at Da Nang's scale

| Verified users | Twilio Verify (US$0.335 each, ×1.3 for retries) | Zalo ZNS OTP (300 VND, about US$0.0115) |
|---|---|---|
| 500 | about US$220 | about US$7 |
| 2,000 | about US$870 | about US$30 |
| 10,000 | about US$4,350 | about US$150 |

- The per-message prices are verified (section 2 above and section 4 below).
- The 1.3× retry factor and the rate of about 26,000 VND per US dollar are **assumptions**. Without retries, Twilio is US$0.335 per verified user.
- Each user verifies once, so the cost scales with sign-ups, not activity.

## 3. Supabase Auth support (verified)

- **Built-in providers ([Phone Login docs](https://supabase.com/docs/guides/auth/phone-login)):** "MessageBird, Twilio, Vonage, and TextLocal (community-supported)". The source ([`sms_provider.go`](https://github.com/supabase/auth/blob/master/internal/api/sms_provider/sms_provider.go)) also has **`twilio_verify`**.
  - WhatsApp works only with Twilio and Twilio Verify.
  - By default "a user can only request an OTP once every 60 seconds".
  - The docs warn that some countries regulate SMS, with India's DLT as the example.
- **Send SMS hook ([docs](https://supabase.com/docs/guides/auth/auth-hooks/send-sms-hook)):**
  - It "replaces Supabase's built-in SMS sending" with any HTTP endpoint or Postgres function. It receives `user` and `sms.otp`, is signed with Standard Webhooks, and treats an empty 200 response as success.
  - The [hooks page](https://supabase.com/docs/guides/auth/auth-hooks) lists it under the **Free and Pro** plans.
  - This is the route to ZNS or a Vietnamese aggregator later: the Go API, or a small function, sends the code through Zalo instead.
- **Adding a phone to an email account (source):**
  - [`user.go`](https://github.com/supabase/auth/blob/master/internal/api/user.go): `PUT /user` with a new `phone` sets `phone_change` and sends an OTP of type `phone_change`.
  - [`verify.go`](https://github.com/supabase/auth/blob/master/internal/api/verify.go): verifying that OTP calls `ConfirmPhoneChange`, which sets `phone` and `phone_confirmed_at`.
  - [`phone.go`](https://github.com/supabase/auth/blob/master/internal/api/phone.go): this path goes through the Send SMS hook when the hook is enabled.
  - `PUT /user` refuses a number that another user already has (`IsDuplicatedPhone`). That makes one number map to one account.
  - Sign-up by phone can stay off (`[auth.sms] enable_signup = false`), because this is phone *change*, not phone sign-up.
- **The JWT carries it (source):**
  - [`tokens/service.go`](https://github.com/supabase/auth/blob/master/internal/tokens/service.go): `AccessTokenClaims` has `phone`.
  - The Go API can gate interactive RPCs on a `+84` prefix in that claim with no extra database call. It can also check `auth.users.phone_confirmed_at` for certainty.
- **Unverified:** the `verifyOtp` JS reference page shows only `type: 'sms'`. That `phone_change` is accepted comes from the server source, not the client docs. Also unverified: that `twilio_verify` handles `phone_change`. The server routes both types through the same provider code (`verify.go`), but this was not tested.

## 4. Zalo

- **Verified:** [Zalo Business Solutions ZNS pricing](https://zalo.solutions/zns/pricing) puts an **OTP template at 300 VND** per message, excluding VAT. Other templates cost 200–300 VND.
- **Verified:** [Infobip's Zalo docs](https://www.infobip.com/docs/zalo/get-started.md) say that to create an Official Account "you must have a personal Zalo account and a Vietnam phone number (+84)", and that Zalo must verify the OA before use.
- **Unverified (secondary, [eSMS help centre](https://helpcenter.esms.vn/thong-tin-dich-vu/tra-cuu-thong-tin-ho-so/zalo/zalo-oa/quy-trinh-tao-zalo-oa)):** OA verification needs a business registration certificate, as an original or a notarised copy, with a matching industry code. No source says whether a **foreign** company, or any individual, can pass. Zalo's own OA help pages were not reachable, and the ZNS developer page rendered empty.
- **Our reading:** a US individual cannot get a verified OA, so ZNS is closed until there is an entity. That would most likely be a Vietnamese one.
- **Limit:** ZNS reaches only people who use Zalo. It is a cheaper channel for the same phone check, not a way around it. Visitors without a Vietnamese number are not on it.

## 5. Foreign residents and visitors

- **Mirror and secondary, Decree 163/2024/NĐ-CP Arts. 17–18 and Circular 08/2026/TT-BKHCN:** a foreigner registers a Vietnamese SIM with a valid passport or an e-ID account. The carrier must authenticate the subscriber, for foreigners through immigration data or the e-ID account. The full text was not read.
  - So a verified `+84` number stands for a verified identity, for foreigners as well as citizens.
- **Unverified (secondary, [VCCI copy of Decree 69/2024](https://vcci.com.vn/legal-document/69-2024-nd-cp) and provincial police notices):** "số định danh của người nước ngoài" (a foreigner's identification number) exists. But a level-2 VNeID account for a foreigner needs a **temporary or permanent residence card** and a Vietnamese mobile number.
  - A tourist on a DL or EV visa has neither.
  - Decree 320/2026 amends Decree 69; it was not read.
- **Verified (Art. 16.2(b)):** the fallback is "số định danh cá nhân". Under the Law on Identification, that is issued to Vietnamese citizens. The law itself was not re-read for this note.
- **Unverified:** whether a third party like Issho could check an ID number at all. Checking one would need a connection to the national population database or VNeID's authentication service. That connection seems to require an agreement with the MPS (C06), and a US individual is unlikely to get one.
- **Paths, as we read them:**
  - **Resident foreigner:** usually has a Vietnamese SIM, so the same phone check.
  - **Visitor with a local SIM:** a tourist SIM registered with a passport, so the same phone check.
  - **Visitor without a Vietnamese SIM:** no fallback Issho can check today. Let them sign in and browse, but block Plans, joining and messaging (Art. 16.2(d)). Whether a self-declared passport number is enough is a question for counsel.

## 6. PDPL Art. 29.2

- **Verified (background note):** the PDPL bars social networks from requiring an image or video of an ID document to verify an account.
- None of the options above needs an image. Phone OTP and ZNS collect only a number, and an ID-number fallback would be a number too. So Art. 29.2 rules out photo-of-passport flows but nothing proposed here.

## Open questions

1. Does Twilio Verify deliver to all three major Vietnamese carriers under a sender it has registered? Test with real SIMs before the soft launch.
2. Does Twilio accept an individual's account for Verify traffic to Vietnam?
3. Can a foreign company, or an individual, get a verified Zalo OA? Ask Zalo Business support once an entity exists.
4. For counsel:
   - Is SMS OTP enough to meet "xác thực tài khoản bằng số điện thoại"?
   - Does a passport number count as a lawful fallback?
   - Is a visitor without a SIM simply limited to read-only use?
