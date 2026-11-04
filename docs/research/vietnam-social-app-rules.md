# Research: Vietnam's rules for a social app

Issue [#335](https://github.com/shrujancheruku/issho/issues/335), part of map [#334](https://github.com/shrujancheruku/issho/issues/334). Researched 2026-11-04 against main at `92f1ad3`. This branch is never merged; anything adopted moves into `docs/ROADMAP.md` (Launch gates) and the READMEs. This is a reading of the texts, not legal advice.

Starting point: what Issho holds about a person and where it goes.

- **Data:** email (sign-in), display name, **birth year** (18+ enforced in `profile/validate.go`), **gender** and same-gender preference, home **area** (a ward, picked by hand; the app doesn't read device location), languages, free-text interests, plan sign-ups and attendance, event feedback and meet-again picks, **group chat** (events and crews), reports, moderation flags, suspensions and push tokens. There are no profile photos in the schema yet.
- **Where it goes:** Supabase Postgres in **Seoul** for `issho-dev` (`ap-northeast-2`), with production planned for **Tokyo** (`ap-northeast-1`, [docs/supabase-hosted.md](../supabase-hosted.md)). The API runs on a Compute Engine VM in Tokyo, backups go to Cloudflare R2, errors go to Sentry (EU region), and text goes to Google (Gemini, Cloud Translation, Maps) and push messages to Expo ([docs/deploy.md](../deploy.md), [docs/architecture.md](../architecture.md)). **None of it is in Vietnam.**
- **Operator:** one owner, no Vietnamese company. Where Issho is legally based is an open question (below), and the answer changes everything under Decree 147.

## Answer in brief

- **Decree 13/2023 no longer applies.** From **2026-01-01** the **Personal Data Protection Law (Law 91/2025/QH15)** and its decree, **Decree 356/2025/ND-CP**, replaced it. Fines come from **Decree 330/2026/ND-CP** (in force 2026-08-19).
- **The PDPL applies from the first user.** It has no size threshold for the core rules: consent per purpose, never by default, recorded and provable; a published privacy policy; a way to see, correct and delete data; and breach reports within 72 hours. Issho already has in-app deletion. It lacks a privacy policy, consent records and a breach plan.
- **Hosting in Tokyo or the US makes no difference. Hosting anywhere outside Vietnam is a "cross-border transfer".** The owner must prepare a **cross-border transfer impact assessment dossier** and file it with the Ministry of Public Security within **60 days of the first transfer**. The small-business relief does **not** cover this dossier. It is the one filing that can't be avoided.
- **The processing impact assessment dossier** (also due within 60 days) and the **data protection officer** can be skipped by small enterprises and startups for 5 years (micro-enterprises indefinitely). The relief is lost when a business processes sensitive data, and Decree 356 counts *"data tracking behavior and activities in the use of … social networks, online communication services"* as sensitive. A sole owner with no enterprise isn't named in the relief either. **Cheapest safe option: file both dossiers.** They are paperwork, not fees.
- **Decree 147 almost certainly treats Issho as a social network.** Its definition covers any app that lets users "exchange, interact and share information with each other", and group chat does that. What it requires depends on who runs Issho:
  - **Run from abroad** (a foreign person or company, servers abroad): Issho is a *foreign social network*. The heavy duties, including **verifying accounts with a Vietnamese mobile number**, apply only once it averages **100,000 visits a month from Vietnam over 6 months**, or leases data storage in Vietnam. A waitlist soft launch is far below that. **Phone verification is not needed now.**
  - **Run by a company in Vietnam:** Issho is a *domestic social network*. It needs a Vietnamese legal entity, a **notification certificate** (or a **licence** from 10,000 visits or more than 1,000 regular users a month), a `.vn` main domain, user data on servers in Vietnam, a Vietnamese national responsible for content, and phone verification from day one. An individual can't hold this at all.
- **Phone verification dates:** Decree 147 took effect **2024-12-25**. Providers it covered had to verify existing accounts within 90 days, by **2025-03-25**. A foreign provider that crosses 100,000 visits later has 60 days to notify the authority. The decree sets no separate grace period for verification.
- **The 2025 Cybersecurity Law (Law 116/2025/QH15, in force 2026-07-01)** says foreign providers handling Vietnamese users' data must store it in Vietnam and open a branch or office. The details are left to a Government decree. Under the 2018 regime this was triggered only after a Ministry of Public Security order. Not verified for 2026.
- **Waiting for the Stripe Identity verification planned for the public launch:** PDPL Article 29(2) bars social networks from requiring **images or videos of ID papers as an account authentication factor**. Revisit before building it for Vietnam.

## Personal data protection: Law 91/2025 and Decree 356/2025

Law 91/2025/QH15 was passed 2025-06-26 and took effect **2026-01-01** (Art. 38(1)). Decree 356/2025/ND-CP of 2025-12-31 details it and took effect the same day. Its Art. 42(2) ends Decree 13/2023/ND-CP. Sources: [Law 91, English](https://english.luatvietnam.vn/dan-su/law-on-personal-data-protection-law-no-91-2025-qh15-405135-d1.html); [Decree 356, English](https://english.luatvietnam.vn/decree-no-356-2025-nd-cp-dated-december-31-2025-of-the-government-detailing-a-number-of-articles-and-measures-for-the-implementation-of-the-law-on-p-422896-doc1.html); [Decree 356, Vietnamese](https://luatvietnam.vn/dan-su/nghi-dinh-356-2025-nd-cp-quy-dinh-chi-tiet-luat-bao-ve-du-lieu-ca-nhan-422896-d1.html). The official file is on [vbpl.moj.gov.vn](https://vbpl.moj.gov.vn/TW/Lists/vbpq/Attachments/187276/ND.356.2025.doc). The English texts are unofficial translations.

### Does it apply to Issho?

Yes. Law Art. 1(2) and Decree 356 Art. 2 cover Vietnamese and foreign organisations and individuals in Vietnam, and *"foreign agencies, organizations and individuals directly involved in or related to the processing of personal data of Vietnamese citizens"*. There is no size floor and no exemption for individuals. Strictly, a foreign operator is covered for **Vietnamese citizens'** data. Treat every user the same.

### What it requires, from the first user

| Requirement | Source | Issho today | Cheapest compliant option |
|---|---|---|---|
| Consent based on clear knowledge of the data type, purpose, controller and the person's rights; explicit, per purpose, printable or copyable; *"Silence or non-response is not considered consent"* | Law Art. 9(2)–(4) | Onboarding records the accepted code-of-conduct version only | Before the profile is saved, show a consent screen per purpose (matching, chat, push, analytics later), unticked by default. Store who agreed to what version, and when |
| Store consent; *the burden of proof is on the controller*; no default consent; tell people when data is sensitive | Decree 356 Art. 6(2)–(4) | No consent records | A `consents` row (user, purpose, policy version, time). Name the possibly sensitive items in the notice (below) |
| Withdraw consent, view, correct, delete, restrict, object | Law Art. 4(1), Art. 10 | In-app deletion and profile editing exist | Add "withdraw consent" (in practice, delete the account or turn off a purpose) and an email address for requests |
| Social networks: clear notice of what is collected; **no ID images or videos as an authentication factor**; a way to refuse cookies; a "do not track" option or tracking only with consent; no reading messages without consent; **publish a privacy policy**; access, correction and deletion | Law Art. 29(1)–(6) | No privacy policy; no tracking yet (PostHog is planned, ROADMAP) | A privacy policy page (also needed by both stores). Ship PostHog only with an opt-in. The web app should set no non-essential cookies |
| Report a breach to the authority *"within 72 hours after detecting a violation"* that could harm people | Law Art. 23(1) | No plan | A one-page breach runbook in `docs/` |
| Children: consent of the child and the legal representative from age 7 for publishing private information | Law Art. 24 | 18+ enforced by birth year | Nothing more, beyond keeping the 18+ gate |

Decree 356 Art. 6(1) accepts consent given *"via email or platforms with consent mechanisms"* and other printable formats, so an in-app tick box with a stored record works.

### Is anything Issho holds "sensitive"?

Decree 356 Art. 4(1) lists sensitive data. **Gender and date of birth are basic data** (Art. 3), not sensitive. A hand-picked ward is not an *"individual's position determined through positioning services"* (Art. 4(1)(h)). Two items are close calls:

- **(l)** *"Data tracking behavior and activities in the use of telecommunications services, social networks, online communication services, and other services in cyberspace"*. Attendance, check-ins, feedback and meet-again picks could fall under this.
- **(c)** *"Information on private life, personal secrets, or family secrets"*. Chat contents could fall under this.

The decree gives no guidance on either. This matters because **processing sensitive data cancels the small-business relief** (next section).

### The filings: impact assessments and the data protection officer

| | Cross-border transfer dossier | Processing impact assessment dossier | Data protection staff |
|---|---|---|---|
| Rule | Law Art. 20; Decree 356 Art. 17–18 | Law Art. 21; Decree 356 Art. 19 | Law Art. 33(2); Decree 356 Art. 13 |
| Triggered by | Any of: data stored in Vietnam moved abroad; sending it to a recipient abroad; *"using platforms outside the territory … to process personal data collected in Vietnam"* (Law Art. 20(1)(c)); Decree 356 Art. 17(1) adds storage on servers abroad or with foreign cloud providers | Starting to process personal data (the controller) | "Agencies and organizations" must name qualified staff or hire a data protection service |
| Deadline | One original dossier within **60 days of the first transfer** (Art. 20(2); Decree Art. 18(4)) | Within **60 days of starting processing** (Art. 21(1); Decree Art. 19(4)) | From when processing starts |
| Form | Report on Form 09 plus Form 01a/01b, the transfer contract and data protection policies (Decree Art. 18(2)) | Report on Form 10 plus Form 02a/02b (Decree Art. 19(2)) | Written appointment; college degree, 2 years' experience, data protection training (Decree Art. 13) |
| Updates | Every 6 months when something has changed; at once on reorganisation, closure or a change of service provider (Law Art. 22) | Same | — |
| Exemptions | Only: state agencies; storing *employees'* data in the cloud; *"the personal data subject transferring his/her personal data across the border"*; journalism; data already public; emergencies; HR; travel, payment, hotel, visa or scholarship procedures (Law Art. 20(6), Decree Art. 17(3)) | State agencies | — |
| Small-business relief (Law Art. 38(2)–(3), Decree 356 Art. 41) | **None** | Small enterprises and startups may skip it for 5 years from 2026-01-01; micro-enterprises and household businesses skip it altogether | Same as the processing assessment |

The relief does **not** apply to a business that *"directly process[es] sensitive personal data"*, provides processing services, or has reached **100,000 data subjects** in total (Decree 356 Art. 41).

The authority answers within 15 days. An incomplete file has 30 days to be fixed (Decree Art. 18(5)–(6), 19(5)). Filings go to the specialised agency under the **Ministry of Public Security** (Decree Art. 39).

**For Issho:**

- **The cross-border dossier is unavoidable** while the database, API, backups, Sentry, Google and Expo are all abroad. The region doesn't matter: Tokyo, Seoul and the US are all "outside Vietnam", and the law has no list of approved countries. The "data subject transferring their own data" exemption is meant for a person sending their own data, not for a provider's architecture. Don't rely on it. Hosting in Vietnam would only shrink the dossier, since Sentry, Gemini, Translation and Expo would still be abroad. **Cheapest:** file one dossier listing every foreign recipient (Supabase/AWS Tokyo, Google Cloud Tokyo, Cloudflare, Sentry, Google APIs, Expo) within 60 days of the first real user in Vietnam, and update it when a vendor changes.
- **The processing assessment and the data protection officer:** the relief needs an *enterprise* (small, startup, micro or household). It's lost if behaviour tracking (Art. 4(1)(l)) counts as sensitive. **Cheapest safe option:** file the processing dossier too. Most of its content (purposes, data types, flows, consent method, security, risks; Decree Art. 19(3)) is the same as the cross-border one. Art. 33(2) speaks of "agencies and organizations", so a sole individual arguably needs no data protection officer. If Issho becomes a small company, the owner can name themself if they meet Decree 356 Art. 13, or skip it under the relief if no sensitive data is processed.

### Penalties

- **Ceilings:** up to 5% of the previous year's revenue for an organisation's cross-border violations, 10 times the proceeds for selling data, and **VND 3 billion** otherwise. Individuals pay half (Law Art. 8(3)–(6)).
- **Decree 330/2026/ND-CP**, in force 2026-08-19 ([VOV](https://vov.vn/chinh-tri/tu-1982026-tu-y-thu-thap-xu-ly-du-lieu-ca-nhan-bi-phat-toi-70-trieu-dong-post1326424.vov)): for example, VND 30–50 million for processing without consent, default consent, or missing consent records, and VND 50–70 million for treating silence as consent or ignoring a withdrawal. The fines for missing dossiers weren't found.

## Internet services and social networks: Decree 147/2024

[Decree 147/2024/ND-CP](https://datafiles.chinhphu.vn/cpp/files/vbpq/2024/11/147-nd.signed.pdf) (official signed scan, Government portal) was issued 2024-11-09 and is **in force since 2024-12-25** (Art. 83). It replaced Decree 72/2013. The quotes below are from that scan, read with OCR. The only later change found is a correction to an appendix form ([Công văn 48/CP-KGVX, 2025-01-24](https://thuvienphapluat.vn/cong-van/Cong-nghe-thong-tin/Cong-van-48-CP-KGVX-2025-dinh-chinh-Nghi-dinh-147-2024-ND-CP-643256.aspx)).

### Is Issho a social network?

Almost certainly. Art. 3(25): *"Mạng xã hội (social network) là hệ thống thông tin được thiết lập trên nền tảng trang thông tin điện tử hoặc ứng dụng trên mạng, cung cấp các dịch vụ, công cụ cho phép người sử dụng dịch vụ cung cấp, trao đổi, tương tác, chia sẻ thông tin với nhau."* (A social network is an information system on a website or an app that provides services and tools for users to provide, exchange, interact and share information with each other.)

There is no exception for small or purpose-built apps. Issho's event and crew chats are exactly this. Its "crews" also resemble Art. 3(29)'s *nhóm cộng đồng* (group).

### Two regimes, decided by who runs it (Art. 24(6)(a))

- **Foreign social network:** *"do tổ chức, doanh nghiệp, cá nhân nước ngoài cung cấp xuyên biên giới vào Việt Nam"* (provided cross-border into Vietnam by a foreign organisation, enterprise or individual). Governed by Art. 23. "Cross-border provision" means an organisation or individual **abroad** serving users in Vietnam (Art. 3(33)).
- **Domestic social network:** *"do cơ quan, tổ chức, doanh nghiệp có tư cách pháp nhân tại Việt Nam cung cấp"* (provided by a body with legal personality in Vietnam).
  - A **high-traffic** network has ≥ **10,000 visits a month** (6-month average) or more than **1,000 regular users a month**. It needs a **licence**.
  - Below that it is **low-traffic** and needs a **notification certificate** (Art. 24(6)(b)).

A **domestic** provider must:

- be a Vietnamese legal entity whose registered business lines cover social networks (Art. 25(4)(a));
- use a **`.vn` main domain** and store user data on servers with **Vietnamese IP addresses** (Art. 26(2)), with **at least one server in Vietnam** (Art. 27(2)(e));
- have content staff, with the person responsible for content a **Vietnamese national** (Art. 27(1)(a));
- keep account, login, logout and IP logs for **2 years** (Art. 27(2)(b));
- store name, date of birth and a Vietnamese mobile number (or personal ID number), and **verify accounts by Vietnamese mobile number** (Art. 27(3)(a)–(b));
- remove illegal content within 24 hours of a request (Art. 24(4));
- display its licence details (Art. 24(9));
- report every year by 25 November (Art. 24(10)).

An individual can't be a domestic provider at all.

### Foreign social networks: the 100,000-visit threshold (Art. 23(3))

The duties apply only to foreign providers that *"có sử dụng dịch vụ cho thuê chỗ lưu trữ dữ liệu tại Việt Nam hoặc có tổng số lượt truy cập từ Việt Nam (total visits) thường xuyên trong 01 tháng (số liệu thống kê trung bình trong thời gian 06 tháng liên tục) từ 100.000 (một trăm nghìn) lượt trở lên"*: those that lease data storage in Vietnam, or average **100,000 or more visits a month from Vietnam over 6 consecutive months**. Those providers must:

- **(a)** notify their contact details to the authority (Form 10) within **60 days** of crossing the threshold;
- **(b), (c)** monitor content and remove illegal content within **24 hours** of a request (Art. 23(5));
- **(đ)** store users' name, date of birth and a Vietnamese mobile number (or personal ID number); under-16s register with a parent's details;
- **(e)** *"Thực hiện xác thực tài khoản của người sử dụng dịch vụ mạng xã hội bằng số điện thoại di động tại Việt Nam"* (verify social network accounts with a Vietnamese mobile number). A personal ID number is allowed only when the user confirms they have no Vietnamese number. *"Bảo đảm chỉ những tài khoản đã xác thực mới được đăng tải thông tin (viết bài, bình luận, livestream)"* (only verified accounts may post, comment or livestream);
- **(h), (i)** run a complaints tool and act within 48 hours on complaints about content banned by the Cybersecurity Law;
- **(l)** publish how content is distributed;
- **(q)** report every year by **25 November** (Form 09).

Point (o) also says only foreign providers that have notified may offer livestreaming *"hoặc cung cấp các hoạt động có phát sinh doanh thu dưới mọi hình thức"* (or any revenue-earning activity). It sits inside clause 3, which applies only above the threshold, so whether a small paid service must notify is unclear.

Below the threshold, Art. 23(1) still requires compliance with Vietnamese law in general, and Art. 23(2) lets the authorities block a foreign provider that breaks the law or won't cooperate on illegal content.

**Dates:** Art. 82(10) gave covered providers **90 days from 2024-12-25** (to 2025-03-25) to verify existing active accounts. Art. 82(11) makes non-compliance after that punishable. No separate grace period is given for verification by a provider that crosses the threshold later.

**Who enforces it now:** the decree names the Ministry of Information and Communications' Authority of Broadcasting and Electronic Information (ABEI). Since 2025-03-01 ABEI sits under the **Ministry of Culture, Sports and Tourism** ([Decree 43/2025](https://thuvienphapluat.vn/phap-luat-nha-dat/nghi-dinh-432025ndcp-ve-co-cau-to-chuc-bo-van-hoa-the-thao-va-du-lich-1010.html)). The decree's filing address (`report_abei@mic.gov.vn`) may have changed. Not verified.

### What this means for Issho

- **If Issho is run from outside Vietnam** (owner or company abroad, servers abroad, which fits today's architecture), it is a foreign social network far below 100,000 visits a month. **No phone verification, notification or annual report is due.** Cost: nothing now. Two things to do:
  - Keep the ability to act on a takedown request within 24 hours. Reporting, suspensions and the admin SPA already do most of this.
  - **Watch Vietnamese traffic.** The threshold counts visits, not users. Add a monthly count of Vietnam sessions to the metrics, and plan phone OTP (Supabase Auth supports SMS providers) long before 100,000.
- **If Issho is operated from inside Vietnam,** it needs a Vietnamese company, a notification certificate, a `.vn` domain, servers in Vietnam, a Vietnamese content lead and phone verification before launch. That is a large project and would be the expensive path.
- **If the owner runs it as an individual living in Vietnam,** the decree offers no legal home: a domestic network must be a legal entity, and the foreign regime is meant for providers abroad.

The owner's question is which of these fits. The decree doesn't say how a foreign individual living in Vietnam who runs a service hosted abroad is classed. **Not verified; worth a local lawyer's view before the public launch.**

## Cybersecurity Law 2025 (in force 2026-07-01)

Law 116/2025/QH15 replaced the 2018 Cybersecurity Law and the 2015 Network Information Security Law (Art. 44). Text: [hethongphapluat.com, Art. 25](https://hethongphapluat.com/luat-an-ninh-mang-2025/dieu-25), [Art. 41](https://hethongphapluat.com/luat-an-ninh-mang-2025/dieu-41). This is a secondary copy of the text; the official gazette wasn't reached.

- **Art. 25(2)**, for domestic and foreign telecom, internet and value-added service providers "in Vietnam":
  - verify information when users register digital accounts;
  - give user information to the Ministry of Public Security's cybersecurity force within 24 hours of a request (3 hours if urgent);
  - remove violating content within 24 hours (6 hours if urgent);
  - keep logs, and keep user data after an account closes, for the period set by law.
- **Art. 25(3):** domestic and foreign providers that collect or process Vietnamese users' personal data, relationship data or user-created data must **store it in Vietnam** for a period set by the Government. Foreign providers must also open a **branch or representative office**. Art. 25(4) leaves the details to a Government decree.
- **Under the 2018 law** ([Decree 53/2022, Art. 26(3), 26(6), 27](https://vcci.com.vn/legal-document/nghi-dinh-532022nd-cp-huong-dan-luat-an-ninh-mang)), the same duty applied to a foreign enterprise only after:
  - its service had been used to break the law;
  - it had failed to comply with a written request from the ministry;
  - the Minister of Public Security had issued a decision.

  The enterprise then had **12 months** to localise, and had to keep data for at least 24 months.

**For Issho:** under the 2018 model, nothing applies unless the ministry orders it. Whether the 2026 decree keeps that trigger wasn't found (one source names a Decree 333/2026, not read). If it makes localisation automatic, it would override everything above and need servers in Vietnam. **Check this before the public launch.** The duty to keep data after an account closes may also conflict with Issho's cascade deletion once the period is defined.

## Cheapest compliant path for the soft launch

1. **Decide and write down the operator** (a person abroad, a foreign company, or a Vietnamese company). Running Issho from outside Vietnam keeps Decree 147 to "obey the law and handle takedown requests".
2. **Before the first user:**
   - a privacy policy in Vietnamese and English (data, purposes, recipients abroad, rights, contact, possibly sensitive items);
   - consent per purpose, unticked by default, stored with the policy version;
   - a requests contact;
   - an opt-in before any analytics.

   The same policy serves Apple and Google ([store paths note](https://github.com/shrujancheruku/issho/blob/research/store-paths-vietnam/docs/research/store-paths-vietnam.md)).
3. **Within 60 days of the first user in Vietnam:** file the **cross-border transfer dossier** (Form 09 + 01a/01b) and the **processing impact assessment dossier** (Form 10 + 02a/02b) with the Ministry of Public Security. Update them every 6 months if anything changes.
4. **A breach runbook:** notify within 72 hours.
5. **Watch:** Vietnamese monthly visits (100,000 means phone verification, notification and annual reports); the decree under the 2026 Cybersecurity Law (localisation); and PDPL Art. 29(2) before any ID-document verification.

## Open questions for the owner

- **Who is the operator, and where is it based?** A person abroad, a foreign company, or a Vietnamese company. This decides which Decree 147 regime applies, and whether the small-business relief is available.
- **Do the dossiers get filed in the owner's own name, or a company's?** Both need a named controller and a signature.
- **Should the privacy policy treat attendance, feedback and chat as possibly sensitive?** That is the conservative reading of Decree 356 Art. 4(1)(c) and (l).
- **Should a local lawyer review this before the public launch?** The ambiguities are the classification of an individual operator and the 2026 localisation decree.

## Not verified here

- **Decree 356 forms:** the Form 09, 10, 01a/b and 02a/b templates weren't read, nor the filing portal or address, nor whether filing has a fee.
- **Decree 330/2026:** the fines for missing dossiers. A Decree 363/2026 on penalties "in the data field" also appears on thuvienphapluat.vn; how the two relate wasn't checked.
- **Who enforces Decree 147 now:** whether ABEI's filing addresses changed after the 2025 move to the Ministry of Culture, Sports and Tourism. A Decree 298/2026 reportedly replaces Decree 43/2025.
- **The 2026 Cybersecurity Law decree:** whether one has been issued under Law 116/2025, and whether localisation still needs a ministry order. Decree 53/2022's status after 2026-07-01 is also unknown.
- **Counting visits:** how "total visits" and "regular users" are counted for an app, rather than a website.
- **Language:** whether the consent and the privacy notice must be in Vietnamese. It's assumed sensible, not found as a rule.
- **Sources:** the English texts of Law 91 and Decree 356 are unofficial translations (LuatVietnam), and thuvienphapluat.vn refused automated access. The Decree 147 quotes come from OCR of the signed scan, so check the wording against the original before quoting it elsewhere.
