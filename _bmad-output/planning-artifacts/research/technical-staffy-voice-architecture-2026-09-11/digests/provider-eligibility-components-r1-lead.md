# Digest: provider eligibility and Russian speech components — round 1 lead

Accessed: 2026-09-11

## Findings

- **claim:** OpenAI's current API country list does not include Russia and explicitly says that access or provision of access outside listed countries may lead to account suspension. This makes OpenAI Realtime an unsuitable default production dependency for a Russia-operated Staffy pilot, even though its API technically supports WebRTC, WebSocket and SIP.
  - **source:** https://help.openai.com/en/articles/5347006-openai-api-supported-countries-and-territories
  - **publisher:** OpenAI
  - **pub_date:** current, accessed 2026-09-11
  - **confidence:** high for published eligibility; medium for project consequence
  - **class:** compatibility
- **claim:** OpenAI's unsupported-country guidance independently repeats the risk of blocking or suspension, but it is the same publisher and therefore corroborates wording rather than providing an independent source.
  - **source:** https://help.openai.com/en/articles/9131992-chatgpt-and-api-services-in-unsupported-countries-and-territories
  - **publisher:** OpenAI
  - **pub_date:** current, accessed 2026-09-11
  - **confidence:** high
  - **class:** compatibility
- **claim:** The OpenAI Realtime API currently exposes low-latency speech-to-speech over WebRTC, WebSocket and SIP; technical capability does not remove the country-access constraint.
  - **source:** https://platform.openai.com/docs/api-reference/realtime
  - **publisher:** OpenAI
  - **pub_date:** current, accessed 2026-09-11
  - **confidence:** high
  - **class:** feature
- **claim:** Yandex SpeechKit's current streaming STT documentation states a maximum streaming session duration of five minutes and requires opening a new session. A component pipeline using SpeechKit therefore needs explicit session rollover that preserves call context.
  - **source:** https://yandex.cloud/ru-kz/docs/speechkit/qa/stt
  - **publisher:** Yandex Cloud
  - **pub_date:** 2026-08-18
  - **confidence:** high for the documented limit; medium for the architectural consequence
  - **class:** compatibility
- **claim:** Yandex SpeechKit v3 exposes bidirectional streaming gRPC synthesis intended for real-time output, allowing a replaceable STT/LLM/TTS pipeline instead of a monolithic voice-agent API.
  - **source:** https://yandex.cloud/en/docs/speechkit/tts-v3/api-ref/grpc/Synthesizer/streamSynthesis
  - **publisher:** Yandex Cloud
  - **pub_date:** 2025-08-08
  - **confidence:** medium-high; official but older than the one-month compatibility window
  - **class:** integration
- **claim:** SaluteSpeech documents gRPC streaming recognition with interim hypotheses, automatic end-of-phrase detection and OAuth 2 bearer authorization, so it is technically a plausible Russian STT component rather than an end-to-end agent runtime.
  - **source:** https://developers.sber.ru/docs/ru/salutespeech/api/grpc/recognition-stream-2
  - **publisher:** Sber Developers
  - **pub_date:** 2026-03-05
  - **confidence:** medium-high
  - **class:** integration
- **claim:** SaluteSpeech's corporate tariff page updated 2026-08-31 simultaneously warns that purchases by new clients have been unavailable since 2026-07-15 and describes corporate prepaid/PAYG plans. Current onboarding availability is therefore ambiguous and requires direct vendor confirmation before selection.
  - **source:** https://developers.sber.ru/docs/ru/salutespeech/tariffs/legal-tariffs
  - **publisher:** Sber Developers
  - **pub_date:** 2026-08-31
  - **confidence:** high for the internal inconsistency; unverified for actual onboarding
  - **class:** ecosystem

## Contradictions

- SaluteSpeech's current corporate tariff page contains mutually difficult-to-reconcile onboarding language. Do not treat a published price as proof that a new Staffy corporate account can be activated.

## Leads

- Ask Sber directly whether new legal-entity SaluteSpeech projects can be activated and under which contract path.
- Test Yandex SpeechKit reconnection at the five-minute boundary while preserving partial hypotheses and interruption state.

## Not found

- No independent source was sought to override providers' own eligibility or API documentation; these are authoritative for access rules and interface contracts but do not establish production quality.
