# Changelog

## [0.11.0](https://github.com/shrujancheruku/issho/compare/v0.10.0...v0.11.0) (2026-11-07)


### ⚠ BREAKING CHANGES

* **db:** one rooms table and one messages table for outing and crew chat ([#513](https://github.com/shrujancheruku/issho/issues/513))

### Bug Fixes

* **deps:** update dependency [@vis](https://github.com/vis).gl/react-google-maps to v1.10.2 ([#365](https://github.com/shrujancheruku/issho/issues/365)) ([4ab9708](https://github.com/shrujancheruku/issho/commit/4ab970859df895f19d5ce77edaf53f300fcd466f))
* **deps:** update dependency @connectrpc/connect to v2.3.0 ([#376](https://github.com/shrujancheruku/issho/issues/376)) ([36ac568](https://github.com/shrujancheruku/issho/commit/36ac56828dd2fbf985e819cdf1b6ff05ec829c06))
* **deps:** update dependency @connectrpc/connect-web to v2.3.0 ([#377](https://github.com/shrujancheruku/issho/issues/377)) ([dde7c56](https://github.com/shrujancheruku/issho/commit/dde7c56aca5377ee9bf3ad2631d5e8eb5c470aff))
* **deps:** update dependency @sentry/react-native to ~8.33.0 ([#378](https://github.com/shrujancheruku/issho/issues/378)) ([77c6d00](https://github.com/shrujancheruku/issho/commit/77c6d002dd697e31f8c970b3173a3b594b42d38f))
* **deps:** update dependency @tanstack/react-query to v5.106.1 ([#379](https://github.com/shrujancheruku/issho/issues/379)) ([c824635](https://github.com/shrujancheruku/issho/commit/c824635fc07cffe0bf71322ddb8b799c2a7ff83c))
* **deps:** update dependency react-native-gesture-handler to ~3.4.0 ([#380](https://github.com/shrujancheruku/issho/issues/380)) ([e47f454](https://github.com/shrujancheruku/issho/commit/e47f4548775fcf04a667a28992a471e0d78c83b2))
* **deps:** update dependency react-native-safe-area-context to ~5.11.0 ([#381](https://github.com/shrujancheruku/issho/issues/381)) ([eed2328](https://github.com/shrujancheruku/issho/commit/eed23284f8476acabf433069e417a597ac87a219))
* **deps:** update dependency react-native-screens to v4.30.1 ([#366](https://github.com/shrujancheruku/issho/issues/366)) ([6ba69f9](https://github.com/shrujancheruku/issho/commit/6ba69f9dfee5846b4bc494bbffa3e68619358b69))
* **deps:** update dependency react-native-worklets to v0.13.1 ([#367](https://github.com/shrujancheruku/issho/issues/367)) ([4e1ddf3](https://github.com/shrujancheruku/issho/commit/4e1ddf340714d5fa0011a985f9a609c78e461e66))
* **deps:** update expo ([#368](https://github.com/shrujancheruku/issho/issues/368)) ([efa799c](https://github.com/shrujancheruku/issho/commit/efa799c7fdc4414818334109a405f1553e781bb1))
* **deps:** update module connectrpc.com/connect to v2 ([#471](https://github.com/shrujancheruku/issho/issues/471)) ([0363829](https://github.com/shrujancheruku/issho/commit/0363829cdd529cefb9721e3a9b9cc13752634e9f))
* **deps:** update module github.com/jackc/pgx/v5 to v5.12.0 ([#382](https://github.com/shrujancheruku/issho/issues/382)) ([bbe10f2](https://github.com/shrujancheruku/issho/commit/bbe10f2b15b7966f5b81f9f6ec7061a6465c4bc2))
* **deps:** update module github.com/micahparks/keyfunc/v3 to v3.9.1 ([#383](https://github.com/shrujancheruku/issho/issues/383)) ([23730e0](https://github.com/shrujancheruku/issho/commit/23730e042514d93e7fdc8250053a20bc2ab56e8b))
* **deps:** update react monorepo to v19.3.1 ([#369](https://github.com/shrujancheruku/issho/issues/369)) ([dd8dbc7](https://github.com/shrujancheruku/issho/commit/dd8dbc7a5a1488c001588414ab7f76a166f93657))
* **deps:** update river monorepo to v0.51.0 ([#384](https://github.com/shrujancheruku/issho/issues/384)) ([7d096ce](https://github.com/shrujancheruku/issho/commit/7d096ce8ff0c7e4ff2fdf189de128ce49329167d))


### Code Refactoring

* **db:** one rooms table and one messages table for outing and crew chat ([#513](https://github.com/shrujancheruku/issho/issues/513)) ([acedc11](https://github.com/shrujancheruku/issho/commit/acedc1110f63975e608d55db8f602f9d1099f5c9))

## [0.10.0](https://github.com/shrujancheruku/issho/compare/v0.9.0...v0.10.0) (2026-11-04)


### ⚠ BREAKING CHANGES

* **data:** tokyo's areas become its municipalities ([#272](https://github.com/shrujancheruku/issho/issues/272))
* **api:** typed enums instead of strings in the admin protos ([#264](https://github.com/shrujancheruku/issho/issues/264))
* **api:** page the admin lists with keyset tokens ([#233](https://github.com/shrujancheruku/issho/issues/233))
* **backend:** a city pack is authoritative for its areas and overrides ([#235](https://github.com/shrujancheruku/issho/issues/235))
* **db:** one baseline migration with the cleaned schema; reset issho-dev ([#224](https://github.com/shrujancheruku/issho/issues/224))
* one chat module for outing and crew rooms; one socket per room in the app ([#138](https://github.com/shrujancheruku/issho/issues/138))
* ModerationService no longer has GetModerator, ListTranslationLanguages, ListTranslations or ReviewTranslation. Only the admin SPA calls ModerationService, and it uses none of them.

### Features

* **admin:** confirm irreversible actions; tests for every flow ([#218](https://github.com/shrujancheruku/issho/issues/218)) ([1c5a008](https://github.com/shrujancheruku/issho/commit/1c5a0081de7908eda243c22aa83a8720cfe851b8))
* **admin:** show what an authoritative city-pack apply removed ([#266](https://github.com/shrujancheruku/issho/issues/266)) ([36f4f31](https://github.com/shrujancheruku/issho/commit/36f4f319d142697842edaa0f2292fe6843d46ea5))
* **api:** a reason for unavailable suggestions ([#270](https://github.com/shrujancheruku/issho/issues/270)) ([5e351fe](https://github.com/shrujancheruku/issho/commit/5e351fefa745eedc0165614d5ef4f1aff03a9353))
* **api:** bound and cap the app's unpaged lists ([#265](https://github.com/shrujancheruku/issho/issues/265)) ([c3ce3f8](https://github.com/shrujancheruku/issho/commit/c3ce3f8c5cddc648a4edf9bf417108a738393f13))
* **api:** error reasons as a Connect detail; one error shape in errs ([#219](https://github.com/shrujancheruku/issho/issues/219)) ([40a6b87](https://github.com/shrujancheruku/issho/commit/40a6b87b2e28abf2dcffa8551b3667b6173ac8ee))
* **api:** page the admin lists with keyset tokens ([#233](https://github.com/shrujancheruku/issho/issues/233)) ([14be51d](https://github.com/shrujancheruku/issho/commit/14be51dbc2f7e3045ca212cffd0f1a19cbb640b8))
* **api:** protovalidate annotations as the single source of field limits ([#262](https://github.com/shrujancheruku/issho/issues/262)) ([02ff837](https://github.com/shrujancheruku/issho/commit/02ff837ca7dd311d7ffb17248e260dea84eeea67))
* **api:** request size limits, server timeouts and an RPC deadline ([#227](https://github.com/shrujancheruku/issho/issues/227)) ([350a15e](https://github.com/shrujancheruku/issho/commit/350a15edfc5ac96e7f2354b04352c96d1b57536b))
* **backend:** a city pack is authoritative for its areas and overrides ([#235](https://github.com/shrujancheruku/issho/issues/235)) ([b4b4353](https://github.com/shrujancheruku/issho/commit/b4b43531679ab3d3139b5184389aec85898b4de6))
* **backend:** admin apply and venue discovery as River jobs, GetAdminJob, 60 s interactive Gemini ([#281](https://github.com/shrujancheruku/issho/issues/281)) ([9d38aff](https://github.com/shrujancheruku/issho/commit/9d38affa900d23f88392cf6b2d1e599bef3ba2ef))
* **backend:** fail fast in production with an app env, and no missing key deletes data ([#232](https://github.com/shrujancheruku/issho/issues/232)) ([490d974](https://github.com/shrujancheruku/issho/commit/490d9744856a5b561e6579ca1e99b51006930d7c))
* **backend:** one httpjson client policy for Google, GitHub, Expo and Supabase ([#280](https://github.com/shrujancheruku/issho/issues/280)) ([424f0c8](https://github.com/shrujancheruku/issho/commit/424f0c8073a9db6e09a4ab3ca8a671b326d7882b))
* **backend:** one pending-change module (repoedit) for the editors; rolling city-pack PRs ([#303](https://github.com/shrujancheruku/issho/issues/303)) ([c436dfe](https://github.com/shrujancheruku/issho/commit/c436dfe08242442c506d311e9909fc9f08f7eaa5))
* **backend:** per-city matching config, timing and meeting places in the city pack ([#287](https://github.com/shrujancheruku/issho/issues/287)) ([93299c9](https://github.com/shrujancheruku/issho/commit/93299c9e52e1e54c5cbb7224e572924c7875e7a5))
* **backend:** worker queues, per-kind timeouts and attempts, walk budget from the deadline ([#275](https://github.com/shrujancheruku/issho/issues/275)) ([e230bdb](https://github.com/shrujancheruku/issho/commit/e230bdbc876a41c862226ec4ae05059f33ac9ca2))
* **data:** add Da Nang and Hoi An as one city pack, with ward boundaries ([#130](https://github.com/shrujancheruku/issho/issues/130)) ([2cd110a](https://github.com/shrujancheruku/issho/commit/2cd110aaa7c998f64b16936181f7fe4fae9d2c32))
* **data:** tokyo's areas become its municipalities ([#272](https://github.com/shrujancheruku/issho/issues/272)) ([8a74180](https://github.com/shrujancheruku/issho/commit/8a741807de27f48ca3878f8d2d4b465acc20d1d1))
* **db:** retention for notifications and crew chat ([#230](https://github.com/shrujancheruku/issho/issues/230)) ([3c9801b](https://github.com/shrujancheruku/issho/commit/3c9801b1d759f02a67e9d585a70fe5ee7d2cf135))
* **infra:** seat the site's invite round a table that fills as people say yes ([#118](https://github.com/shrujancheruku/issho/issues/118)) ([5285533](https://github.com/shrujancheruku/issho/commit/5285533a7a8e78ee0d6433aacc1e16b68587e8e8))
* **infra:** sentry cron monitor alert for failed or missed backups ([#222](https://github.com/shrujancheruku/issho/issues/222)) ([96295ba](https://github.com/shrujancheruku/issho/commit/96295ba8ab56733306164a770ac587b89847c8dc))
* **infra:** show the site's app panels in the README, animated ([#128](https://github.com/shrujancheruku/issho/issues/128)) ([f1899aa](https://github.com/shrujancheruku/issho/commit/f1899aa6f9cfd3dda370204f2ced3911b158ba3d))
* **infra:** site map, live chat and crew offer; deploy the site from CI ([#113](https://github.com/shrujancheruku/issho/issues/113)) ([b75e9d1](https://github.com/shrujancheruku/issho/commit/b75e9d15547c66786fd0197f72005e87b7fc23b5))
* **mobile:** launch screen, round logos and favicons, site light and dark ([#112](https://github.com/shrujancheruku/issho/issues/112)) ([0ff5b1e](https://github.com/shrujancheruku/issho/commit/0ff5b1e425a8b6c615ccc56cd7c4742075c00628))
* **mobile:** one errorText helper and ErrorLine; reasons on screens; sign-in error keys ([#234](https://github.com/shrujancheruku/issho/issues/234)) ([637e3cb](https://github.com/shrujancheruku/issho/commit/637e3cb68d790b674d11b10dab1e88c70c019e98))
* **mobile:** redesign the app as Together, Plans and You ([#109](https://github.com/shrujancheruku/issho/issues/109)) ([acd3352](https://github.com/shrujancheruku/issho/commit/acd33526f1cb8fa9bf3c5dd2100a5e083a0556bc))
* **mobile:** refetch on foreground and on received pushes ([#274](https://github.com/shrujancheruku/issho/issues/274)) ([b745ea0](https://github.com/shrujancheruku/issho/commit/b745ea0aa147d30abe3c17da5dfb266163449c21))


### Bug Fixes

* **admin:** a hash matching an Object prototype key no longer crashes the router ([#301](https://github.com/shrujancheruku/issho/issues/301)) ([7da69df](https://github.com/shrujancheruku/issho/commit/7da69dfd37cb76765e5210d3ee632dda920da280))
* **admin:** stop inlining font subsets the csp blocks ([#313](https://github.com/shrujancheruku/issho/issues/313)) ([8b76f2d](https://github.com/shrujancheruku/issho/commit/8b76f2ddc54bd679def244ab87ae2e90b0271d76))
* **backend:** close withdraw and chat-send races, check suspension per send ([#276](https://github.com/shrujancheruku/issho/issues/276)) ([faf3184](https://github.com/shrujancheruku/issho/commit/faf31849042ff380df872e4d689c535e4b8a2165))
* **backend:** one item never stops a pass or a push batch ([#223](https://github.com/shrujancheruku/issho/issues/223)) ([dcdde08](https://github.com/shrujancheruku/issho/commit/dcdde08f9cf0e2b69da1586f6d6d25c4d9ad042a))
* **backend:** venue opening window from the activity's hours ([#282](https://github.com/shrujancheruku/issho/issues/282)) ([58c2b37](https://github.com/shrujancheruku/issho/commit/58c2b372ff336c9a2c17c70f2de3179d73b32709))
* **infra:** centre the text on the site's activity chips ([#126](https://github.com/shrujancheruku/issho/issues/126)) ([4330321](https://github.com/shrujancheruku/issho/commit/43303218f4036f944bd1981ce0690e55f6771bbf))
* **infra:** change the follow-up in the site's group chat ([#124](https://github.com/shrujancheruku/issho/issues/124)) ([fd45efc](https://github.com/shrujancheruku/issho/commit/fd45efca8256d91d55a1e25c396e3c3550242bb1))
* **infra:** darken the site's footer in light mode ([#125](https://github.com/shrujancheruku/issho/issues/125)) ([3971783](https://github.com/shrujancheruku/issho/commit/3971783da33a04840eed3448327a521b41b24d96))
* **infra:** deploy writes the missing Kamal role env files before migrating ([#285](https://github.com/shrujancheruku/issho/issues/285)) ([96fd3b2](https://github.com/shrujancheruku/issho/commit/96fd3b213d546997888afb878b2586941f504434))
* **infra:** drop the site's Love it chip from the top of the plan card ([#121](https://github.com/shrujancheruku/issho/issues/121)) ([f958029](https://github.com/shrujancheruku/issho/commit/f9580294da8b7f36f1ae3254a2d287525a26835c))
* **infra:** keep the site's day-card buttons side by side on phones ([#127](https://github.com/shrujancheruku/issho/issues/127)) ([674cc47](https://github.com/shrujancheruku/issho/commit/674cc47b65d55116b19c51ef7a5ee2550804a57d))
* **infra:** level the site's heading quotes; README follows the site ([#117](https://github.com/shrujancheruku/issho/issues/117)) ([1f82fdf](https://github.com/shrujancheruku/issho/commit/1f82fdf4ac1921fd5474576454825a84381dc7cf))
* **infra:** play the site's panels when they reach mid-screen ([#123](https://github.com/shrujancheruku/issho/issues/123)) ([1170623](https://github.com/shrujancheruku/issho/commit/1170623cb0dcfc87dab0b30d7db15e88d3ba9cc9))
* **infra:** run kamal migrate on the web role only ([#120](https://github.com/shrujancheruku/issho/issues/120)) ([09c74e3](https://github.com/shrujancheruku/issho/commit/09c74e39c1716431fe03cc64bce9b1e78b710009))
* **infra:** show who's going on the site's plan card and even out its dashed rings ([#122](https://github.com/shrujancheruku/issho/issues/122)) ([f843701](https://github.com/shrujancheruku/issho/commit/f8437014a3098ebfe3fec805d5f3fc2cb1facfdc))
* **infra:** smooth the site's picked-slot pop ([#114](https://github.com/shrujancheruku/issho/issues/114)) ([6f4bd08](https://github.com/shrujancheruku/issho/commit/6f4bd08f35694b4e04187eaafc2470ed71fa1d53))
* **infra:** softer Japanese sign-off in the site footer ([#116](https://github.com/shrujancheruku/issho/issues/116)) ([c061f6e](https://github.com/shrujancheruku/issho/commit/c061f6ef5d83884be8cb2412c5892c59ed5066f0))
* **mobile:** keep loaded data when a refetch fails ([#273](https://github.com/shrujancheruku/issho/issues/273)) ([2aa1d13](https://github.com/shrujancheruku/issho/commit/2aa1d13174ee31b044fbc3b0765f2893271c3221))
* **mobile:** save a profile from settings only when every section is complete ([#133](https://github.com/shrujancheruku/issho/issues/133)) ([48258d9](https://github.com/shrujancheruku/issho/commit/48258d9799076f11f0e3094fe11d313ae9386849))
* **mobile:** stop leaked query gc timers holding Jest open for 5 minutes ([#246](https://github.com/shrujancheruku/issho/issues/246)) ([1b67a0a](https://github.com/shrujancheruku/issho/commit/1b67a0a1cb7bf9bf1ee41158398878d8730bbb0c))
* **mobile:** stop map crashes, load faster, trace performance ([#111](https://github.com/shrujancheruku/issho/issues/111)) ([3a37d0b](https://github.com/shrujancheruku/issho/commit/3a37d0bc33f6e5e2e5ee6dd5707d70f805f3f3fd))
* **mobile:** unblock Android builds; smoother site copy and footer ([#115](https://github.com/shrujancheruku/issho/issues/115)) ([d16d642](https://github.com/shrujancheruku/issho/commit/d16d6427bdb2fc778504e83165ec73369f111c51))
* shutdown drain, crews_city retries, worker UI strings and smaller review fixes ([#134](https://github.com/shrujancheruku/issho/issues/134)) ([9224ace](https://github.com/shrujancheruku/issho/commit/9224ace299546e269c39858a9313c21e030f5385))
* update go work sum ([b953c2d](https://github.com/shrujancheruku/issho/commit/b953c2dbcd1ffa29035bc188562572066ed73e76))
* Update README.md ([4fc3bdd](https://github.com/shrujancheruku/issho/commit/4fc3bdd064871ea6f1964fd47df062f7d45230ca))


### Performance Improvements

* **api:** batch rpc for crew suggestion cards, one call instead of a GetPlan each ([#284](https://github.com/shrujancheruku/issho/issues/284)) ([f30b312](https://github.com/shrujancheruku/issho/commit/f30b31277f23c1face2282047b440b58b6e6971e))
* **backend:** batch the matching pass's turnout query, signup loads and matrix writes ([#294](https://github.com/shrujancheruku/issho/issues/294)) ([4cd1443](https://github.com/shrujancheruku/issho/commit/4cd144390a5d12f9ca5e6ce4c2ffa61275465460))


### Code Refactoring

* **api:** typed enums instead of strings in the admin protos ([#264](https://github.com/shrujancheruku/issho/issues/264)) ([fc2a621](https://github.com/shrujancheruku/issho/commit/fc2a62104e92e3fb0bf7c7e03f3fb540b57407d7))
* clear technical debt; remove ModerationService's unused RPCs ([#119](https://github.com/shrujancheruku/issho/issues/119)) ([110a39f](https://github.com/shrujancheruku/issho/commit/110a39fcc2bf7a2ec12576082a7281bd6bc21363))
* **db:** one baseline migration with the cleaned schema; reset issho-dev ([#224](https://github.com/shrujancheruku/issho/issues/224)) ([6e2219f](https://github.com/shrujancheruku/issho/commit/6e2219fa6af379ea6b75a59919e6cefcba6a1734))
* one chat module for outing and crew rooms; one socket per room in the app ([#138](https://github.com/shrujancheruku/issho/issues/138)) ([e2b3d60](https://github.com/shrujancheruku/issho/commit/e2b3d605d24f921c879c80acd9b7a57538ab6324))

## [0.9.0](https://github.com/shrujancheruku/issho/compare/v0.8.0...v0.9.0) (2026-10-26)


### Features

* **admin:** city-pack editor that opens pull requests (M8) ([#62](https://github.com/shrujancheruku/issho/issues/62)) ([8c1bbad](https://github.com/shrujancheruku/issho/commit/8c1bbad5dd8a0b00ee55deef8872c2e2074b1f3d))
* **admin:** domain metrics (M8) ([#61](https://github.com/shrujancheruku/issho/issues/61)) ([a804eea](https://github.com/shrujancheruku/issho/commit/a804eea5d6d3f99e77a6b29b34b01707e077302f))
* **admin:** edit a plan's stops (M8) ([#59](https://github.com/shrujancheruku/issho/issues/59)) ([5ab5285](https://github.com/shrujancheruku/issho/commit/5ab52854309e80c19b999f511824b822c9c53129))
* **admin:** suggest better translations with Gemini in the editor ([#78](https://github.com/shrujancheruku/issho/issues/78)) ([a5533d5](https://github.com/shrujancheruku/issho/commit/a5533d529e134aee36cc53944836fe87bf008309))
* **admin:** translation editor page and a translators-only view ([#77](https://github.com/shrujancheruku/issho/issues/77)) ([48e3754](https://github.com/shrujancheruku/issho/commit/48e3754418b9867a5a004a028209c578b801d2a4))
* **api:** add Traditional Chinese as a display language ([#83](https://github.com/shrujancheruku/issho/issues/83)) ([d12b748](https://github.com/shrujancheruku/issho/commit/d12b7488e9c040a625fe47ee6538ba53d8436851))
* **api:** check cached UI strings by version so corrections reach phones ([#72](https://github.com/shrujancheruku/issho/issues/72)) ([adcb33a](https://github.com/shrujancheruku/issho/commit/adcb33ae8084a36c4ed776cb4985aa4ad3c249fb))
* **api:** translation editor service with a translator role ([#76](https://github.com/shrujancheruku/issho/issues/76)) ([05072fe](https://github.com/shrujancheruku/issho/commit/05072fe51570c120f6c12848af3b43e8dcfc4cd0))
* apply the Issho design system to the app, admin and site ([#70](https://github.com/shrujancheruku/issho/issues/70)) ([af41c77](https://github.com/shrujancheruku/issho/commit/af41c778bd6575119f3e7a376c0d81f9eb37b15b))
* **backend:** add a Gemini translator with per-key notes and a translation guide ([#73](https://github.com/shrujancheruku/issho/issues/73)) ([e0f0978](https://github.com/shrujancheruku/issho/commit/e0f097853693a8a7448c6eeec8a21a7463028073))
* **backend:** make Gemini the default translator and describe crew per language ([#74](https://github.com/shrujancheruku/issho/issues/74)) ([75c438b](https://github.com/shrujancheruku/issho/commit/75c438b7919bb6d76f522c85c5b1253819b64867))
* **backend:** tool sentry-test sends one test error ([#101](https://github.com/shrujancheruku/issho/issues/101)) ([b4bb2e5](https://github.com/shrujancheruku/issho/commit/b4bb2e5f42fb40b1beccaeccc6c7d052f7c46c62))
* human versions of UI strings over a plain English source ([#71](https://github.com/shrujancheruku/issho/issues/71)) ([4c3844c](https://github.com/shrujancheruku/issho/commit/4c3844cd956f5e4823cece2e9a70d59edde0133c))
* **infra:** go live on joinissho.com ([#95](https://github.com/shrujancheruku/issho/issues/95)) ([d0dea0e](https://github.com/shrujancheruku/issho/commit/d0dea0e373c15031ef5de8be0839db74b66c00ad))
* **infra:** host the sites and API under joinissho.com ([#93](https://github.com/shrujancheruku/issho/issues/93)) ([32dc012](https://github.com/shrujancheruku/issho/commit/32dc0122fb7c970ef49666418b54cd44dd685da7))
* **infra:** nightly off-site database backup to Cloudflare R2 ([#92](https://github.com/shrujancheruku/issho/issues/92)) ([2f2a264](https://github.com/shrujancheruku/issho/commit/2f2a264c70e2c4984624b9d0d5db57ef570fb6a7))
* **infra:** point the Kamal deploy at the Tokyo server ([#94](https://github.com/shrujancheruku/issho/issues/94)) ([df0b2b4](https://github.com/shrujancheruku/issho/commit/df0b2b4bb0c56371a81c5c1fc2d8b23ea5b6d9e3))
* **infra:** switch on the nightly backup to R2 ([#102](https://github.com/shrujancheruku/issho/issues/102)) ([c962d06](https://github.com/shrujancheruku/issho/commit/c962d063712e617f064829285919028e9440ac97))
* **infra:** wire Sentry error reporting, off until a DSN is set ([#91](https://github.com/shrujancheruku/issho/issues/91)) ([6fbcefb](https://github.com/shrujancheruku/issho/commit/6fbcefb2f13d9cc0d1b74936433f13e451680278))
* **matching:** weigh one activity against two in every plan ([#89](https://github.com/shrujancheruku/issho/issues/89)) ([3d29f55](https://github.com/shrujancheruku/issho/commit/3d29f55f5dce5891a5ab49db0680b635aee494fd))
* **mobile:** add expo-updates for over-the-air updates ([#88](https://github.com/shrujancheruku/issho/issues/88)) ([4acc98e](https://github.com/shrujancheruku/issho/commit/4acc98eaaa301da7b082c8d5a66208bca99aef4e))
* **mobile:** app icon and splash from the brand mark ([#98](https://github.com/shrujancheruku/issho/issues/98)) ([d9917b5](https://github.com/shrujancheruku/issho/commit/d9917b5df8642d670ce5dbbf5516b473908b0410))
* **mobile:** bundle the city packs' languages for a fresh install ([#79](https://github.com/shrujancheruku/issho/issues/79)) ([f06b15e](https://github.com/shrujancheruku/issho/commit/f06b15e6c2ef47552ede76ca321ba58e580c17e9))
* **mobile:** confirm account deletion on the web too ([#90](https://github.com/shrujancheruku/issho/issues/90)) ([7151cc7](https://github.com/shrujancheruku/issho/commit/7151cc779d116336690794c0bfad6e6734afb473))
* **mobile:** lay out right to left for Arabic, Hebrew and others ([#85](https://github.com/shrujancheruku/issho/issues/85)) ([5341513](https://github.com/shrujancheruku/issho/commit/534151359c23345bbf38cf60e9723b6effbb5418))
* **mobile:** let suspended people delete their account ([#65](https://github.com/shrujancheruku/issho/issues/65)) ([caefeb8](https://github.com/shrujancheruku/issho/commit/caefeb86ce907c6a10b7812989fb59866fdc9bd8))
* **mobile:** mirror-safe styles and a fake language for layout checks ([#82](https://github.com/shrujancheruku/issho/issues/82)) ([0c454a5](https://github.com/shrujancheruku/issho/commit/0c454a54a7d5c794958da0dc7bf8317eb1c2735a))
* **mobile:** pick the typeface by the text's language ([#80](https://github.com/shrujancheruku/issho/issues/80)) ([6d1a8ba](https://github.com/shrujancheruku/issho/commit/6d1a8ba9c11cf324158f95323b9438c3bf088d47))
* **mobile:** plural forms per language in UI text ([#84](https://github.com/shrujancheruku/issho/issues/84)) ([f1a98d9](https://github.com/shrujancheruku/issho/commit/f1a98d9105b58b02aaaf4453e0d9a3713fe1c86e))
* **mobile:** upload the app's source maps to Sentry from EAS builds ([#104](https://github.com/shrujancheruku/issho/issues/104)) ([31cdc98](https://github.com/shrujancheruku/issho/commit/31cdc98239502c9b1e83b95ee412552ff4f874bf))


### Bug Fixes

* **backend:** keep the admin's email out of city-pack pull requests ([#81](https://github.com/shrujancheruku/issho/issues/81)) ([ca62ccd](https://github.com/shrujancheruku/issho/commit/ca62ccdab0e290fb43490214ca57709008ee7551))


### Performance Improvements

* **backend:** cache translations in one write and ping idle connections less ([#106](https://github.com/shrujancheruku/issho/issues/106)) ([f3bd012](https://github.com/shrujancheruku/issho/commit/f3bd0129b9d4ba9a8ab9929906ba565269b6f17a))
* **backend:** load profiles in one query and cache city data for the feed ([#107](https://github.com/shrujancheruku/issho/issues/107)) ([ad775c9](https://github.com/shrujancheruku/issho/commit/ad775c9ad3afcc23fb1114568c883fb7ac3d281c))

## [0.8.0](https://github.com/shrujancheruku/issho/compare/v0.7.0...v0.8.0) (2026-10-24)


### Features

* **admin:** admin SPA with the moderation queue (M8) ([#51](https://github.com/shrujancheruku/issho/issues/51)) ([14dfe73](https://github.com/shrujancheruku/issho/commit/14dfe73c8719003bf5970e379828fbb752726d06))
* **admin:** plan review and plans made by hand (M8) ([#56](https://github.com/shrujancheruku/issho/issues/56)) ([b6d9d10](https://github.com/shrujancheruku/issho/commit/b6d9d1092ff31a13d65c27708f32e6ca0ea4cbdb))
* **admin:** review proposed groups before invitations (M8) ([#57](https://github.com/shrujancheruku/issho/issues/57)) ([7279206](https://github.com/shrujancheruku/issho/commit/7279206a8610efd9e9964229cc440cb055a1c56e))
* **admin:** suspend accounts and a person view (M8) ([#55](https://github.com/shrujancheruku/issho/issues/55)) ([0856a82](https://github.com/shrujancheruku/issho/commit/0856a822bf510fd177088f989c355825191e55e0))
* **admin:** venue curation (M8) ([#58](https://github.com/shrujancheruku/issho/issues/58)) ([a40ca98](https://github.com/shrujancheruku/issho/commit/a40ca981face6c456b17a86cb35102b2e09245eb))
* **web:** host the admin SPA and web build on Cloudflare Workers ([#52](https://github.com/shrujancheruku/issho/issues/52)) ([3cdeeeb](https://github.com/shrujancheruku/issho/commit/3cdeeebf991d26d2a5058d425c81018beaf43444))

## [0.7.0](https://github.com/shrujancheruku/issho/compare/v0.6.0...v0.7.0) (2026-10-23)


### Features

* **api:** compose walks as itineraries, with a Gemini vs Claude benchmark ([#45](https://github.com/shrujancheruku/issho/issues/45)) ([fecc9fb](https://github.com/shrujancheruku/issho/commit/fecc9fbcad472d24970818a898f5e68d7984b551))
* **api:** feedback, relationships, friends in matching and crews (M7) ([#47](https://github.com/shrujancheruku/issho/issues/47)) ([cd1f77e](https://github.com/shrujancheruku/issho/commit/cd1f77e3e7e168a10a095e2d29571a6e64ccc94a))


### Bug Fixes

* **app:** name the plan on crew suggestions; clearer leave note (M7) ([#48](https://github.com/shrujancheruku/issho/issues/48)) ([c2653bf](https://github.com/shrujancheruku/issho/commit/c2653bf7ce034e2dc2151f877ce1b7cd3c2f0c0d))

## [0.6.0](https://github.com/shrujancheruku/issho/compare/v0.5.0...v0.6.0) (2026-10-22)


### Features

* **api:** check-in and sharing an outing with a trusted contact (M6) ([#40](https://github.com/shrujancheruku/issho/issues/40)) ([233869f](https://github.com/shrujancheruku/issho/commit/233869f6d99e1dfc6de94782a0c75f66f119dc2a))
* **api:** event invitations, confirmations and the event screen (M6) ([50b72cd](https://github.com/shrujancheruku/issho/commit/50b72cd49fdeb8f3af5a142783bac65a9c6e2353))
* **api:** event invitations, confirmations and the event screen (M6) ([#33](https://github.com/shrujancheruku/issho/issues/33)) ([722b9d8](https://github.com/shrujancheruku/issho/commit/722b9d8055517f463f27bb286a137e26255b6aaf))
* **api:** group chat for the people going on an event (M6) ([#36](https://github.com/shrujancheruku/issho/issues/36)) ([5e23287](https://github.com/shrujancheruku/issho/commit/5e2328765197f9af73b0a40667b6e1c855baa7c1))
* **api:** keep a closed plan's screen for two weeks ([#44](https://github.com/shrujancheruku/issho/issues/44)) ([497047a](https://github.com/shrujancheruku/issho/commit/497047afabf03c99d37113f4fdd8592b8f7b7083))
* **api:** meeting point for plans that start with a walk (M6) ([#42](https://github.com/shrujancheruku/issho/issues/42)) ([282a984](https://github.com/shrujancheruku/issho/commit/282a9841671d4dad437bde3be3fbc23dca4ebb1a))
* **api:** plan and event maps with the walking route (M6) ([#39](https://github.com/shrujancheruku/issho/issues/39)) ([d4efe62](https://github.com/shrujancheruku/issho/commit/d4efe62373427e4fe6c1f4d18d3f236fb51f93f6))
* **api:** plan feed, plan details and signups (M6) ([ff9cfb8](https://github.com/shrujancheruku/issho/commit/ff9cfb8378af10c67ce7eaa6389255f039c96ea6))
* **api:** plan feed, plan details and signups (M6) ([#31](https://github.com/shrujancheruku/issho/issues/31)) ([5b18bd8](https://github.com/shrujancheruku/issho/commit/5b18bd859a8d3f028e827ac2e653fae8e18341ac))
* **api:** plan titles, venues and a timeline of stops ([f597c21](https://github.com/shrujancheruku/issho/commit/f597c21752fb10d5862792257603c103c9cf1b8f))
* **api:** push delivery receipts and a notice when a plan closes (M6) ([#43](https://github.com/shrujancheruku/issho/issues/43)) ([89cb94a](https://github.com/shrujancheruku/issho/commit/89cb94a18afa712e74894c6198959499f470d0c4))
* **api:** push notifications for invitations and group changes (M6) ([80e867c](https://github.com/shrujancheruku/issho/commit/80e867ceb91e0e809e8277f1eb3a749ae872d828))
* **api:** push notifications for invitations and group changes (M6) ([#34](https://github.com/shrujancheruku/issho/issues/34)) ([69ca4e3](https://github.com/shrujancheruku/issho/commit/69ca4e30d8325affdb16df85e7ac9e06eaffb834))
* **backend:** travel-time refresh and venue opening hours (M6) ([6a8bf74](https://github.com/shrujancheruku/issho/commit/6a8bf743b949a1ff82ddadea5bf00022134db1d6))
* **backend:** travel-time refresh and venue opening hours (M6) ([#35](https://github.com/shrujancheruku/issho/issues/35)) ([ac7184e](https://github.com/shrujancheruku/issho/commit/ac7184eeffc3e4b669d19b2235bb38fb79bdf2a6))
* **backend:** worker that opens and schedules plans (M6) ([b84a5d9](https://github.com/shrujancheruku/issho/commit/b84a5d91247b74fb870df28c6e6163c70a686430))
* **backend:** worker that opens and schedules plans (M6) ([#30](https://github.com/shrujancheruku/issho/issues/30)) ([1a7ca4e](https://github.com/shrujancheruku/issho/commit/1a7ca4ed4eb85dbb9a444bb5cdf8307e978743bf))
* **db:** plans, signups, events and groups schema (M6) ([a662f2a](https://github.com/shrujancheruku/issho/commit/a662f2a1e3d6857d14f5acf3134ae4a7d8e2cf4f))
* **db:** plans, signups, events and groups schema (M6) ([#29](https://github.com/shrujancheruku/issho/issues/29)) ([3077d43](https://github.com/shrujancheruku/issho/commit/3077d43481d9320dd5878d33dc430df3bb9a9ff4))
* **mobile:** map of open plans on Google Maps (M6) ([#38](https://github.com/shrujancheruku/issho/issues/38)) ([43fec9c](https://github.com/shrujancheruku/issho/commit/43fec9c3ce61c86054595e9a1c4850c2f96562eb))
* **mobile:** plan details as a card of labelled rows ([0303672](https://github.com/shrujancheruku/issho/commit/0303672b8b0485ae5c495e85a9f9b2b70f4cef28))
* **mobile:** plan feed and signup screen (M6) ([6901f2b](https://github.com/shrujancheruku/issho/commit/6901f2bc6049be91604a39a91fdf98d5748e6e5b))
* **mobile:** return to the feed after signing up ([e50800b](https://github.com/shrujancheruku/issho/commit/e50800b96b94cf32e044f36cf9358c58ef7a04db))
* plan feed, signup screen, titles and timeline (M6) ([#32](https://github.com/shrujancheruku/issho/issues/32)) ([41fbcef](https://github.com/shrujancheruku/issho/commit/41fbcef97728e6d360108cf267b2451a52641297))


### Bug Fixes

* **ci:** check PostGIS health over TCP, not the init-time socket ([8ff3dee](https://github.com/shrujancheruku/issho/commit/8ff3dee28eda01d295762bd6a764c2161597147f))
* **deps:** pin expo-notifications 58.1.5 instead of exempting day-old packages ([f2f3468](https://github.com/shrujancheruku/issho/commit/f2f34680a1c93d44277be206513c34ec828f3ad9))
* drop bracketed examples from plan titles and stops ([8e57310](https://github.com/shrujancheruku/issho/commit/8e57310bfc5963fd56d28e54db94684072547885))
* **mobile:** attribute Google venue data on plan and event timelines ([#37](https://github.com/shrujancheruku/issho/issues/37)) ([6ed1ae2](https://github.com/shrujancheruku/issho/commit/6ed1ae21cc1331f3ac5de20c6054f3a6103dd59d))
* **mobile:** avoid toSorted, which Hermes lacks; lint against it ([ec787b7](https://github.com/shrujancheruku/issho/commit/ec787b7bb9952fed2d8033ef52627f28edb720b9))
* **mobile:** log why push registration is skipped in development builds ([8f5f7f1](https://github.com/shrujancheruku/issho/commit/8f5f7f1ec4c7ec60342d2d02294d64db4b4c0ab4))
* **mobile:** plan screen layout, translations and stale UI text ([9b93606](https://github.com/shrujancheruku/issho/commit/9b93606c47e6dcc613acccc01340bf196ac80673))

## [0.5.0](https://github.com/shrujancheruku/issho/compare/v0.4.0...v0.5.0) (2026-10-21)


### Features

* **api:** profile, city/language lists and UI text RPCs ([4ce4722](https://github.com/shrujancheruku/issho/commit/4ce47222fb1c4f997005ba1e45dca458adfaf959))
* **api:** suggest activities from free-text interests with Claude ([bc2d20a](https://github.com/shrujancheruku/issho/commit/bc2d20abd85e4fc7047cc3b37d6e383cf18f23cb))
* **data:** merge the activity catalogue from 230 into 57 ([c228351](https://github.com/shrujancheruku/issho/commit/c228351062b984e6cfe6b47dc2d6f51bd60b6a17))
* **data:** replace "Try a new sport" with the sports it covered ([477a476](https://github.com/shrujancheruku/issho/commit/477a476b8b720cf4ae1b0e0268a0308ea94ff82c))
* **data:** split team and racket sports into individual sports ([77fe52a](https://github.com/shrujancheruku/issho/commit/77fe52a190c5a44876040bc598112b7116dd6f38))
* **infra:** sign-in emails in the user's language ([c75ea73](https://github.com/shrujancheruku/issho/commit/c75ea73dcfb4683bc27c81483077414782ccf562))
* **infra:** sign-in emails in the user's language (M5) ([#24](https://github.com/shrujancheruku/issho/issues/24)) ([a23ec9c](https://github.com/shrujancheruku/issho/commit/a23ec9c996d88209b2b7374e6c5e1b084020b725))
* M5 profile, onboarding and app i18n ([#20](https://github.com/shrujancheruku/issho/issues/20)) ([8fb0aaf](https://github.com/shrujancheruku/issho/commit/8fb0aaf9fd691f6bbe9dd096c149d4a55c7f73a4))
* **mobile:** describe your interests and confirm suggested activities ([400ae69](https://github.com/shrujancheruku/issho/commit/400ae69855ddfc05580096fbfadb6f12b85060e2))
* **mobile:** explain interest levels before the activity grid ([0501330](https://github.com/shrujancheruku/issho/commit/050133086423247c84dffe45216e0ea477aae329))
* **mobile:** fit form screens to a centred column on wide screens ([4ad3d20](https://github.com/shrujancheruku/issho/commit/4ad3d203ee39153c45d9ccf2a070107f807b227e))
* **mobile:** keep a web build working, checked in CI ([2d7fdbc](https://github.com/shrujancheruku/issho/commit/2d7fdbc08bb91cdd261d8a5f967e1d237435746b))
* **mobile:** keep a web build working, fitted to desktop screens ([#21](https://github.com/shrujancheruku/issho/issues/21)) ([a7bcbd7](https://github.com/shrujancheruku/issho/commit/a7bcbd72404a3fc50a7c4dead79880391aee2563))
* **mobile:** onboarding, profile settings and app i18n ([7ae534a](https://github.com/shrujancheruku/issho/commit/7ae534a22c63dd907383560f15492eabbec294b6))
* suggest activities from free-text interests with Gemini on Agent Platform (M5) ([#23](https://github.com/shrujancheruku/issho/issues/23)) ([17bb55e](https://github.com/shrujancheruku/issho/commit/17bb55e87445e2d2af48ce24e56d6fb60d015480))


### Bug Fixes

* **api:** call Gemini through Google Cloud's Agent Platform ([3eae8f6](https://github.com/shrujancheruku/issho/commit/3eae8f60162c295c824d6580ac8426521801e625))
* **api:** use current activity ids in the profile tests ([94f40c4](https://github.com/shrujancheruku/issho/commit/94f40c4619a4536c5cadb7d6de44348acedd79dc))
* **mobile:** code of conduct asks for platonic intentions, not a ban ([b2dea3f](https://github.com/shrujancheruku/issho/commit/b2dea3ffea58d69475b5211a5d352d9181bfdfe7))
* **mobile:** remove the animated example tile from the interest legend ([d069469](https://github.com/shrujancheruku/issho/commit/d0694699e11943a45bac9b64126f2d7c9a86b514))
* **mobile:** show the code-of-conduct heading once ([448db9e](https://github.com/shrujancheruku/issho/commit/448db9edd324d37d79b500f736d6ec87ee72aa1c))
* **mobile:** start from the device's first two supported languages ([e0ca66d](https://github.com/shrujancheruku/issho/commit/e0ca66daea07af133032112bad00a309c00be91c))
* **mobile:** wrap long chip labels and drop the duplicate availability heading ([78d0bf8](https://github.com/shrujancheruku/issho/commit/78d0bf81c93dccb8708ab9d2c65196621855f4d3))

## [0.4.0](https://github.com/shrujancheruku/issho/compare/v0.3.0...v0.4.0) (2026-10-21)


### Features

* **infra:** add marketing landing page and GitHub Pages deploy ([#8](https://github.com/shrujancheruku/issho/issues/8)) ([4149f56](https://github.com/shrujancheruku/issho/commit/4149f5666782733b208f6dfa4ff278f2221b3bed))
* **infra:** dark theme, Dela Gothic One and reordered sections ([#10](https://github.com/shrujancheruku/issho/issues/10)) ([debab34](https://github.com/shrujancheruku/issho/commit/debab34b4fc1038bf88c3c372fbb4b28b8e30e70))
* **infra:** favicon circle, lighter hero, kanji meanings, sleeker steps, copy tweaks ([#11](https://github.com/shrujancheruku/issho/issues/11)) ([2eed1fa](https://github.com/shrujancheruku/issho/commit/2eed1fae85391a16219d96e7a39c94b3cf5f87ff))
* **infra:** rewrite the landing page copy for flow and voice ([#16](https://github.com/shrujancheruku/issho/issues/16)) ([e85abbe](https://github.com/shrujancheruku/issho/commit/e85abbee9186fdd14def6676d53bd91428f78198))
* **infra:** seigaiha dividers, logo-coloured kanji and a thread timeline ([#12](https://github.com/shrujancheruku/issho/issues/12)) ([a24b687](https://github.com/shrujancheruku/issho/commit/a24b687823017ff17376658e0883e59736a346c6))
* **infra:** set the kanji in the logo's font ([#15](https://github.com/shrujancheruku/issho/issues/15)) ([03da5eb](https://github.com/shrujancheruku/issho/commit/03da5eb0aed7032b560bca9e8b57eb0c887c17e4))
* **infra:** thinner two-sided dividers, aligned timeline and kanji statement words ([#13](https://github.com/shrujancheruku/issho/issues/13)) ([21c7d6e](https://github.com/shrujancheruku/issho/commit/21c7d6e3d41ced262341c5c2aef4fc55c9b49c11))
* **matching:** candidate plans, group assembly, invites and a simulator ([5b9eaea](https://github.com/shrujancheruku/issho/commit/5b9eaea91b9155c48e7436e16ab813de80b52ad4))
* **matching:** M4 matching engine (plan first, time second) ([#17](https://github.com/shrujancheruku/issho/issues/17)) ([5c374c4](https://github.com/shrujancheruku/issho/commit/5c374c4f140a0229509f581a2731eb1b4ea4eadf))
* **matching:** make group size and gender balance preferences ([0e41111](https://github.com/shrujancheruku/issho/commit/0e41111766c0e4452dfc9a0487ff95df0257cfb6))
* **matching:** plan first, time second ([2ab3665](https://github.com/shrujancheruku/issho/commit/2ab36655e187301795f6b1a132ac9882d855939d))
* **matching:** pure matching package with step 1 bucketing ([396b1a4](https://github.com/shrujancheruku/issho/commit/396b1a401d3616a00b4366a57f82ae23a8a13726))


### Bug Fixes

* **infra:** centre the timeline thread on each sun ([#14](https://github.com/shrujancheruku/issho/issues/14)) ([56b0474](https://github.com/shrujancheruku/issho/commit/56b0474eeee96c9f2f5b1dcd9842495f7ea6c841))

## [0.3.0](https://github.com/shrujancheruku/issho/compare/v0.2.0...v0.3.0) (2026-10-20)


### Features

* M3 walk/drive travel times, venue discovery, on-demand translation and an Austin pack ([#5](https://github.com/shrujancheruku/issho/issues/5)) ([877265c](https://github.com/shrujancheruku/issho/commit/877265cf4f4f1ce45d09ed8bd5070be057ce579c))

## [0.2.0](https://github.com/shrujancheruku/issho/compare/v0.1.0...v0.2.0) (2026-10-20)


### Features

* **api:** add IdentityService.DeleteAccount ([13b884f](https://github.com/shrujancheruku/issho/commit/13b884f432eefd991edad45b80f4b4514c69999b))
* **brand:** add Issho logo and mark ([e24adce](https://github.com/shrujancheruku/issho/commit/e24adce4b40e56b539d07ba385047bd2d0e17ac2))
* **brand:** layer outlined parts and switch the wordmark font ([50e870c](https://github.com/shrujancheruku/issho/commit/50e870c14ef0fa9f1c9578ef3745e6a5edb45a15))
* **brand:** layer outlined parts and switch the wordmark font ([#4](https://github.com/shrujancheruku/issho/issues/4)) ([d39d464](https://github.com/shrujancheruku/issho/commit/d39d46478a27c2b41e41d7b1d14e77d03e64b344))
* **brand:** use a single outlined logo on every background ([0a37f43](https://github.com/shrujancheruku/issho/commit/0a37f437b06593986b885ce87a9a1d8cb8566597))
* **infra:** configure issho-dev email via Resend SMTP ([221031d](https://github.com/shrujancheruku/issho/commit/221031d073eb8bffaad6e8d003a7c70f85329d8f))
* **infra:** send 6-digit sign-in codes instead of links ([fcd9193](https://github.com/shrujancheruku/issho/commit/fcd919367b0f90962ab6d6715d0b8342ab5985cc))
* M2 email sign-in, hosted Supabase, secure sessions and account deletion ([#3](https://github.com/shrujancheruku/issho/issues/3)) ([e0e5f2d](https://github.com/shrujancheruku/issho/commit/e0e5f2da5825b34726a8e129a891e3db008449ef))
* **mobile:** sign in with email codes and gate navigation on auth ([bf0afa6](https://github.com/shrujancheruku/issho/commit/bf0afa69aaa27de1062d15de7e94de8b163fb2d3))
* **mobile:** use Dela Gothic One for standout text ([2d92f8b](https://github.com/shrujancheruku/issho/commit/2d92f8be2541189ef337f4f1b02c59a0eb375de8))


### Bug Fixes

* **backend:** stop leaking internal error details to clients ([f561bcb](https://github.com/shrujancheruku/issho/commit/f561bcb3c6237362570f1b38d81c2af71656d10e))
* **db:** keep migration bookkeeping out of the public schema ([974f1ae](https://github.com/shrujancheruku/issho/commit/974f1aea6d7bfc111601b546b9206975a2b91898))
* **infra:** disable the Supabase Data API locally ([6427414](https://github.com/shrujancheruku/issho/commit/6427414c3a4fd8fdd6ef99a42561f2ae021da5ea))
* **mobile:** keep the sign-in code field above the keyboard on Android ([b807646](https://github.com/shrujancheruku/issho/commit/b807646883ca7785418904f9d3100ee4cc6db00c))

## 0.1.0 (2026-10-20)


### Features

* scaffold Issho monorepo with Go API, Expo app and local Supabase ([75ce44c](https://github.com/shrujancheruku/issho/commit/75ce44cd8b24ded9ac7f359ae5cd17e0ad4532ec))


### Bug Fixes

* **backend:** deny cross-origin requests when no origins are configured ([6d64b3d](https://github.com/shrujancheruku/issho/commit/6d64b3d7151ec2d1cf22f1fc4b60bf658b33b6ee))
* **ci:** make CI pass from a clean checkout ([75f7a20](https://github.com/shrujancheruku/issho/commit/75f7a203a86136574af2e834debf329393f4ceff))
