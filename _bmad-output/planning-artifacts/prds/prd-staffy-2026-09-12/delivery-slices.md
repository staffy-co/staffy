# Staffy MVP Delivery Slices

Этот файл является scope-контрактом PRD, а не архитектурой. Каждый диапазон включает все ID внутри диапазона. `Manual-with-audit` разрешает ручную операцию пилота, но не отменяет авторизацию, Confirmation, один Response Owner, Action Receipt, fail-closed и измерение результата.

| Требования | Первый обязательный gate | Delivery mode на этом gate | Минимальное доказательство |
|---|---|---|---|
| FR-1–FR-6 | controlled обоих пилотов | productized core; setup первых Tenant может быть assisted | изоляция, роли, version/readiness и rollback на опубликованной конфигурации |
| FR-7–FR-8 | controlled соответствующего Connector | connector/configuration + productized capability validation | account-level contract test включает timeout до dispatch/после remote commit, duplicate/reordered webhook, key conflict и partial completion; неготовая Capability недоступна |
| FR-9–FR-17 | controlled | productized для пилотных каналов | единая Conversation, provenance, identity, one-owner, transition/degraded/reconnect tests |
| FR-18–FR-26 | controlled | productized policy runtime + Tenant/Domain Pack configuration | safe intake/promise, ambiguous Confirmation rejected, альтернативный путь |
| FR-27–FR-33 | controlled до первого mutating action | productized invariant | authorize/revalidate/fence → intent → side effect → Receipt/reconcile без false success |
| FR-34–FR-41 | controlled | productized handoff state + external Inbox/manual operator fallback | acknowledgement, fallback/reassignment, decision-ready context и Recovery continuation |
| FR-42–FR-45 | controlled | productized autonomy/eval/rollback control | режим класса, blocking eval и scoped rollback |
| FR-46–FR-48 | controlled → live | productized event capture; offline/manual report допустим до scale | сравнимые когорты и отчёт по version/class/outcome |
| FR-49 | third-client/platform proof | productized/assisted guided launch | независимый внешний клиент публикует существующий pack/connectors и проходит controlled exercise за один рабочий день без кода |
| FR-50–FR-56 | controlled autoparts | Domain Pack + ABCP/catalog connector | UJ-A1 до единственного customer Order с applicability evidence |
| FR-57–FR-59 | controlled autoparts | productized supplier fence + manual manager task | exact-version approval/blocked/expired receipt; supplier-send не обходится |
| FR-60–FR-63 | live autoparts | connector/reconciliation + offline business report допустим | правдивый ABCP/1С status и маржинальный исход по когорте |
| FR-64–FR-68 | controlled Montagio | Domain Pack/configuration; pricing table может управляться вручную | safe intake, Policy Outcomes P1–P6, P7 Safety Interrupt и versioned service×market offer |
| FR-69–FR-74 | controlled Montagio | productized commitment/assignment invariants; supply/dispatch допускают manual-with-audit | supply drill, capacity reservation; для публичной подрядной модели Commitment адресован Montagio, а субподрядчик назначается под её ответственность; прямая клиент↔мастер ветка допустима только как честно раскрытая закрытая проверка спроса |
| FR-75–FR-77 | live Montagio до scale | productized linkage; finance/claims operations допускают manual-with-audit | вручную сверенная оплаченная микрокогорта первых 3–5 стандартизированных услуг только после OQ-10; подрядная модель включает договор/оплату/чек/гарантию Montagio и Recovery evidence |
| FR-78–FR-82 | third-client/platform proof; underlying controls — controlled | assisted/configuration для reference pilots, productized guided experience к FR-49 | UJ-P1/P2, knowledge/behavior/voice/media preview и no-code supported launch |
| FR-83 | controlled до real-customer path | supported external Inbox либо minimal Staffy-provided surface | full handoff/return/recovery rehearsal |
| FR-84 | до controlled и повторно до live | versioned artifact; approval может быть manual-with-audit | frozen Operating Profile, Gate Definition и Coverage Matrix |
| FR-85 | до платного live-пилота | manual contract/invoice/activation допустим; usage visibility productized | versioned Entitlement, active/paused/ended и reconciled usage |
| NFR-1–NFR-11 | controlled | productized safety/reliability for walking skeleton | failure, reconnect, voice-route and critical-outcome suite |
| NFR-12–NFR-13 | live capacity gate | load test + configured admission/degraded policy | Pilot Load Envelope × approved safety factor |
| NFR-14–NFR-20 | до real ПДн/audio/media | productized access/audit; policies and approvals may be manual | artifact data map, consent/retention and hostile-file tests |
| NFR-21–NFR-23 | controlled | productized supported-path behavior | alternative path, clear Commitment and human control |
| NFR-24–NFR-28 | controlled → live | productized correlation/release/rollback; offline cost report допустим initially | end-to-end trace, release manifest, budget/reconcile/rollback test |
| NFR-29–NFR-32 | controlled voice/text | eval + human assessment | segmented conversation acceptance report before live |

## Walking-skeleton outcome order

1. **Safe intake:** identity/provenance, Work Items, Safety Interrupt, supported channel and honest capability-fit.
2. **Safe promise:** current Owning System facts, Policy Outcome, explicit versioned Commitment and unambiguous Confirmation.
3. **Safe fulfilment:** one fenced/revalidated Business Action, capacity/supplier guard and Action Receipt/reconciliation.
4. **Safe close/recovery:** human acknowledgement, truthful ownership transition, outcome/recovery linkage and no false success.
5. **Business proof:** frozen cohort definitions, comparable human path, contribution/margin/recovery and Staffy commercial evidence.

Escalation to a separate subsystem is required only when the current manual/configured mechanism cannot meet the next gate's volume, auditability, safety or response-time threshold.
