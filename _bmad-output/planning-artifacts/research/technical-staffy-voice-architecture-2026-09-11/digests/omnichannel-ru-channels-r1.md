# Omnichannel Russia channel integration — technical digest (round 1)

**Decision served:** choose the safest, easiest integration strategy for Staffy businesses in Russia across Telegram, MAX, VK messaging, Jivo, Avito Messenger, website chat (Django and hosted builders such as ABCP), and Android/iOS applications. Include WhatsApp Business only if an official, currently usable Russia API path can be verified.

**Research date:** 2026-09-12  
**Freshness target:** compatibility <=1 month; landscape <=3 months.  
**Method:** current official API/developer/help/legal sources were preferred. No project files were inspected. “Confirmed” below means the capability is present in a current official source; it does **not** mean it has passed an account-level production test. Numerical limits and failure behavior supported by only one publisher are explicitly treated as single-source constraints and are not used as the main reason for the recommendation.

## Decision summary

There is no verified single “universal channel API.” The safest durable approach is a **Staffy-owned canonical conversation core plus a thin official adapter per channel**, with **Jivo as an optional ingress/shared-inbox bridge**, not the system of record. For any one business identity and channel, select either the direct adapter or the Jivo-mediated adapter; never run both as active writers.

Recommended initial order:

1. First-party web chat plus the normalized core and human-handoff state machine.
2. Telegram and MAX direct adapters; Jivo Bot API/Chat API for customers already using Jivo.
3. VK and Avito after account-level eligibility, permissions, callback, media, and reply-window tests.
4. Native Android/iOS clients against the same Staffy API; package a reusable SDK only after the web protocol stabilizes.
5. Defer WhatsApp Business Platform. This run found official documentation for the WhatsApp Business **app** and a reference to the Platform, but no official source establishing current API onboarding and operation for a Russia-based business/number.

## Confirmed capability matrix

Legend: **C** = confirmed in an official source retrieved this run; **P** = proposed Staffy design/assumption; **U** = not verified in the retrieved official material. Finding IDs point to the evidence records below.

| Channel | Official eligibility | Inbound transport | Outbound/reply constraints | Files / voice | Identity IDs | Delivery / read | Human handoff / shared inbox | Auth and webhook verification | Documented limits | Embed / SDK path | Rollout |
|---|---|---|---|---|---|---|---|---|---|---|
| Telegram Bot / Business Bot | **C:** bot token via BotFather; Business Connection rights exist [F1, F3] | **C:** webhook or `getUpdates`, mutually exclusive; updates retained <=24h [F1] | **C:** ordinary bot conversation begins from user interaction; Business bot `can_reply` is restricted to eligible private chats with recent incoming messages (24h) [F2, F3] | **C:** inbound files and voice; outbound document/audio/voice methods [F3] | **C:** `user.id`, `chat.id`, `message_id`, and `business_connection_id` [F1, F3] | **Partial:** Business API can mark inbound business messages read; standard bot delivery/read receipts were not found [F1] | **Partial:** business-account chat management exists; ordinary bot has no confirmed native shared inbox. Implement Staffy write lease [F3] | **C:** bot token; HTTPS webhook; optional `X-Telegram-Bot-Api-Secret-Token` [F1] | **U:** no numeric limits are used in this decision; apply conservative throttles and load-test | Deep link to bot; optional Business Connection. Web embed is a link/QR, not an embedded Telegram inbox | Tier 1 |
| MAX bot | **C:** create a bot and receive token; verified-profile/“MAX for Business” path is documented [F5, F6] | **C:** production webhook; long polling only for development/testing; cannot use both [F5, F6] | **Partial:** send to `user_id` or `chat_id`; retrieved docs did not state a general proactive-message window [F5] | **C:** image, video, audio, file uploads; a distinct voice-note semantic was not confirmed [F5] | **C:** `user_id`, `chat_id`, message/update timestamp [F5, F6] | **U:** event list confirms create/edit/remove, not delivery/read [F6] | **U:** no bot-to-human/shared-inbox primitive found in the retrieved MAX API pages; implement Staffy handoff or use Jivo if connected | **C:** `Authorization` token; HTTPS:443 with trusted certificate; optional secret checked through `X-Max-Bot-Api-Secret` [F6] | **C, single publisher:** 30 API rps overall and 2 messages/s per dialog are documented; verify in staging [F5] | Native MAX bot; `open_app` can launch a MAX mini-app [F5] | Tier 1, but canary first because API changed during 2026 |
| VK community messaging | **C:** current official schema is API v5.199 and contains community-message objects/methods [F7] | **Partial/U:** Callback API is the expected path, but the current developer portal did not render in this run; community long-poll availability was not revalidated [F7] | **Partial:** `messages.send` exists; official errors include deny-send/privacy/reply-timeout conditions. Exact business reply policy requires account test [F7] | **C:** attachments and `audio_message` objects with MP3/OGG links exist [F7] | **C:** `peer_id`, message/conversation IDs, author/community admin IDs [F7] | **Partial:** message objects expose outbound/read-related fields and `was_listened` for audio; a durable delivery receipt was not confirmed [F7] | **Partial:** `admin_author_id` shows a human community admin can be the sender; exclusivity/handoff is application state [F7] | **Partial/U:** access token is confirmed by schema; callback confirmation/secret must be rechecked in the live portal/account | **U:** no current numeric messaging rate limit was verified | Direct community adapter; no website embed required | Tier 2 pending end-to-end community test |
| Jivo Bot API (Staffy as bot provider) | **C:** Bot API is listed for VIP PRO; provider ID is obtained by contacting Jivo [F9] | **C:** HTTPS JSON webhooks initiated by Jivo [F9] | **C:** bot returns `BOT_MESSAGE`; after `CHAT_CLOSED` it cannot send to that chat [F9] | **Partial:** retrieved Bot API explicitly documents text/Markdown/buttons; media behavior was not found and must not be assumed [F9] | **C:** event ID, `client_id`, `chat_id`, `site_id`, channel ID/type [F9] | **U:** no delivery/read events found in Bot API | **C:** `INVITE_AGENT`, `AGENT_UNAVAILABLE`, and automatic operator invitation on bot timeout [F9] | **C:** unique token carried in endpoint URL; HTTP error handling documented. No separate request signature was found [F9] | **C, single publisher:** 3s request timeout, 2 retries, 15s answer timer, hourly block after loop/limit; validate operationally [F9] | Connect once inside Jivo and assign bot to Jivo channels | Tier 1 for existing Jivo customers |
| Jivo Chat API (Staffy/custom chat feeding Jivo inbox) | **C:** asynchronous custom channel for mobile, desktop, or custom website widget [F10] | **C:** bidirectional HTTPS POST endpoints/webhooks [F10] | **C:** operator replies are POSTed to customer endpoint; online status can be queried [F10] | **C:** video, audio, voice, photo, sticker, document, location [F10] | **C:** caller supplies stable `sender.id`; Jivo returns `recipient.id` unchanged; message IDs exist [F10] | **U:** docs say message IDs may be used for delivery/read notifications “in future versions” [F10] | **C:** conversations appear to Jivo operators like other channels [F10] | **Partial:** random endpoint path and `JIVO_PUBLIC_ID`; no cryptographic signature documented [F10] | **C, single publisher:** retry behavior is documented; not independently verified [F10] | Custom Staffy widget/native app -> Staffy server -> Jivo Chat API | Tier 1 where Jivo is the chosen human inbox |
| Avito Messenger | **C:** official Avito business page confirms API for own CRM and ready Jivo integration; access depends on eligible tariff/subscription [F11] | **U in this run:** official developer portal was linked but its technical page could not be fetched; webhook details therefore remain unverified | **U:** send/delete/read capabilities and any initiation/reply constraints require official portal/account test | **U:** official business page does not specify image/voice API behavior | **Partial:** business account credentials are issued in “For professionals”; exact user/chat ID contract unverified [F11] | **U** | **C:** Jivo can be connected through Avito OAuth-style authorization from Jivo; direct shared-inbox behavior unverified [F11] | **Partial:** direct API uses issued `client_id`/`client_secret` per official setup page; webhook verification unverified [F11] | **U** | Prefer Jivo connector for fastest validated customer path; direct adapter only after developer-portal contract test | Tier 2 / gated |
| First-party website chat (Django) | No external eligibility; controlled by Staffy/customer | **P:** same-origin HTTPS bootstrap plus WebSocket/SSE and REST fallback | **P:** no platform reply window; Staffy policy controls proactive prompts and consent | **P:** signed upload URLs, AV scan, codec allowlist; browser recording optional | **P:** tenant-scoped anonymous visitor ID upgraded to authenticated customer ID only after proof | **P:** Staffy can define accepted/delivered/read semantics end-to-end | **P:** Staffy inbox or Jivo Chat API | **P:** signed short-lived visitor token, Origin allowlist, CSRF protections, rate limiting | Staffy-controlled | Add one deferred `<script>` tag in Django base template; do not expose tenant secrets to browser | Tier 0 |
| Hosted builder / ABCP | **C:** ABCP docs allow scripts/custom code in HTML-mode infoblocks/footer [F12] | Same as Staffy website widget (**P**) | Same as Staffy website widget (**P**) | Same as Staffy website widget (**P**) | Same as Staffy website widget (**P**) | Same as Staffy website widget (**P**) | Staffy inbox or Jivo (**P**) | Public site key only in snippet; server issues signed session (**P**) | Staffy-controlled | Insert Staffy loader in footer infoblock via HTML editor; support CSP-safe external script and no inline secret | Tier 0 |
| Android / iOS application chat | App owner controls eligibility | **P:** native HTTPS/WebSocket client to same Staffy conversation API; REST fallback | **P:** Staffy-controlled | **P:** native picker/recorder uploads to Staffy object store | **P:** authenticated app user ID plus tenant; device ID is not a person identity | **P:** explicit client ACK/read event can be supported | **C/P:** Jivo Chat API explicitly covers mobile sources; otherwise Staffy inbox [F10] | **P:** OAuth/session token in OS secure storage; certificate validation; short-lived upload URLs | Staffy-controlled | First release: thin native wrapper over common protocol. Jivo offers a Mobile SDK link, but its concrete Android/iOS contract was not examined in this run [F10] | Tier 1 protocol, Tier 3 packaged SDK |
| WhatsApp Business Platform | **Not verified for Russia API use:** official help confirms the Business app and points businesses to the Platform, but does not establish Russia-based Platform onboarding/operation [F13] | Not evaluated | Not evaluated | Not evaluated | Phone number (app registration); Platform identity contract not evaluated | Not evaluated | Business app is human-operated; API shared inbox not evaluated | Not evaluated | Not evaluated | Do not integrate through QR/session scraping or unofficial libraries | Deferred |

## Recommended architecture spine

### 1. Canonical model and connector contract

Use tenant-scoped canonical entities:

- `ChannelConnection(tenant_id, channel, external_account_id, mode, credentials_ref, status)`
- `Conversation(channel_connection_id, external_conversation_id, state, assigned_actor)`
- `Participant(channel, external_user_id, display_snapshot)`
- `Message(external_message_id, direction, content_parts, reply_to, status, occurred_at)`
- `Attachment(kind, external_locator, stored_object_ref, media_type, size)`
- `Receipt(kind=accepted|delivered|read|listened, source, occurred_at)`
- `Handoff(state=bot_active|handoff_requested|human_active|bot_resume_pending)`

An adapter only translates official channel events into this model and sends canonical outbox commands. Never invent a delivery/read receipt: an HTTP 200 means **accepted by API**, not delivered or read, unless a channel emits a specific receipt.

### 2. Safe webhook ingestion

For every channel:

1. Terminate TLS, validate the platform-specific secret/signature/confirmation where available.
2. Resolve the exact tenant and connection from an unguessable endpoint identifier; do not accept a tenant ID from the untrusted payload as authority.
3. Persist the raw envelope and a deduplication key before returning success.
4. Acknowledge quickly, then normalize and process asynchronously.
5. Use idempotent outbox delivery keyed by `(connection, canonical_message_id, operation)`.
6. Treat retries and duplicate events as normal. MAX explicitly warns about duplicates and has webhook retry/unsubscribe behavior [F6]; Jivo also retries [F9, F10]. These are single-publisher operational facts, so validate them in a canary.

### 3. One writer per conversation

The hardest omnichannel failure is bot and human both replying. Enforce a short renewable **write lease** per conversation. When handoff begins, revoke the bot lease before exposing the conversation to a human. Jivo provides explicit transfer events; direct Telegram/MAX/VK/Avito adapters need the same state machine in Staffy.

For a channel already connected to Jivo, configure Staffy either as:

- a **Jivo Bot API provider**, letting Jivo own the inbox and human assignment; or
- the **direct channel adapter**, letting Staffy own the inbox.

Do not configure a direct adapter and Jivo connector to answer the same external identity.

### 4. Identity and privacy

Keep identity keys channel-local: `(tenant_id, channel, external_user_id)`. Never merge a Telegram/VK/MAX/Avito identity because names or phone-like fields match. Link profiles only after an authenticated action such as signed-in website/app session, order verification, or OTP-based claim.

Store the primary conversation ledger and first collection of Russian citizens’ personal data in Russian-hosted databases. Article 12 of Federal Law 152-FZ requires advance notification before cross-border transfer and allows restrictions/prohibitions; the current law text was retrieved from the official legal-information portal [F14]. The exact hosting/data-flow of each platform was not established in this run, so maintain a per-connector data-flow map and obtain legal review before sending message content or attachments to any foreign processor. This is an implementation caveat, not legal advice.

### 5. Website and mobile delivery

Build the first-party widget once and reuse its protocol:

- Django: loader script in the base template; backend creates a short-lived signed visitor session bound to tenant and allowed origin.
- ABCP/other builders: external loader script in a footer/custom-HTML block. ABCP explicitly permits such script placement [F12].
- Android/iOS: native UI calls the same REST/WebSocket API. A WebView can be a temporary packaging option, but a native client avoids browser-cookie and bridge complexity. This is a Staffy design proposal, not a channel-vendor capability claim.
- Jivo customers: route the widget/app through Jivo Chat API if Jivo must remain the human inbox [F10].

## Rollout tiers and exit gates

### Tier 0 — channel core and first-party web chat

Ship canonical event schema, raw-event ledger, dedupe, outbox, attachment pipeline, receipt semantics, write lease, handoff state machine, and Staffy widget. Validate Django and ABCP snippet installation.

**Exit gate:** duplicate webhook replay produces one message; failed outbound operations retry without duplicate sends; human takeover prevents bot output.

### Tier 1 — Telegram, MAX, Jivo, mobile protocol

- Telegram direct adapter using webhooks and secret header; test standard bot and Business Connection as separate products.
- MAX direct adapter using HTTPS:443 webhook, secret header, and conservative internal throttles.
- Jivo Bot API for existing Jivo accounts; Jivo Chat API where the customer wants Staffy’s custom client but Jivo’s operator inbox.
- Android/iOS use the common API; defer a public SDK until the protocol has stabilized.

**Exit gate:** account onboarding, token rotation, media round-trip, bot pause/handoff, reply restrictions, and deletion behavior pass in real test accounts. For MAX, repeat this gate after any API changelog item affecting domains, certificates, chat discovery, or membership.

### Tier 2 — VK and Avito

- VK: confirm community permissions, callback confirmation/secret, actual message/read events, media upload flow, reply timeout, and current numeric limits in a test community.
- Avito: first validate the business tariff, developer credentials, webhook verification, chat IDs, text/image/voice behavior, read marking, and automation rules in the official developer portal and a sandbox/test account. Until then, the Jivo-to-Avito connector is the lower-effort supported path for Jivo customers [F11].

### Deferred — WhatsApp Business Platform

Do not ship an API integration until Meta or an authorized BSP provides written confirmation of onboarding eligibility for the Russia-based legal entity/number and a real number-registration/webhook test succeeds. The WhatsApp Business app’s availability is not evidence that Platform API onboarding is available [F13].

## Evidence records

### Telegram

**F1 — Claim:** Telegram Bot API supports mutually exclusive long polling and HTTPS webhooks; webhook requests can carry `X-Telegram-Bot-Api-Secret-Token`; updates are retained no longer than 24 hours; user/chat/message and Business Connection IDs are defined; `readBusinessMessage` can mark an incoming business message read.  
**Source URL:** https://core.telegram.org/bots/api  
**Publisher:** Telegram  
**Publication/update date:** rolling documentation, latest material indexed 2026-08 (exact page-update day not stated)  
**Accessed:** 2026-09-12  
**Confidence:** medium (fresh official source; no independent publisher)  
**Class:** compatibility / protocol / identity / receipt / auth

**F2 — Claim:** Telegram’s official bot introduction says bots cannot start conversations; a user must first message the bot or add it to a group.  
**Source URL:** https://core.telegram.org/bots  
**Publisher:** Telegram  
**Publication/update date:** rolling documentation; no explicit date  
**Accessed:** 2026-09-12  
**Confidence:** medium  
**Class:** eligibility / outbound constraint

**F3 — Claim:** Telegram Bot Features documents files and voice as bot inputs, Business Connection updates, `can_reply`, a 24-hour recent-incoming-message condition for eligible business chats, and business chat management links.  
**Source URL:** https://core.telegram.org/bots/features  
**Publisher:** Telegram  
**Publication/update date:** rolling documentation; current 2026 feature set  
**Accessed:** 2026-09-12  
**Confidence:** medium  
**Class:** compatibility / media / outbound constraint / handoff

### MAX

**F5 — Claim:** Current MAX API docs require `platform-api2.max.ru`, authorization by header token, recommend webhooks for production, document `user_id`/`chat_id`, allow image/video/audio/file attachments, support mini-app launch buttons, and document 30 rps overall plus no more than two messages per second per dialog.  
**Source URL:** https://dev.max.ru/docs-api  
**Publisher:** MAX  
**Publication/update date:** rolling documentation, current 2026-09; page includes changes effective 2026-09-09 and 2026-09-30  
**Accessed:** 2026-09-12  
**Confidence:** medium; numeric limits unverified beyond the publisher  
**Class:** compatibility / auth / protocol / media / identity / rate limit

**F6 — Claim:** MAX’s webhook subscription method requires HTTPS on port 443 and a trusted certificate, supports an optional secret in `X-Max-Bot-Api-Secret`, requires a 200 response within 30 seconds, retries failed deliveries, and can unsubscribe a bot after prolonged failure; the event model includes message create/edit/remove but no read/delivery event in the retrieved list.  
**Source URL:** https://dev.max.ru/docs-api/methods/POST/subscriptions  
**Publisher:** MAX  
**Publication/update date:** rolling documentation, current 2026-09  
**Accessed:** 2026-09-12  
**Confidence:** medium; retry/failure timing is unverified beyond the publisher  
**Class:** compatibility / webhook / auth / failure behavior / receipt gap

### VK

**F7 — Claim:** VK’s official schema is compatible with API 5.199 and defines `messages.send`, attachments, peer/conversation/message IDs, community-admin sender identity, audio-message MP3/OGG links, read/listened-related fields, and message-policy errors including deny-send/privacy/reply-timeout conditions.  
**Source URL:** https://github.com/VKCOM/vk-api-schema  
**Publisher:** VKCOM  
**Publication/update date:** rolling `master`; API schema version 5.199; exact update date not stated  
**Accessed:** 2026-09-12  
**Confidence:** medium (official repository, but portal/schema mismatches have historically been reported; account test required)  
**Class:** compatibility / identity / media / outbound constraint / receipt

### Jivo

**F9 — Claim:** Jivo Bot API is a webhook-based bot-provider integration; it uses a unique token in endpoint URLs, defines client/chat/channel IDs, supports text/Markdown/buttons, offers explicit `INVITE_AGENT`, emits `CHAT_CLOSED`, and documents timeout/retry/limit behavior. Provider onboarding requires contacting Jivo and the page lists VIP PRO.  
**Source URL:** https://www.jivo.ru/help/api/bot-api.html  
**Publisher:** Jivo (ООО «Живой Сайт»)  
**Publication/update date:** rolling documentation; page footer 2026, exact update date not stated  
**Accessed:** 2026-09-12  
**Confidence:** medium; operational timing/limit is single-publisher and needs staging verification  
**Class:** eligibility / protocol / auth / handoff / outbound constraint / rate limit

**F10 — Claim:** Jivo Chat API is an asynchronous bidirectional HTTPS integration for custom website, mobile, and desktop clients; callers supply stable user/message IDs, operator replies return that user ID, multimedia includes audio and voice, and delivery/read notification use is explicitly described as future rather than current.  
**Source URL:** https://www.jivo.ru/help/api/chat-api.html  
**Publisher:** Jivo (ООО «Живой Сайт»)  
**Publication/update date:** rolling documentation; page footer 2026, exact update date not stated  
**Accessed:** 2026-09-12  
**Confidence:** medium  
**Class:** protocol / embedding / identity / media / receipt gap / shared inbox

### Avito, web builders, WhatsApp, compliance

**F11 — Claim:** Avito’s official business page confirms Messenger API access for a customer’s own CRM, says it depends on an eligible tariff/subscription, and documents a ready Jivo connection flow plus direct `client_id`/`client_secret` issuance. The technical developer page did not render in the research tool, so webhook/media/read semantics remain unverified here.  
**Source URL:** https://avito.akamaized.net/business/tools/messenger  
**Publisher:** Avito  
**Publication/update date:** rolling business documentation; exact update date not stated  
**Accessed:** 2026-09-12  
**Confidence:** medium for access paths; low/unverified for omitted technical behavior  
**Class:** eligibility / integration path / auth / evidence gap

**F12 — Claim:** ABCP documentation permits custom scripts and other generated code in HTML-mode infoblocks, including footer blocks, which is sufficient to install an external Staffy loader snippet.  
**Source URL:** https://www.abcp.ru/d/docs/website/faq-interface-and-content  
**Publisher:** ABCP  
**Publication/update date:** rolling documentation, retrieved current 2026-09-12  
**Accessed:** 2026-09-12  
**Confidence:** medium  
**Class:** embedding / compatibility

**F13 — Claim:** WhatsApp’s official help confirms the WhatsApp Business app can be downloaded and registered and points larger businesses toward the WhatsApp Business Platform, but this page does not establish Russia-specific Platform API eligibility or operation.  
**Source URL:** https://faq.whatsapp.com/665643701880397/?cms_platform=android&locale=ru_RU  
**Publisher:** WhatsApp  
**Publication/update date:** rolling help article; current as retrieved 2026-09-10/12, exact publication date not stated  
**Accessed:** 2026-09-12  
**Confidence:** medium for app availability; **low/unverified** for Russia Platform API eligibility  
**Class:** eligibility gap / landscape

**F14 — Claim:** The current official text of Federal Law 152-FZ states that an operator must notify the competent authority before cross-border transfer and describes circumstances in which transfer can be restricted or prohibited.  
**Source URL:** https://ips.pravo.gov.ru/api/ips/legislation/document?baseid=None&hash=98490812b3409e2a8d78a11ca9010f434ea3d9250a11dbbdb78690cd5551bdd6  
**Publisher:** Official Internet Portal of Legal Information, Russian Federation  
**Publication/update date:** Federal Law dated 2006-07-27; retrieved consolidated text dated 2025-09-01  
**Accessed:** 2026-09-12  
**Confidence:** high for the quoted statutory requirement; application to a specific Staffy flow still needs counsel  
**Class:** regulatory / data transfer

## Gaps and next verification work

1. **Avito developer contract:** retrieve the official technical portal in a browser/account session and record webhook signature/secret, subscription scopes, message/media formats, voice handling, read events, reply rules, numeric limits, and commercial eligibility by category/tariff.
2. **VK portal/account test:** validate Callback API and Community Long Poll against current portal UI, webhook secret behavior, reply-timeout rules, media uploads, receipts, and live rate limits. The official schema is current but not a substitute for an end-to-end community test.
3. **MAX handoff and receipts:** confirm whether any business/shared-inbox or read/delivery capability exists outside the retrieved bot API pages; verify audio versus true voice-note rendering.
4. **Telegram Business operations:** test pause/resume and simultaneous human use, rights changes, 24-hour eligibility behavior, and ordinary-bot versus Business Connection product boundaries.
5. **Jivo media through Bot API:** Chat API media is documented, but Bot API response media was not found. Validate what inbound media reaches a bot provider and whether the bot can return it.
6. **Data residency:** obtain each vendor’s current processing-location/DPA materials. The channel API docs do not establish where payloads, attachments, backups, or support logs are stored.
7. **Mobile SDK:** choose native versus WebView after threat modeling; document push-notification delivery and token lifecycle separately. This run verified only that Jivo Chat API supports mobile-origin conversations, not a complete Android/iOS Staffy SDK design.
8. **WhatsApp:** only revisit after official Meta/BSP confirmation specifically covering the Russia-based entity and phone number plus a successful production-like webhook test. App availability alone is insufficient.

## Bottom line

Build one Staffy conversation protocol, not one “universal” third-party connector. Use official channel APIs directly where current contracts are clear, and use Jivo when the customer values its existing operator inbox and explicit bot-to-human transfer. Treat receipts as unknown unless explicitly emitted, keep identity channel-local, persist before acknowledging webhooks, and enforce one active writer. Ship web/Telegram/MAX/Jivo first; gate VK and Avito on real-account tests; defer WhatsApp Business Platform.
