# The owner operates Issho as an individual until the public launch

Vietnam's PDPL and cybersecurity rules need a named operator (the controller who files the impact assessments and answers the Ministry of Public Security). For the Pilot and the soft launch that operator is the owner, as an individual and a US resident, with no company. A Texas single-member LLC takes over before the public launch ([The stages of launch](../spec/launch.md)). We chose this because incorporating costs money and admin before there is anything to protect, and nothing before the public launch strictly needs a company. Details are in the resolution of [Legal operator and Vietnam filings](https://github.com/shrujancheruku/issho/issues/362), and the facts are in [the research findings](https://github.com/shrujancheruku/issho/blob/research/vietnam-data-localization/docs/research/vietnam-data-localization.md).

## Considered options

- **A US LLC from the first member.** Rejected for now. It would keep Issho a "foreign provider" while the owner lives in Vietnam during the Pilot, limit liability, skip a handover later, and probably exempt the Play account from the 12-tester closed test. Deferred, not dropped.
- **A Vietnamese company.** Rejected: Decree 147's licence, `.vn` domain, servers in Vietnam and phone verification would apply from day one.

## Consequences

- The impact assessments are filed in the owner's name (individual forms, passport number). The handover to the LLC means re-filing them, new privacy policy and terms, and telling everyone, with a fresh tick where the notice's recipient changes.
- Store accounts are personal: Play's closed test applies, and Apple shows the owner's legal name as the seller.
- A lawyer checks the policy, the terms and the accepted risks before the public launch, not before ([Privacy](../spec/privacy.md#the-floor)).
- Accepted risks: the owner is personally liable; with the owner living in Vietnam on a tourist visa, both "provider from abroad" and the visa's purpose are open to question.
- When the LLC is formed, form it in Texas (no state income tax, no franchise tax below the threshold, no foreign-LLC registration at home), with default tax treatment and a commercial registered agent whose address goes on public records.
