# Verification: Go/Pion media-core decision digest

**Artifact verified:** `go-decision-media-scope-r1.md`  
**Verification scope:** semantic support for C1-C16, with high-validation attention to capability/version claims and all failure, HA, performance, scale, and delivery assertions.  
**Research firewall:** no project evidence was admitted beyond the named digest. Checks used only current public primary documentation, source repositories, and official package metadata.  
**Verification date:** 2026-09-11.

## Result

The digest's central technical boundary survives verification: **Pion WebRTC is a WebRTC implementation, not a SIP/PSTN server; the official bridge example imports `emiago/sipgo`; and `CyCoreSystems/ari` is a Go client for an Asterisk ARI service, not an Asterisk replacement.** Asterisk ARI and External Media also support the claimed control/media seam.

The digest is suitable for synthesis after four wording/status corrections:

1. Do not say the official Pion example “adds G.711 handling.” Its code negotiates PCMU and forwards RTP; its README **points to** `zaf/g711` decoding/encoding when mixing is required and says multiparty calls need mixing.
2. Do not generalize LiveKit server room routing/draining documentation to the separately deployed LiveKit SIP service. The cited page does not establish SIP-service HA or active SIP-call migration.
3. Do not label the “three-minute onboarding” proposition **overturned** merely because it is unsupported. Its correct status is **unverified**; unsupported is not contradicted.
4. Present staged adoption as the conservative evidence-supported default, not as a universally proven optimum. Carrier interoperability, real load, availability requirements, and cost remain decision inputs outside the admitted evidence.

Status tally: **10 verified; 5 verified with wording/status correction; 1 unverified as written; 0 disputed; 0 overturned.**

## Claim-by-claim verification

| Claim | Status | Semantic-support finding | Required correction or guardrail |
|---|---|---|---|
| **C1** | **Verified** | Pion's README calls it a pure-Go implementation of the WebRTC API and lists PeerConnection, send/receive audio/video, ICE restart, trickle ICE, STUN/TURN, direct RTP/RTCP access, packetizers, DTLS, and SRTP. | “Not a packaged communications platform” is a scope inference. Prefer: “The documented `pion/webrtc` surface is a WebRTC library; it does not document packaged SIP/PSTN, PBX, SBC, recording, or HA services.” |
| **C2** | **Verified with wording correction** | The README documents TURN connectivity in the WebRTC library. It does not document provisioning relay hosts, credentials, capacity, abuse controls, or HA. The Pion organization separately publishes `pion/turn`, itself an API for building TURN clients/servers. | Say: “`pion/webrtc` can use TURN, but a TURN service must be provisioned and operated separately, whether built with `pion/turn` or supplied by another implementation.” Do not imply the wider Pion project has no TURN-server code. |
| **C3** | **Verified** | Pion's official SIP-to-WebRTC example says bridging is possible because of the separate `emiago/sipgo` library; the example code imports `github.com/emiago/sipgo` and creates the SIP UA/server with it. This directly supports the conclusion that `pion/webrtc` itself is not the SIP server in the example. | Retain. The strongest precise formulation is: “`pion/webrtc` does not itself supply SIP signaling or a PSTN edge; Pion's official example adds `sipgo` for that role.” |
| **C4** | **Verified with wording correction** | Pion documents RTP/RTCP access and packetizers, not general media encoding/transcoding completeness. The SIP example negotiates PCMU on both sides and forwards RTP. Its README says multiparty calling requires mixing and points to `zaf/g711` for decode/encode around that mixing. | Replace “the example adds/calls for G.711 handling” with: “The example uses a same-codec PCMU path; for multiparty mixing its README points to G.711 decode/encode. Production paths need a codec/resampling/transcoding plan, although actual transcoding is required only when media processing or endpoint codec mismatch demands it.” |
| **C5** | **Verified** | The `ari` README explicitly describes the package as a Go library for accessing Asterisk ARI, configures Asterisk HTTP and WebSocket endpoints, and lists wrappers for playback, IVR, and recording. It therefore presupposes a running Asterisk instance. | Retain. “Pion + go-ari” still includes Asterisk wherever `go-ari` is used for channels, bridges, or media control. |
| **C6** | **Verified** | The repository README says `5.x.x` is current, says major APIs change, and shows installation with `/v6`. GitHub tags show v5.3.2 as the latest tagged line, while pkg.go.dev exposes `/v6` as an untagged pseudo-version and warns that the highest tagged major is v5. | Retain the inconsistency finding. Add the pkg.go.dev module page as the independent package-metadata check and avoid naming a compatible combination until the exact module revision and Asterisk release are tested. |
| **C7** | **Verified** | The current ARI Channels API lists originate/create, dial, answer, DTMF, hold/unhold, recording, RTP statistics, and External Media. External Media selects UDP/RTP, TCP/AudioSocket, or WebSocket/none and requires an audio format. Asterisk's External Media guide states that an external channel can receive bridge media and inject media back into the bridge, with Asterisk transcoding to the selected format. | Retain. For version precision, note that transport availability varies by Asterisk release; the “Latest API” surface must be checked against the chosen release. |
| **C8** | **Verified** | Asterisk's transfer guide says that when `PJSIP_TRANSFER_HANDLING()=ari-only` is enabled, the ARI application must act on REFER, report progress, manipulate channels/bridges for attended transfer, and originate/dial a new destination channel for blind transfer. | Retain the `ari-only` qualifier prominently. Without that configuration, the same guide says Asterisk handles REFER itself; the application-owned workflow is not the universal Asterisk default. |
| **C9** | **Verified** | LiveKit's SIP repository describes a separate SIP service for inbound/outbound calling, digest authentication, DTMF, trunks/dispatch, and room bridging. It uses Redis for communication/session state. LiveKit's self-hosting page requires Internet access to SIP port 5060 and RTP ports 10000-20000. | Retain. This verifies deployment dependencies and reachability, not carrier interoperability or SIP-service failover behavior. |
| **C10** | **Verified** | Current LiveKit telephony documentation lists send/receive DTMF, call forwarding and agent-assisted transfer, HD voice via wideband codecs, and TLS/SRTP secure trunking. | Retain the existing provider-compatibility caveat. These are vendor-documented features, not proof that every feature works with every target trunk/provider combination. |
| **C11** | **Unverified as written** | The cited distributed page verifies Redis-backed multi-node routing and graceful connection draining for **LiveKit room-server nodes**. It warns that terminating a node hosting active sessions would be disruptive, then documents graceful signal-driven drain. It does not document the separate SIP service's HA behavior, nor active SIP-call/room migration after abrupt node loss. | Replace with: “LiveKit server documents Redis-backed room placement and graceful draining. The admitted evidence does not establish LiveKit SIP-service failover or seamless active-call/active-room migration after abrupt node failure; both remain unverified.” Apply the same correction to the decision matrix and executive HA wording. |
| **C12** | **Verified with wording correction** | The `ari` README verifies node-bearing resource keys and points to `ari-proxy`; the proxy README verifies multi-instance discovery, command/event routing, and distribution of new calls. Neither document claims replication or migration of an already active Asterisk channel/media session to another Asterisk node. | Characterize this only as an absence-of-guarantee finding: “The docs support distributed addressing/control and new-call distribution, but do not establish active-call migration.” Do not convert silence in the docs into a measured failure claim. |
| **C13** | **Verified as an evidence-quality finding** | None of the admitted round-1 sources supplies two independent production measurements for “hundreds of times” language speedup, sub-millisecond end-to-end media handling, telephony call capacity, or a Pipecat per-worker ceiling. Pion's benchmark material concerns specific WebRTC workloads/connection phases; LiveKit's published benchmark is a synthetic single-room participant workload, not SIP/AI call capacity. | Retain all underlying numbers as **unverified** and exclude them from selection. Do not replace them with Pion or LiveKit synthetic benchmark numbers as though they measured this system. |
| **C14** | **Verified as an evidence-quality finding** | The admitted sources do not support the quoted schedule multipliers, calendar estimates, or team sizes. The component list makes the estimate's scope sensitivity visible but does not generate a defensible schedule. | Retain as **unverified**. “Scope-incomplete” is an assessment, not a quantitative disproof. |
| **C15** | **Verified with status correction** | LiveKit's self-hosting and SIP docs verify that deployment requires public signaling/media reachability, trunk/dispatch resources, addressing, authentication choices, numbers, and RTP configuration. They do not support a universal three-minute production onboarding time. Some additional acceptance categories in C15 are reasonable engineering checks but are not all established by the single cited README. | Change the duration proposition from **overturned** to **unverified**. Use: “No admitted source establishes a universal three-minute production onboarding time; the docs show configuration and reachability prerequisites, while provider-specific codec, DTMF, transfer, security, NAT, and policy acceptance still require testing.” |
| **C16** | **Verified with wording correction** | C1-C10 support the capability comparison: Pion supplies the WebRTC portion, Asterisk exposes a mature ARI/External Media seam, and LiveKit supplies a separate SIP-to-room service. That makes staged adoption a rational, reversible default under uncertainty. The sources do not prove it is optimal for an unspecified carrier mix, load, SLO, team, or cost profile. | Say: “The evidence supports staged adoption as the conservative default pending project-specific carrier, load, HA, and cost validation.” Keep final business-choice confidence at medium. |

## Load-bearing wording corrections

### Pion and the official SIP example

Use this precise wording throughout:

> Pion WebRTC implements WebRTC primitives; it does not itself supply SIP signaling or a PSTN edge. Pion's official SIP-to-WebRTC example imports `emiago/sipgo` to accept SIP. The example uses a same-codec PCMU RTP path; its README points to `zaf/g711` decode/encode for multiparty mixing and states that multiparty calls require mixing.

This preserves the digest's conclusion while avoiding the inaccurate implication that the example implements a general G.711 codec/transcoding layer.

### `go-ari` and Asterisk

Use this precise wording:

> `CyCoreSystems/ari` is a Go client for Asterisk ARI. It controls resources owned by a running Asterisk instance over ARI; it does not implement Asterisk's SIP channel driver, bridges, media engine, or External Media service. `ari-proxy` can distribute control traffic and new-call handling across applications/Asterisk instances, but its documentation does not establish migration of an active channel or media session between Asterisk nodes.

### Asterisk External Media and call control

Use this precise wording:

> Current Asterisk ARI exposes channel/call-control operations and External Media channels. External Media can connect an application to a bridge through supported transport/encapsulation combinations and a chosen audio format, allowing media to flow out of and into the bridge. Version support must be checked against the selected Asterisk release. In `ari-only` SIP transfer mode, the ARI application owns transfer progress and bridge/channel manipulation; that responsibility should not be generalized to Asterisk's default REFER handling.

### LiveKit SIP and HA

Use this precise wording:

> Self-hosted LiveKit telephony deploys a separate SIP service alongside the LiveKit room server and Redis, with public SIP and RTP reachability. LiveKit documents distributed room placement and graceful drain for LiveKit server nodes. The admitted sources do not document equivalent SIP-service HA semantics or seamless migration of an active SIP call/room after abrupt node loss.

### Performance, scale, delivery, and failure behavior

- All claimed Go/Python speed ratios, sub-millisecond media timings, LiveKit call counts, Pipecat concurrency limits, delivery multipliers, calendar estimates, and team sizes remain **unverified**.
- LiveKit's published room benchmark and Pion's benchmark tooling are workload-specific and do not validate end-to-end SIP/AI voice capacity or latency.
- Graceful draining is not active-session migration. Documentation of graceful shutdown does not prove behavior under abrupt process, host, network, or Redis failure.
- `go-ari`/`ari-proxy` distributed addressing and routing do not prove replication of live Asterisk media/channel state.
- Lack of evidence is **unverified**, not **overturned**. This applies especially to the “three-minute onboarding” duration.

## Source checks

1. [Pion WebRTC README](https://raw.githubusercontent.com/pion/webrtc/master/README.md)
2. [Pion SIP-to-WebRTC example README](https://github.com/pion/example-webrtc-applications/tree/main/sip-to-webrtc)
3. [Pion SIP-to-WebRTC example source](https://raw.githubusercontent.com/pion/example-webrtc-applications/main/sip-to-webrtc/main.go)
4. [Pion organization repositories](https://github.com/pion)
5. [CyCoreSystems Go ARI client](https://github.com/CyCoreSystems/ari)
6. [Go package metadata for `CyCoreSystems/ari/v6`](https://pkg.go.dev/github.com/CyCoreSystems/ari/v6)
7. [CyCoreSystems ARI proxy](https://github.com/CyCoreSystems/ari-proxy)
8. [Asterisk ARI Channels API](https://docs.asterisk.org/Latest_API/API_Documentation/Asterisk_REST_Interface/Channels_REST_API/)
9. [Asterisk External Media guide](https://docs.asterisk.org/Development/Reference-Information/Asterisk-Framework-and-API-Examples/External-Media-and-ARI/)
10. [Asterisk ARI transfer handling](https://docs.asterisk.org/Configuration/Interfaces/Asterisk-REST-Interface-ARI/Introduction-to-ARI-Transfer-Handling/)
11. [LiveKit SIP service README](https://raw.githubusercontent.com/livekit/sip/main/README.md)
12. [LiveKit self-hosted SIP server](https://docs.livekit.io/transport/self-hosting/sip-server/)
13. [LiveKit telephony features](https://docs.livekit.io/telephony/features/)
14. [LiveKit distributed multi-region server](https://docs.livekit.io/transport/self-hosting/distributed/)
15. [LiveKit benchmarking](https://docs.livekit.io/transport/self-hosting/benchmark/)

## Verification disposition

**Accept with corrections.** The core staged-adoption verdict remains technically defensible, but synthesis must carry the four corrections above—especially the Pion/G.711 wording and the separation between LiveKit room-server HA documentation and LiveKit SIP-service/active-call HA behavior.
