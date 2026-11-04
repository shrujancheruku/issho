# Research: store paths to real users in Vietnam

Issue [#336](https://github.com/shrujancheruku/issho/issues/336), part of map [#334](https://github.com/shrujancheruku/issho/issues/334). Researched 2026-11-04 against main at `cdd353a`. This branch is never merged; anything adopted moves into `docs/ROADMAP.md` (Launch gates) and the READMEs.

Starting point: an Expo app built with EAS, bundle id `com.joinissho.app`. The owner has a **personal** Google Play developer account (identity verification pending) and has not joined the Apple Developer Program. Sign-in is email OTP only. The app has profiles, photos shown to attendees, group chat, reporting (`ReportPerson`, `ReportCrewMember`), system flags, suspensions and in-app account deletion (`DeleteAccount`).

Vietnamese law for a social app (Decree 147 and so on) is ticket [#335](https://github.com/shrujancheruku/issho/issues/335). This note covers only what the **stores** require.

## Answer in brief

- **Android: a waitlist soft launch can run entirely on Play's closed testing track, and the soft-launch users are the 12 testers Google requires.** New personal accounts (created after 2023-11-13) can't use production or open testing until **at least 12 testers have been opted in to a closed test for 14 consecutive days** and Google approves a production-access application (usually 7 days or less). Closed tests take email lists or Google Groups (200 lists of up to 2,000 people each), so waitlist people can install from the Play Store straight away. Production can come later.
- **iOS: the shortest path is TestFlight external testing with a public link** (up to 10,000 testers, builds last 90 days). It needs the paid Apple Developer Program (99 USD a year; an individual needs no D-U-N-S number) and a TestFlight App Review of the first build. TestFlight apps must follow the full App Review Guidelines, so it's no shortcut around the rules below. An App Store release limited to Vietnam is the alternative, and takes the same work plus full App Review.
- **Both stores require users to be able to block other users. Issho has reporting but no blocking**, so this is a gate on both platforms. Also missing or unconfirmed: accepting terms before posting (Play), filtering of objectionable material (Apple), published contact information (Apple), and a **web page for requesting account deletion** (Play). In-app deletion already meets both stores' rules.
- **Both reviews need a way to sign in without a real inbox.** Email OTP means the reviewers need a demo account whose code they can get (for example a fixed code for one review address), or a demo mode Apple approves in advance.
- **Vietnam itself is open on both stores.** Both list Vietnam as a country. The Vietnam-specific store rules (game licences) apply only to games. Apple shows a Vietnam-only age rating (Decree 147, Article 38): user content and chat alone give **00+**, but declaring the "Social media" capability gives **16+**.
- **Android build:** from 2026-08-31, new apps and updates on Play must target **API level 36** (Android 16).

## Google Play

### The testing requirement for new personal accounts

From [App testing requirements for new personal developer accounts](https://support.google.com/googleplay/android-developer/answer/14151465):

- It applies to "personal developer accounts created after November 13, 2023". The owner's account (created 2026-10-26) is one of them.
- You must "run a closed test" before applying for production. "At least 12 testers must be opted in to your closed test when you apply," and each must have been "opted in continuously for at least 14 days". If a tester opts out and back in, the 14 days have to be consecutive.
- **Production and pre-registration stay disabled until then, and "open testing becomes available after you gain production access."** Internal testing is a separate track. The page doesn't count internal testers toward the 12.
- You apply from the Dashboard ("Apply for production"). The form asks how easily you recruited testers, how engaged they were, what feedback came in and how you gathered it, your audience and value, a first-year install estimate, and what you changed after testing. Review "usually takes 7 days or less". Google may ask for more testing if there were too few testers or too little engagement. Working test credentials are required if the app needs a login.
- The page sets no device or country conditions on testers.

So the earliest production date is the day closed testing starts plus 14 days plus Google's review, and only after identity verification has cleared.

### Tracks

From [Set up an open, closed or internal test](https://support.google.com/googleplay/android-developer/answer/9845334):

| Track | Who | Limits | Notes |
|---|---|---|---|
| Internal | Named emails | 100 testers per app | Available "within minutes"; "might not be subject to standard Play policy or security reviews"; testers can be anywhere |
| Closed | Email lists, **Google Groups** (closed only) | 200 lists, up to 2,000 people each; up to 50 lists per track | Can be targeted at countries (extra closed tracks can't) |
| Open | Anyone, listed on Play | Unlimited, or a cap of at least 1,000 | Locked until production access (above) |
| Production | Everyone in chosen countries | – | Locked until production access |

Country settings "apply across all tracks", and targeting follows the country of the user's Google Play account, not where they are ([answer 7550024](https://support.google.com/googleplay/android-developer/answer/7550024)).

**What this means for Issho:** the soft launch's waitlist fits the closed track. Add accepted people to a Google Group used by the closed test, and they opt in and install from the Play Store. Their first 14 days are also Google's testing requirement. The ROADMAP's plan to use internal testing for the owner's own devices still works, but internal testers don't open the way to production.

### Policies that apply to Issho

**User generated content** ([policy](https://support.google.com/googleplay/android-developer/answer/9876937)). An app with UGC must:

1. require users to "accept the app's terms of use and/or user policy before users can create or upload UGC";
2. define objectionable content and behaviour, and ban it in those terms;
3. moderate "as is reasonable and consistent with the type of UGC hosted by the app", including "an in-app system for reporting and blocking objectionable UGC and users";
4. for "UGC features that enable 1:1 user interaction with specific users", offer "in-app functionality for blocking users";
5. stop in-app monetisation from rewarding objectionable behaviour (Issho has none).

Reporting and moderation exist. **Blocking does not exist.** A terms screen before the profile is created is unconfirmed: a quick search of the repo found no terms or privacy-policy screen.

**Account deletion** ([requirements](https://support.google.com/googleplay/android-developer/answer/13327111)). Apps that create accounts need "an in-app path to delete their app accounts and associated data" (Issho has one) **and a web resource** where users can ask for deletion without reinstalling the app. The link goes in the Data safety form, must work, must name the app or developer, and must make the request easy to find. Data kept for legal or security reasons must be disclosed, for example in the privacy policy.

**Data safety form** ([help](https://support.google.com/googleplay/android-developer/answer/10787469)): required for every app, on the App content page, and it carries the deletion link above.

**Target API level** ([requirements](https://developer.android.com/google/play/requirements/target-sdk)): "Starting August 31 2026, new apps and app updates must target Android 16 (API level 36) or higher". An extension to 2026-11-01 was available on request, and that date has passed. Check that Expo SDK 58's default `targetSdkVersion` is 36 or higher before the first upload.

### Vietnam on Google Play

- Vietnam is a supported location for developer registration and for distribution ([answer 9306917](https://support.google.com/googleplay/android-developer/answer/9306917), [country availability](https://support.google.com/googleplay/answer/2843119)).
- The Vietnam entry on [Requirements for distributing apps in specific countries](https://support.google.com/googleplay/android-developer/answer/6223646) is about **online games** only (licence under Decree 147/2024/ND-CP from 2024-12-25). It doesn't mention social apps, and the page says it's "not an exhaustive list of legal requirements". Social-app law is #335.
- Android's new **developer verification** for sideloaded apps starts on 2026-09-30 in Brazil, Indonesia, Singapore and Thailand only, and goes global in 2027 ([developer verification](https://developer.android.com/developer-verification)). Vietnam isn't affected in 2026, and Play apps are registered automatically.

## Apple

### Getting an account

From [Apple Developer Program enrollment](https://developer.apple.com/programs/enroll/) and [D-U-N-S Number](https://developer.apple.com/help/account/membership/D-U-N-S/):

- 99 USD per membership year (local prices may differ).
- **Individual:** an Apple Account with two-factor authentication, and you must be of legal age. No D-U-N-S number. The seller shown on the App Store is the person's legal name.
- **Organization:** needs a legal entity (no trade names or DBAs) and a D-U-N-S number (free from Dun & Bradstreet, then up to 2 business days for Apple to receive it).
- The app record, TestFlight, the App Store and the APNs key for iOS push all wait on this membership.

### TestFlight (shortest path)

From the [TestFlight overview](https://developer.apple.com/help/app-store-connect/test-a-beta-version/testflight-overview), [Invite external testers](https://developer.apple.com/help/app-store-connect/test-a-beta-version/invite-external-testers) and [TestFlight](https://developer.apple.com/testflight/):

- **Internal testers:** up to 100 App Store Connect users on the team. No review. Not useful for the public.
- **External testers:** up to **10,000** per app, invited by email or by a **public link** ("Open to Anyone" or filtered by device and OS, with a tester cap between 1 and 10,000). "Anyone can share this link", and testers who join by link show up as anonymous.
- **Review:** builds added to an external group are sent to TestFlight App Review. "The first build you submit requires a full review, but later builds for the same version might not." One build per version can be in review at a time, and at most six submissions in 24 hours.
- **Required information:** a beta app description, a feedback email, contact information and beta review information (sign-in details) before external testing can start.
- **Builds expire after 90 days.** A soft launch longer than that needs a new build, plus possibly a new review, every 90 days.
- Guideline 2.2: a TestFlight build "should be intended for public distribution and should comply with the App Review Guidelines", and testers can't be paid or rewarded in any way. The UGC and deletion rules below therefore apply to TestFlight as well.
- Apple's pages don't mention any country restrictions on TestFlight testers.

EAS Submit can upload to App Store Connect, after which the build is added to an external group.

### App Review rules for a social app

From the [App Review Guidelines](https://developer.apple.com/app-store/review/guidelines/):

**1.2 User-Generated Content.** Apps where users contribute content "must include":

- "A method for filtering objectionable material from being posted to the app". Issho's system flags cover gender-leaning picks, not content. Nothing filters names, bios, photos or chat messages before they're posted.
- "A mechanism to report offensive content and timely responses to concerns". Issho has `ReportPerson` and `ReportCrewMember`, plus the admin queue. "Timely" depends on the owner, who moderates alone (map #334).
- "The ability to block abusive users from the service". **Missing.** Suspension is a moderator action and doesn't count as users blocking each other.
- "Published contact information so users can easily reach you". Needs a visible support contact in the app and the listing.

**5.1.1(v) account deletion** ([guideline](https://developer.apple.com/app-store/review/guidelines/#data-collection-and-storage), [Offering account deletion in your app](https://developer.apple.com/support/offering-account-deletion-in-your-app/)): apps that create accounts must let people "initiate deletion of their account within the app", easy to find (typically account settings), deleting "the entire account record, along with associated personal data", and deactivation alone "is insufficient". Apps outside regulated industries must not make people phone, email or go through support. Issho's Profile-tab deletion, which cascades to all app data, meets this. Sign in with Apple token revocation doesn't apply, since Issho doesn't use it, and guideline 4.8 (login services) applies only to apps using third-party sign-in.

**2.1 App completeness:** "include demo account info (and turn on your back-end service!) if your app includes a login". Apple must approve a demo mode in advance if no account can be given. With email OTP, the reviewer needs an account they can actually sign in to. The same goes for Play's production application.

### Vietnam on the App Store

- Vietnam is an App Store country ([pricing and availability start times](https://developer.apple.com/help/app-store-connect/reference/app-store-pricing-and-availability-start-times-by-country-or-region/)). Availability can be limited to Vietnam in App Store Connect.
- **Games** need a Vietnamese licence shown on the Vietnam product page ([Game distribution on the App Store in Vietnam](https://developer.apple.com/news/?id=06h4gf33)). Issho isn't a game.
- **Vietnam age ratings** since 2026-06-18, under Article 38 of Decree 147 ([announcement](https://developer.apple.com/news/?id=yrrb45pw), [age rating values](https://developer.apple.com/help/app-store-connect/reference/app-information/age-ratings-values-and-definitions)): four ratings worked out from the questionnaire. "User-generated content" and "Messaging and chat" alone give **00+**. "Social media" (a feed that redistributes or amplifies UGC to many users) gives **16+**. Issho has no feed, so 00+ or a content-driven rating is likely. Answer the questionnaire carefully.

## Shortest path, step by step

**Android (no money needed):**

1. Google clears identity verification.
2. Create the app `com.joinissho.app`. Fill in App content: privacy policy URL, Data safety (with the deletion web link), content rating, target audience, ads.
3. Build with target API 36+, then upload the first AAB by hand to a **closed** track available in Vietnam, linked to a Google Group.
4. Add soft-launch people (at least 12) to the Group. They install from Play. Day 0 of the 14 days.
5. On day 14 or later, apply for production with a reviewer login. About 7 days later, production can open in Vietnam (or stay closed while the soft launch runs).

**iOS:**

1. Enroll in the Apple Developer Program as an individual (99 USD). Create the App ID and the App Store Connect record. Create the APNs key.
2. Build and submit with EAS. Create an internal group, then an external group with a public link capped at the waitlist size.
3. Fill in the beta description, feedback email, contact and **reviewer sign-in**. The first build goes to TestFlight App Review.
4. Share the link with accepted waitlist people. Rebuild at least every 90 days.

**Gates both paths share (app work):** user blocking, terms accepted before posting a profile or chatting, a support contact shown in the app, a privacy policy, a web page for requesting deletion, a reviewer sign-in for OTP, and (Apple) a pre-posting filter for objectionable text and photos, or an argument for why moderation after posting is enough.

## Open questions for the owner

- Run the soft launch on **TestFlight and closed testing**, or go for **production and the App Store limited to Vietnam**? The first is faster to start and fits a waitlist. The second needs full App Review and Play's production access anyway.
- Enroll with Apple as an **individual** (your legal name shown as the seller) or set up an **organization** first (D-U-N-S, a legal entity)? Play shows the developer name too.
- How should **blocking** behave in plans and groups? For example, never match the two people again and hide each other's profiles and messages.
- How much **filtering before posting** is enough for Apple 1.2? A word list on names, bios and chat, image moderation for photos, or both?
- How do reviewers sign in with **email OTP**? A fixed code for one review address, kept out of the public repo?

## Not verified here

- Whether closed-track releases go through Play's standard app review. The help page says only internal tests "might not be" reviewed.
- Expo SDK 58's default `targetSdkVersion`.
- Whether Issho already shows terms of use or a privacy policy. A quick repo search found none.
- Current review times. Apple publishes none for TestFlight, and Google says "7 days or less" for production access.
