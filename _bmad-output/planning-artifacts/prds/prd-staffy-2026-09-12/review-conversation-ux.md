---
title: 'Staffy PRD — Reviewer Gate Round 2: Conversational UX and Omnichannel Quality'
status: complete
created: '2026-09-13'
reviewed:
  - prd.md
  - addendum.md
---

# Reviewer Gate Round 2 — Conversational UX and Omnichannel Quality

## Verdict

**REVISE WITH BOUNDED UX AMENDMENTS.** The PRD already defines strong customer journeys, progressive intake, no-photo alternatives, cross-channel provenance, repair of misheard critical entities, real barge-in, two distinct confirmations, human handoff/recovery and a nontechnical setup path. No foundational conversational redesign is needed. Six high-impact interaction contracts and eight medium-impact edge paths remain underspecified.

**Severity counts (unique findings):** Critical 0 · High 6 · Medium 8 · Low 0 · Total 14.

This is a behavioral-document review for product, UX, architecture, QA and pilot operators. Adversarial and edge-path lenses were applied to the full PRD/Addendum. The structure/prose pass used the strategic/pyramid model and found no useful broad cuts in the 11,404-word PRD; its two material observations are incorporated in CX-04 and CX-07. Overlapping lens signals are tagged, not double-counted.

## Findings

### CX-01 — High — “Multitopic dialogue” is tested, but its expected interaction behavior is undefined

- **Lens:** adversarial + edge-case
- **Location:** `prd.md` FR-12, lines 386–393; FR-18, lines 444–451; FR-51, lines 757–764; FR-65, lines 887–894; NFR-29, lines 1277–1279.
- **Trigger condition:** the customer asks about two services or parts, changes the main goal, answers an older question while adding a new request, or pauses one Work Item and returns to it later.
- **Actionable guard:** add a cross-cutting conversational requirement to identify separate intents/Work Items, keep an explicit state for each, announce topic switches when ambiguity exists, allow pause/resume, and recap only unresolved/changed items before Commitment.
- **Potential consequence:** facts, prices, media or confirmations leak between tasks, or the customer must restart a complex dialogue despite passing the generic multitopic eval.

### CX-02 — High — Voice Confirmation has no explicit ambiguity and silence contract

- **Lens:** adversarial + edge-case
- **Location:** `prd.md` FR-25, lines 507–514; NFR-10, lines 1191–1193; NFR-30–NFR-31, lines 1281–1287.
- **Trigger condition:** the customer says a short backchannel such as “угу,” answers a compound question, speaks over the final terms, stays silent, the line clips the response, or another person speaks in the background.
- **Actionable guard:** define an accepted Confirmation as an unambiguous response to one presented material Commitment after the relevant content was actually delivered; silence, clipped/overlapped speech, low-confidence recognition and generic backchannels require focused repair or another modality and never count as consent.
- **Potential consequence:** the conversation sounds natural while the customer experiences an order or booking they did not understand as confirmed.

### CX-03 — High — Disconnect/reconnect is in the test set but has no customer recovery journey

- **Lens:** adversarial + edge-case
- **Location:** `prd.md` FR-12, lines 386–393; FR-17, lines 431–438; FR-25, lines 507–514; NFR-7, lines 1177–1179; NFR-10, lines 1191–1193.
- **Trigger condition:** the call drops or the chat reconnects during intake, playback of final terms, Confirmation, a tool wait or handoff.
- **Actionable guard:** require resumption from the last safe checkpoint, explicit disclosure of what was and was not completed, no inference of Confirmation from a partial utterance, and a configured choice between reconnect, callback, protected-link continuation or human fallback.
- **Potential consequence:** the client repeats the journey, receives duplicate outreach, or cannot tell whether a material action occurred.

### CX-04 — High — Admin preview is not defined as an end-to-end client-and-operator rehearsal

- **Lens:** adversarial + structure + prose
- **Location:** `prd.md` UJ-P1, lines 55–61; FR-8, lines 346–353; FR-80–FR-81, lines 1028–1046; FR-83, lines 1058–1066.
- **Trigger condition:** the configurator previews the voice and passes isolated evals, but the real route includes channel disclosure, knowledge lookup, a mutating tool proposal, Confirmation, degraded mode or handoff into the operator surface.
- **Actionable guard:** require a safe end-to-end rehearsal covering the customer view/heard output and the operator view, with non-mutating or clearly sandboxed Actions, visible expected-versus-actual results, a handoff/return test and a readiness result tied to the version being published.
- **Potential consequence:** each component passes separately while the first customer encounters a broken transition, unsafe tool proposal or unusable operator handoff.

### CX-05 — High — The customer is not explicitly told when control changes between AI and human

- **Lens:** adversarial + edge-case
- **Location:** `prd.md` FR-13, lines 395–402; FR-15, lines 413–420; FR-35, lines 605–612; FR-38, lines 632–639; FR-61, lines 847–854.
- **Trigger condition:** a human accepts the handoff, Staffy remains silent in the same chat, or control later returns to Staffy after the human resolves an exception.
- **Actionable guard:** require a customer-facing transition message identifying whether Staffy or a named/role-described human now responds, what context was passed and what happens next; on return to AI, disclose that change again and preserve the same Case/Conversation.
- **Potential consequence:** customers attribute a human promise to AI or an AI message to a human, reducing trust and making accountability unclear.

### CX-06 — High — Conversational acceptance can be masked by aggregate quality metrics

- **Lens:** adversarial + edge-case
- **Location:** `prd.md` FR-48, lines 726–733; NFR-32, lines 1289–1291; SM-11–SM-12, lines 1313–1314; SM-20, line 1322; SM-C9–SM-C10, lines 1334–1335.
- **Trigger condition:** overall semantic success is high, but voice Confirmation, partner outbound disclosure, no-photo routing, channel transition or post-handoff recovery fails for a smaller critical cohort.
- **Actionable guard:** require conversational metrics to be segmented at least by journey, channel/modality, Conversation Mode and critical transition; define which UX failures block the affected route regardless of aggregate success, then set numeric thresholds after baseline.
- **Potential consequence:** a polished average score promotes a version that reliably fails one vulnerable or high-value customer path.

### CX-07 — Medium — The economic buyer has no capability-fit and purchase journey before setup

- **Lens:** adversarial + structure
- **Location:** `prd.md` Target users, lines 29–35; JTBD, lines 37–43; UJ-P1, lines 53–61; commercial model, lines 1134–1145.
- **Trigger condition:** an owner wants to know whether Staffy supports the required process, channels and systems, what remains assisted/custom, what human coverage is required and what the pilot will cost before supplying credentials and entering setup.
- **Actionable guard:** add a short pre-setup buyer/fit journey: desired business outcome → supported Domain Pack/Connector/Capability map → explicit gaps and human obligations → indicative packaging/usage → pilot scope and evidence plan → proceed, assisted-extension request or honest no-fit.
- **Potential consequence:** the first coherent experience begins only after the buyer has already made decisions the product is supposed to clarify.

### CX-08 — Medium — Setup assumes one uninterrupted session and lacks safe draft/resume/collaboration

- **Lens:** adversarial + edge-case
- **Location:** `prd.md` UJ-P1, lines 55–61; FR-2, lines 292–299; FR-5–FR-6, lines 319–335; FR-78, lines 1008–1016.
- **Trigger condition:** telephony credentials come from an integrator, privacy approval from an owner and fallbacks from operations over several sessions or days.
- **Actionable guard:** require an unpublished draft with clear owner, incomplete/blocked states, save/resume, delegated task/request for missing input and an approval summary before publish; partial setup must never appear active.
- **Potential consequence:** administrators restart work, share credentials outside the system or publish a configuration whose missing responsibility was hidden between participants.

### CX-09 — Medium — Long tool operations have no in-conversation waiting experience

- **Lens:** adversarial + edge-case
- **Location:** `prd.md` FR-17, lines 431–438; FR-19, lines 453–460; FR-30, lines 556–563; NFR-8, lines 1183–1185.
- **Trigger condition:** catalogue lookup, supply check, order creation or reconciliation takes longer than normal conversational latency but has not failed.
- **Actionable guard:** define a threshold-based waiting state with a short truthful acknowledgement, optional progress/status where supported, the ability to cancel or switch to callback/human, and a no-fabricated-ETA rule; prevent repetitive filler from being scored as useful response.
- **Potential consequence:** dead air causes hangups and retries, while excessive filler makes the agent sound evasive and increases duplicate actions.

### CX-10 — Medium — Voice plus protected web companion has no synchronization/timeout experience

- **Lens:** adversarial + edge-case
- **Location:** `prd.md` UJ-A1, lines 177–183; UJ-M3, lines 99–107; FR-16, lines 422–429; FR-26, lines 516–523.
- **Trigger condition:** Staffy sends a VIN/photo/Confirmation link while the call remains open, but the user opens it late, submits twice, opens it on another device or never completes it.
- **Actionable guard:** require the leading channel to acknowledge link delivery and completion, correlate exactly one submission, define waiting/expiry/cancel behavior and let the customer continue through an alternative without losing already confirmed facts.
- **Potential consequence:** the call stalls, media attaches to the wrong Work Item, or duplicate submissions trigger conflicting dialogue state.

### CX-11 — Medium — Multimodal intake stores provenance but does not require usable-evidence feedback and correction

- **Lens:** adversarial + edge-case
- **Location:** `prd.md` FR-10, lines 368–375; FR-16, lines 422–429; FR-55, lines 793–800; FR-82, lines 1048–1056.
- **Trigger condition:** a photo is blurry, shows the wrong object, contains several products, a document has multiple candidate fields, or media from different Work Items arrives together.
- **Actionable guard:** require the agent to state whether evidence is usable, identify what it was associated with, request only the missing angle/field and present extracted material facts for focused correction before they affect a quote or selection.
- **Potential consequence:** the user believes media solved the question while Staffy silently guesses or attaches evidence to the wrong task.

### CX-12 — Medium — Outbound partner contact does not cover voicemail, wrong person or shared phone

- **Lens:** adversarial + edge-case
- **Location:** `prd.md` UJ-M2, lines 87–97; FR-15, lines 413–420; FR-74, lines 968–975.
- **Trigger condition:** Staffy reaches voicemail, a family member/colleague answers, the phone is shared, or the recipient denies being the named customer without explicitly disputing consent.
- **Actionable guard:** define a privacy-minimal greeting and callback path that reveals no sensitive service/order details until the intended person is established; never treat a third party, voicemail or non-answer as customer Confirmation.
- **Potential consequence:** partner outreach feels intrusive, exposes purchase/service context or creates a false record of customer engagement.

### CX-13 — Medium — Handoff context omits communication preferences and accessibility needs

- **Lens:** adversarial + edge-case
- **Location:** `prd.md` UJ-M6, lines 129–135; FR-36, lines 614–621; NFR-21–NFR-23, lines 1241–1251.
- **Trigger condition:** a customer cannot use photos, needs a slower pace, prefers text/callback, uses another supported language or has already explained an accessibility constraint before handoff.
- **Actionable guard:** include preferred channel/time/language, no-photo or accessibility accommodation and explicit “do not repeat” facts in the decision-ready handoff summary; let the operator see and honor them.
- **Potential consequence:** the human asks the same inaccessible question or calls through a channel the customer already said they cannot use.

### CX-14 — Medium — Unsupported language/code-switching has no honest fallback path

- **Lens:** adversarial + edge-case
- **Location:** `prd.md` FR-3, lines 301–308; FR-17, lines 431–438; FR-84, lines 1068–1076; NFR-21, lines 1241–1243.
- **Trigger condition:** a live caller switches language, uses a dialect/accent outside the approved voice profile or requests a supported human language that the current AI configuration cannot reliably handle.
- **Actionable guard:** require clear recognition of an unsupported-language state, a short honest explanation and a configured transfer/callback/text alternative; do not continue high-stakes qualification with unmeasured comprehension.
- **Potential consequence:** the agent appears fluent enough to proceed but misunderstands scope, numbers or consent while the case remains inside the nominal Operating Profile.

## Top findings for the gate decision

1. **CX-01:** define the actual behavior of multitopic/multi-Work-Item dialogue, not only its eval label.
2. **CX-02:** make ambiguous voice responses, silence and overlap invalid as material Confirmation.
3. **CX-03:** add a safe resume contract for disconnect during terms, confirmation, tool wait or handoff.
4. **CX-04:** make admin preview an end-to-end client-and-operator rehearsal with sandboxed actions.
5. **CX-05:** visibly disclose every AI↔human ownership transition to the customer.

## What is already adequately specified

No finding is raised against: first-answer-before-long-intake; no-repeat continuity; provenance and conflict preservation; no-photo and alternative-channel routes; protected transfer for VIN/documents/photos; one writable Inbox and Response Owner; AI disclosure and partner source disclosure; evidence-linked selling without dark patterns; two materially different Montagio confirmations; critical-entity readback; real barge-in and heard-output semantics; human-on-request; acknowledged handoff and callback fallback; linked Recovery Case with customer correction; human operator surface; voice/pronunciation preview; separate voice/text quality measurement; baseline-derived thresholds.

The Addendum correctly keeps channel/media/provider implementation replaceable. The amendments above specify observable user experience and acceptance behavior; protocols, session storage, synchronization mechanisms and UI component choices remain for UX/Architecture.
