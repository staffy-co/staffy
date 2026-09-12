# Go/Pion media-core decision: technical scope audit (round 1)

**Decision:** build an in-house Go SIP/WebRTC/media core now, or adopt telephony/media infrastructure in stages.

**Method:** technical deep recon, selection shape, high validation. The imported chat was read only as a hypothesis inventory and is not evidence. This round uses eight current primary sources. Performance, scale, delivery-time, and failure assertions are treated as unverified unless independently supported.

**Access date for all web evidence:** 2026-09-12.

## Executive verdict

**Do not build a complete Go SIP/WebRTC/media core now. Use staged adoption.** Pion WebRTC is a strong Go implementation of WebRTC primitives, but it does **not** replace a SIP server, PSTN gateway, PBX, SBC, media transcoder, recorder, or highly available session-control plane. The official Pion SIP-to-WebRTC example itself adds a separate SIP library and G.711 handling, demonstrating the missing boundary rather than closing it.

`go-ari` strengthens the case for retaining Asterisk: it is a Go **client for Asterisk's ARI**, not a Go implementation of Asterisk. A design described as “Pion + go-ari” still has Asterisk in the data/control path and therefore is not an own-core replacement.

Recommended staged shape:

1. Use **Asterisk + ARI External Media** as the first PSTN/SIP edge when PBX-grade call control and carrier interoperability are primary. Keep the differentiating AI/session orchestration in Go.
2. Use **self-hosted LiveKit + LiveKit SIP** instead when a unified WebRTC-room model is the hard requirement and target-carrier acceptance tests pass. Do not place Asterisk and LiveKit SIP in tandem by default; that adds another stateful media/signaling hop without evidence that it solves a requirement.
3. Use **Pion selectively** for browser/WebRTC endpoints or narrowly scoped custom media functions. Do not treat Pion as the SIP/PSTN core.
4. Reconsider replacing the adopted edge only after production measurements identify a concrete limitation that cannot be solved through configuration, contribution, or a narrow adapter.

**Verdict on the specific question:** Pion does **not** replace Asterisk or another SIP server. It can replace a WebRTC endpoint/peer implementation and can be used to build a custom gateway, but the gateway, SIP stack, call-control semantics, codecs/transcoding, operations, recording, and HA remain work owned by the team.

## Decision matrix

| Criterion | Own Go core using Pion | Asterisk + Go/ARI | Self-hosted LiveKit + SIP | Decision implication |
|---|---|---|---|---|
| Browser WebRTC | WebRTC primitives are present; application signaling, session service, TURN deployment, observability, and lifecycle remain to build | Possible, but not the cleanest AI/WebRTC abstraction | Native room/participant model; SIP is bridged into rooms | LiveKit or selective Pion is stronger for WebRTC-first use |
| SIP signaling / PSTN trunking | **Not supplied by Pion**; add/build SIP UA/proxy/B2BUA behavior and carrier edge | Supplied by Asterisk/PJSIP; ARI controls calls | Supplied by a separate LiveKit SIP service | Do not call Pion a SIP server |
| RTP/RTCP and secure WebRTC media | Direct RTP/RTCP, ICE, DTLS/SRTP are documented | Asterisk exposes RTP and other External Media transports | LiveKit handles room media; SIP service exposes SIP/RTP ports | All three can move media, at very different abstraction levels |
| Codec handling | Pion documents packetizers and external integration; PSTN codec translation/mixing remains application work | External Media requires an explicit audio format; Asterisk owns channel media behavior | SIP bridge is a separate media component; exact target-carrier codec compatibility still needs an acceptance matrix | The chat omitted a codec/transcoding plan |
| DTMF | Must design WebRTC vs SIP representations and interworking | ARI can send DTMF; Asterisk emits/handles channel events | LiveKit documents send/receive DTMF | Established stacks remove substantial edge-case work |
| Hold / transfer | Must implement re-INVITE/REFER/NOTIFY/bridge state machines | Supported, but an ARI-only application must actively handle SIP REFER flows | LiveKit documents forwarding and agent-assisted transfers | “Transfer” is a call-control workflow, not a media primitive |
| Recording | Must design tap point, mixed vs dual track, gaps, storage and lifecycle | ARI exposes recording operations | Not documented as part of the SIP service itself; plan and validate a separate recording path before selection | Recording is a component, not a checkbox |
| NAT / TURN | ICE/STUN/TURN connectivity is present, but a reachable TURN relay and signaling service still must be operated | SIP/RTP NAT behavior is handled at the telephony edge, outside Pion | SIP service requires public signaling/media reachability; WebRTC and distributed services remain separate operational concerns | SIP NAT and WebRTC ICE/TURN are different problems |
| HA / failure behavior | Entire design is team-owned: admission, affinity, draining, state recovery, call teardown, replay/idempotency | Multi-node routing can be orchestrated, but `go-ari` does not claim active-call migration | Multi-node routing and connection draining are documented; abrupt loss of a room-hosting node remains disruptive in the cited docs | No option proves seamless mid-call failover |
| Delivery risk | Highest and largely unbounded until interoperability suite exists | Lowest for SIP/PSTN-first launch | Lower for unified WebRTC/SIP, subject to provider compatibility and operational validation | Staged adoption preserves reversibility |

## Evidence claims

### C1 — What Pion implements

- **claim:** Pion WebRTC is a pure-Go implementation of the WebRTC API. Its documented surface includes PeerConnection, audio/video send and receive, ICE restart, trickle ICE, STUN/TURN connectivity, direct RTP/RTCP access, packetizers, and DTLS/SRTP. It is a WebRTC building block, not a packaged communications platform.
- **source_url:** https://raw.githubusercontent.com/pion/webrtc/master/README.md
- **publisher:** Pion community
- **pub_date:** undated live repository documentation (current branch snapshot)
- **accessed:** 2026-09-12
- **confidence:** high
- **class:** capability / version-compatibility

### C2 — “TURN support” is not a deployed relay service

- **claim:** Pion's WebRTC API can use TURN as part of ICE connectivity, but the cited Pion component does not by itself establish the operational TURN service, credentials, public reachability, capacity, abuse controls, or HA. Treat TURN as a separately deployed and operated dependency.
- **source_url:** https://raw.githubusercontent.com/pion/webrtc/master/README.md
- **publisher:** Pion community
- **pub_date:** undated live repository documentation (current branch snapshot)
- **accessed:** 2026-09-12
- **confidence:** medium; the separation is an architectural inference from the API/library scope
- **class:** integration / operational gap

### C3 — Pion does not supply SIP/PSTN signaling

- **claim:** Pion WebRTC does not itself provide the SIP/PSTN server assumed in the chat. Pion's official SIP-to-WebRTC example accepts SIP by adding the separate `emiago/sipgo` library.
- **source_url:** https://github.com/pion/example-webrtc-applications/tree/main/sip-to-webrtc
- **publisher:** Pion community
- **pub_date:** undated live repository example (page current in 2026)
- **accessed:** 2026-09-12
- **confidence:** high
- **class:** capability boundary

### C4 — Packet transport is not codec/transcoding completeness

- **claim:** The Pion core documents packetizers and integrations with external media tools; its official SIP bridge example separately calls for G.711 encode/decode and explicitly says multi-party calls require audio mixing. Therefore a production PSTN bridge still needs a codec/resampling/transcoding and mixing plan.
- **source_url:** https://raw.githubusercontent.com/pion/webrtc/master/README.md ; https://github.com/pion/example-webrtc-applications/tree/main/sip-to-webrtc
- **publisher:** Pion community
- **pub_date:** undated live repository documentation/examples
- **accessed:** 2026-09-12
- **confidence:** high
- **class:** media / compatibility

### C5 — What `go-ari` implies

- **claim:** `CyCoreSystems/ari` is a Go client for Asterisk REST Interface. It connects to Asterisk over ARI HTTP and WebSocket endpoints and offers wrappers for tasks such as playback, IVR, and recording. It does not replace Asterisk's SIP, channel, bridge, or media implementation.
- **source_url:** https://github.com/cycoresystems/ari
- **publisher:** CyCoreSystems
- **pub_date:** undated live repository documentation
- **accessed:** 2026-09-12
- **confidence:** high
- **class:** integration / capability boundary

### C6 — `go-ari` does not prove a clean current-version story

- **claim:** The repository README contains internally inconsistent version guidance (`v5.x.x` described as current while the installation example imports `/v6`) and says major-version APIs change. Pin and test the exact module/Asterisk combination; do not accept “use go-ari” as a complete compatibility decision.
- **source_url:** https://github.com/cycoresystems/ari
- **publisher:** CyCoreSystems
- **pub_date:** undated live repository documentation
- **accessed:** 2026-09-12
- **confidence:** high for the documentation inconsistency; compatibility outcome remains unverified until tested
- **class:** version-compatibility / ecosystem risk

### C7 — Asterisk already exposes the AI-media seam

- **claim:** Asterisk ARI exposes call/channel operations including originate/dial, answer, hold, DTMF, recording, RTP statistics, and External Media. External Media supports UDP/RTP, TCP/AudioSocket, and WebSocket transports with a required audio format. A custom Go AI worker can therefore consume/produce media without reimplementing the carrier-facing SIP core.
- **source_url:** https://docs.asterisk.org/Latest_API/API_Documentation/Asterisk_REST_Interface/Channels_REST_API/
- **publisher:** Asterisk Project documentation
- **pub_date:** live “Latest API” documentation; generated version not stated in the retrieved excerpt
- **accessed:** 2026-09-12
- **confidence:** high
- **class:** capability / integration

### C8 — Asterisk transfer support still requires application logic under ARI

- **claim:** In an ARI-only environment, attended and blind SIP transfers are not a one-call API abstraction. On SIP REFER, the ARI application must act, send/coordinate transfer progress, originate the destination leg, and manipulate bridges. Asterisk removes protocol/media work but not the product-specific transfer workflow.
- **source_url:** https://docs.asterisk.org/Configuration/Interfaces/Asterisk-REST-Interface-ARI/Introduction-to-ARI-Transfer-Handling/
- **publisher:** Asterisk Project documentation
- **pub_date:** undated live documentation
- **accessed:** 2026-09-12
- **confidence:** high
- **class:** call control / implementation reality

### C9 — LiveKit SIP is a separate service, not Pion hidden inside an SDK

- **claim:** Self-hosted LiveKit telephony requires a separately deployed LiveKit SIP service. The service handles inbound/outbound SIP, digest authentication, DTMF, SIP trunks/dispatch, and RTP, communicates with LiveKit through Redis, and requires public SIP and RTP reachability.
- **source_url:** https://raw.githubusercontent.com/livekit/sip/main/README.md
- **publisher:** LiveKit
- **pub_date:** undated live repository documentation (current branch snapshot)
- **accessed:** 2026-09-12
- **confidence:** high
- **class:** architecture / operational dependency

### C10 — LiveKit's current telephony surface is broader than the repository README alone

- **claim:** Current LiveKit telephony documentation lists send/receive DTMF, call forwarding and agent-assisted transfers, HD voice, and TLS/SRTP secure trunking. These are product-level features that would otherwise have to be designed and tested in an own Go core.
- **source_url:** https://docs.livekit.io/telephony/features/
- **publisher:** LiveKit
- **pub_date:** undated live documentation
- **accessed:** 2026-09-12
- **confidence:** high for documented capability; target-provider compatibility remains unverified
- **class:** capability / compatibility

### C11 — LiveKit HA is routing plus draining, not proven transparent call migration

- **claim:** LiveKit documents Redis-backed multi-node routing and native connection draining. It also states that terminating an instance hosting active sessions is extremely disruptive. The cited material does not establish seamless migration of an active room/call after abrupt node failure.
- **source_url:** https://docs.livekit.io/transport/self-hosting/distributed/
- **publisher:** LiveKit
- **pub_date:** undated live documentation
- **accessed:** 2026-09-12
- **confidence:** medium; single primary source, so the abrupt-failure conclusion is **unverified** under the high-validation two-source rule
- **class:** failure / HA

### C12 — Multi-node addressing in `go-ari` is not active-call failover

- **claim:** `go-ari` has resource keys that identify which Asterisk node owns a resource and mentions proxy-based routing across instances. This helps multi-node control-plane routing, but the cited documentation does not claim live channel/media-state replication or active-call migration.
- **source_url:** https://github.com/cycoresystems/ari
- **publisher:** CyCoreSystems
- **pub_date:** undated live repository documentation
- **accessed:** 2026-09-12
- **confidence:** medium; absence-of-guarantee inference
- **class:** failure / HA

### C13 — The chat's performance and scale claims are unsupported

- **claim:** Claims that Go is “hundreds of times” faster than Python for this system, that Pion/Go yields sub-millisecond media handling, that a LiveKit node or cluster supports a stated number of calls, or that a Pipecat worker holds only “a few” calls were not verified by two independent production measurements. They must not drive the architecture decision.
- **source_url:** no qualifying external evidence found in the admitted round-1 source set; the imported chat is hypothesis-only
- **publisher:** n/a
- **pub_date:** n/a
- **accessed:** 2026-09-12
- **confidence:** low; **unverified**
- **class:** performance / scale

### C14 — The chat's delivery estimates are unsupported

- **claim:** “2–3 months,” “2.5–3× longer,” “3–4× longer,” and team-size estimates are not evidence-backed. The omitted telephony components make the scope too incomplete for such precision.
- **source_url:** no qualifying external evidence found in the admitted round-1 source set; the imported chat is hypothesis-only
- **publisher:** n/a
- **pub_date:** n/a
- **accessed:** 2026-09-12
- **confidence:** low; **unverified**
- **class:** delivery / cost

### C15 — “Three-minute SIP trunk onboarding” is not established

- **claim:** A carrier connection is not reduced to host, login, and password alone. Public signaling/media reachability, authentication mode, number normalization, codec overlap, early media, DTMF mode, transfer behavior, TLS/SRTP expectations, NAT, and provider policy all need validation. No admitted source supports a universal three-minute onboarding claim.
- **source_url:** https://raw.githubusercontent.com/livekit/sip/main/README.md
- **publisher:** LiveKit
- **pub_date:** undated live repository documentation
- **accessed:** 2026-09-12
- **confidence:** medium for the required configuration categories; the universal duration is **overturned as unsupported**, not numerically disproved
- **class:** interoperability / delivery

### C16 — Selection conclusion

- **claim:** The evidence favors staged adoption because Asterisk and LiveKit SIP already expose the telephony/media seams needed by a Go AI orchestration layer, while Pion covers only the WebRTC portion and leaves the largest SIP/PSTN risks in team-owned code. An own core becomes rational only when a measured, strategically important requirement cannot be met by either adopted path.
- **source_url:** https://raw.githubusercontent.com/pion/webrtc/master/README.md ; https://docs.asterisk.org/Latest_API/API_Documentation/Asterisk_REST_Interface/Channels_REST_API/ ; https://raw.githubusercontent.com/livekit/sip/main/README.md
- **publisher:** Pion community; Asterisk Project; LiveKit
- **pub_date:** live documentation/repository snapshots
- **accessed:** 2026-09-12
- **confidence:** high for scope comparison; medium for the final business choice because project-specific volumes, carrier matrix, and team costs were intentionally outside the evidence firewall
- **class:** selection / recommendation

## Required components if the team nevertheless builds its own Go core

The chat's “Pion + go-ari” proposal is technically incomplete until it assigns an implementation and owner for every row below.

| Component | Minimum production scope | What Pion contributes | What remains |
|---|---|---|---|
| SIP signaling | INVITE/ACK/BYE/CANCEL, provisional responses and early media, transaction/dialog timers, digest auth, REGISTER if required, SDP offer/answer, re-INVITE/UPDATE, REFER/NOTIFY, error/retry semantics | Nothing documented in Pion WebRTC | SIP library/server selection, B2BUA/proxy semantics, interop suite, security and abuse controls |
| PSTN/carrier edge | Trunks, DID routing, caller identity, number normalization, provider ACL/auth, rate limits, cause mapping, billing/CDR, legal intercept obligations where applicable | Nothing | Carrier-specific configuration and acceptance tests; often an SBC/proxy boundary |
| RTP/RTCP media | Port allocation, SSRC/payload mapping, sequence/timestamp continuity, jitter buffer, packet-loss behavior, RTCP feedback, symmetric RTP/NAT behavior | WebRTC RTP/RTCP primitives | SIP RTP profile, clocking, buffering, recovery, telemetry, safe port lifecycle |
| WebRTC signaling | Offer/answer exchange, authentication, room/session admission, reconnect/restart, browser compatibility | PeerConnection/ICE/media primitives | Application signaling service, authorization, persistence, lifecycle and abuse protection |
| ICE/STUN/TURN | Candidate policy, relay credentials, public reachability, UDP/TCP/TLS fallback, quotas, monitoring | ICE/STUN/TURN protocol support in the client/peer library | Deploy and operate TURN; capacity, credential rotation, anti-abuse and HA |
| Codecs/transcoding | At least browser Opus and trunk G.711 variants; likely resampling, frame sizing, PLC, gain, mono/stereo rules; other provider codecs only when required | Packetizers/direct media access | Decode/encode/transcode implementation, licensing review, CPU budgets, quality tests |
| DTMF | WebRTC DTMF, RTP telephone events, SIP INFO and in-band detection where required; deduplication and duration semantics | No SIP interworking layer | Mode negotiation, conversion, events, tests across providers and IVRs |
| Call control | Answer, reject, hold/resume, mute, bridge, consult, blind/attended transfer, escalation to human, hangup causes | Peer connection state only | Telephony state machine, SIP messages, bridge topology, idempotency and race handling |
| Recording | Mixed vs dual-channel, start point, pause/redaction, continuity across transfer, metadata, encryption, retention, upload retry | Low-level media access only | Recorder, muxing/encoding, storage pipeline, failure recovery, audit and policy controls |
| HA / upgrades | Admission routing, session affinity, graceful drain, health, restart policy, multi-node ownership, state recovery, configuration rollout, abrupt-node-loss behavior | Nothing at platform level | Distributed control plane and explicit failure contract; seamless active-call migration must not be assumed |
| Operations | Metrics, tracing by call ID, SIP/RTP packet capture, CDR/CEL equivalent, alarms, QoS/RTP statistics, load shedding, chaos and soak tests | Some protocol statistics/primitives | Full SRE surface and runbooks |
| Security | TLS/SRTP policy, certificate rotation, SIP scanning/fraud controls, TURN abuse, tenant isolation, secret handling, patch process | DTLS/SRTP primitives for WebRTC | Carrier-facing security architecture and operational response |

## Contradictions and gaps in the imported proposal

| Hypothesis from the chat | Audit result |
|---|---|
| “Pion WebRTC + go-ari” is an own Go media/SIP core | **Contradicted.** Pion is WebRTC; `go-ari` is a client for Asterisk. The combination either still depends on Asterisk or must add a separate SIP implementation. |
| The new Go media server “supports native SIP” | **Contradicted as stated.** No named component in the proposal supplies full SIP/PSTN behavior. The official Pion example adds `sipgo`. |
| Pion gives “monolithic control over audio packets,” removing Asterisk/LiveKit | **Misleading.** Packet access does not provide SIP dialogs, carrier interop, transcoding, DTMF interworking, transfer workflows, recording, or HA. |
| A full own core is only a 2–3 month MVP | **Unverified and scope-incomplete.** The estimate omits major workstreams and an interoperability test matrix. |
| Go avoids future rewrites caused by Python/Pipecat limits | **Unverified.** No two-source production measurements support the asserted bottleneck or rewrite threshold. |
| SIP trunk setup takes three minutes | **Unsupported generalization.** A happy-path credential form is not carrier acceptance. |
| LiveKit “will switch” the user/AI worker on failure | **Not established.** The docs support routing and graceful drain, while abrupt loss of an active host is described as disruptive. |
| Recording is naturally covered by the media core | **Gap.** The proposal does not choose a tap point, channel layout, continuity model, format, storage, retry, or compliance lifecycle. |
| TURN is covered because Pion lists TURN | **Gap.** Protocol support is not a deployed relay service. |
| Asterisk and LiveKit should both be in the mandatory path | **Not justified.** Use parallel adapters or choose by channel/tenant requirement; cascading both should require a measured need. |

## Staged architecture recommendation

### Stage 1 — ship the differentiated layer, adopt the undifferentiated edge

- Define internal interfaces such as `TelephonySession`, `MediaStream`, `TransferController`, `Recorder`, and `CallEvents` so the AI logic does not depend directly on ARI or LiveKit room objects.
- For PSTN-first delivery, terminate SIP in Asterisk and use ARI External Media into the Go voice runtime. Keep transfer and escalation policies in the Go application while using Asterisk's channel/bridge/media machinery.
- For browser WebRTC, use LiveKit or a narrow Pion endpoint adapter. Do not make a custom SFU/session platform part of the first telephony milestone.
- Make recording a first-class pipeline with explicit mixed/dual-track choice and continuity across hold/transfer.

### Stage 2 — test LiveKit SIP as an alternative, not an extra mandatory hop

- Run a provider matrix against actual target SIP trunks: inbound/outbound, authentication, early media, codec negotiation, DTMF, caller ID, cancel races, timeouts, TLS/SRTP, blind transfer, attended transfer, long calls, packet loss, NAT, and reconnect behavior.
- Validate the self-hosted topology: LiveKit server, SIP service, Redis, TURN, recording/export path, load balancer, monitoring, graceful drain, backup/restore, upgrade compatibility, and abrupt-node-loss behavior.
- If the target use case is naturally room-centric and the matrix passes, LiveKit SIP can replace Asterisk for that route. If deep PBX/queue/legacy interoperability dominates, retain Asterisk.

### Stage 3 — replace only a measured bottleneck

- Build a custom Pion-based component only where profiling or product requirements identify a narrow advantage: unusual media transformation, browser transport control, specialized codec path, embedded endpoint, or compliance boundary.
- A full SIP/PSTN replacement should require a written business case, full protocol/interop specification, conformance suite, operational ownership, and an exit comparison against continuing to extend Asterisk or LiveKit SIP.

## Strongest counterargument and response

**Counterargument:** a purpose-built Go core could eliminate extra hops, unify observability, tightly control latency, and avoid dependency constraints.

**Response:** that is plausible but currently unverified. The imported proposal offers no two-source measurements showing Asterisk or LiveKit is the load-bearing latency/scale bottleneck, and it does not budget the SIP/PSTN correctness surface. The reversible move is to own the Go orchestration and media-processing contracts now, instrument every stage, and replace only the component proven to constrain the product.

## Open validation gaps

1. Target-carrier interoperability in the intended geography, for both Asterisk and LiveKit SIP.
2. Exact codec list and whether transcoding is required for each carrier/browser/STT/TTS path.
3. Required transfer semantics: REFER passthrough, blind, attended/consultative, queue handoff, and recording continuity.
4. Recording policy: mixed vs dual track, retention, redaction, encryption, tenant export, and audit.
5. Availability target: acceptable outcome for abrupt media-node loss; no admitted evidence proves mid-call migration for any candidate.
6. Measured concurrency and latency using the actual STT/LLM/TTS stack. All numeric scale claims remain unverified.
7. Maintenance/compatibility review for the exact `go-ari` module version and chosen Asterisk release.

## Round-1 source register (8 primary sources)

1. Pion WebRTC README — https://raw.githubusercontent.com/pion/webrtc/master/README.md
2. Pion SIP-to-WebRTC example — https://github.com/pion/example-webrtc-applications/tree/main/sip-to-webrtc
3. CyCoreSystems Go ARI client — https://github.com/cycoresystems/ari
4. Asterisk ARI Channels API — https://docs.asterisk.org/Latest_API/API_Documentation/Asterisk_REST_Interface/Channels_REST_API/
5. Asterisk ARI transfer handling — https://docs.asterisk.org/Configuration/Interfaces/Asterisk-REST-Interface-ARI/Introduction-to-ARI-Transfer-Handling/
6. LiveKit SIP service README — https://raw.githubusercontent.com/livekit/sip/main/README.md
7. LiveKit telephony features — https://docs.livekit.io/telephony/features/
8. LiveKit distributed/multi-region self-hosting — https://docs.livekit.io/transport/self-hosting/distributed/

