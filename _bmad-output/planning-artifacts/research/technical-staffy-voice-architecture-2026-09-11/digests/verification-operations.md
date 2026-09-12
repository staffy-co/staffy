# Verification — operations / TCO / ecosystem

**Проверено:** 2026-09-11  
**Scope:** только `operations-tco-ecosystem-r1-1.md` и восемь первичных веб-источников. Ниже приводится текст норм и технических публикаций, не юридическая консультация.

## Итог

Вердикт по главному архитектурному тезису: **условно подтвержден как рабочая гипотеза, но не подтвержден как безусловный default**. Официальная документация LiveKit действительно поддерживает self-hosted Server, self-hosted Agents, отдельные self-hosted SIP/Egress и подключение внешних model providers. SpeechKit публикует подходящие usage-тарифы. Однако выбранный набор источников не доказывает production-совместимость конкретного адаптера Yandex с выбранной версией Agents, российскую локализацию всех временных данных/логов/backup у SpeechKit, SLA, российскую SIP-связность, нагрузочный профиль или TCO. Поэтому «self-hosted LiveKit + российские STT/TTS» — разумный **conditional baseline для spike/pilot**, а не уже доказанный production-default.

## Результаты spot-check

### A. LiveKit pricing, operations и регионы

- **VERIFIED — текущие ценовые якоря.** LiveKit публикует Ship за $50/мес. и Scale за $500/мес. На Ship включено 5 000 agent-session minutes, затем $0.01/мин; 5 000 минут записей, затем $0.005/мин; 500 000 observability events, затем $0.00003/event. Текущий калькулятор по выбранной конфигурации действительно показывает $0.0672/мин. Region pinning указан начиная со Scale. Источник: [LiveKit Pricing](https://livekit.com/pricing).
- **VERIFIED — operational split.** Для self-hosting документация говорит, что команда сама запускает и масштабирует agent servers, использует собственные provider API keys, строит observability через внешние/custom OpenTelemetry hooks, а SIP, Egress и Ingress разворачивает отдельными сервисами. Cloud включает managed builds/autoscaling/rollback, встроенную observability, SIP/Egress/Ingress и заявленный uptime target 99.99%. Источник: [LiveKit self-hosting overview](https://docs.livekit.io/transport/self-hosting/).
- **VERIFIED — опубликованного российского Cloud-региона нет.** Текущие списки realtime region groups, SIP endpoints и agent deployment regions не содержат Россию. Realtime-группы с несколькими локациями сохраняют только внутригрупповую избыточность; agent deployments перечислены лишь для `us-east`, `eu-central`, `ap-south`. Это подтверждает вывод, что публично описанный LiveKit Cloud нельзя считать российским контуром. Возможные непубличные enterprise/private arrangements не проверены. Источник: [LiveKit regions and endpoints](https://docs.livekit.io/deploy/admin/regions/endpoints/).

### B. Yandex SpeechKit

- **VERIFIED.** В российском прайсе, включая НДС: streaming STT — 0,1626 ₽ за единицу в 15 секунд одноканального аудио; каждый короткий фрагмент округляется вверх независимо, поэтому 1 секунда становится 15 секундами. Непрерывная минута = 4 × 0,1626 = 0,6504 ₽; 10 000 минут = 6 504 ₽. TTS v3 — 0,1626 ₽ за запрос/единицу; стандартный лимит — 250 символов и 24 секунды, а `unsafe_mode`/streaming тарифицируются по каждой начатой 250-символьной единице. TTS v1 — 1 342 ₽ за 1 млн символов. Источник: [Yandex SpeechKit pricing rules](https://aistudio.yandex.ru/ru/docs/speechkit/pricing).

### C. 152-ФЗ и 323-ФЗ

- **VERIFIED — текст 152-ФЗ в найденной официальной консолидации от 01.09.2025.** Часть 5 статьи 18 запрещает при сборе персональных данных граждан РФ использовать базы за пределами РФ для записи, систематизации, накопления, хранения, уточнения и извлечения, кроме прямо перечисленных исключений. Статья 10 относит сведения о состоянии здоровья к специальным категориям и допускает обработку только в предусмотренных частями 2 и 2.1 случаях; среди них есть медицинские цели при обработке профессиональным медицинским работником, обязанным хранить врачебную тайну. Источник: [официальный консолидированный текст 152-ФЗ](https://ips.pravo.gov.ru/api/ips/legislation/document?baseid=None&hash=98490812b3409e2a8d78a11ca9010f434ea3d9250a11dbbdb78690cd5551bdd6).
- **UNVERIFIED — полнота 152-ФЗ на 2026-09-11.** Поиск вернул официальный текст с датой консолидации 01.09.2025, но не дал более поздней официальной редакции. Сами приведенные положения подтверждены; отсутствие последующих изменений после этой даты данным spot-check не доказано.
- **VERIFIED — исходное официальное определение врачебной тайны в 323-ФЗ.** Официальная публикация 323-ФЗ определяет как врачебную тайну сведения о факте обращения за медицинской помощью, состоянии здоровья, диагнозе и иную информацию, полученную при медицинском обследовании и лечении. Это поддерживает осторожный вывод, что запись/транскрипт медицинского звонка потенциально содержит врачебную тайну уже при capture. Источник: [официальная публикация 323-ФЗ](https://publication.pravo.gov.ru/Document/View/0001201111220007).
- **UNVERIFIED — текущая консолидированная статья 13 и ее исключения на 2026-09-11.** Официальный поиск вновь вывел исходную публикацию 2011 года, но не текущую консолидированную редакцию статьи 13. Поэтому digest корректно не должен утверждать актуальный полный перечень оснований раскрытия без отдельной проверки.

### D. Releases

- **OVERTURNED — версия LiveKit в C8 устарела.** На дату проверки latest release — **v1.13.6**, опубликованный 2026-08-26, а не линия «through v1.9.12». При этом содержательный вывод об активной поддержке **VERIFIED и усилен**. Источник: [LiveKit Server releases](https://github.com/livekit/livekit/releases).
- **OVERTURNED — версия Pipecat в C9 устарела.** На дату проверки latest release — **v1.9.0**, опубликованный 2026-09-11 в 03:05, а не v1.7.0. Наблюдение другого потока о v1.9.0 подтверждено. Вывод об активной поддержке **VERIFIED**; предупреждение о migration/observability churn остается обоснованным, но требует version pinning. Источник: [Pipecat releases](https://github.com/pipecat-ai/pipecat/releases).

## Triage

- **Verified:** LiveKit Cloud pricing/allowances; self-hosted-vs-Cloud operational split; отсутствие России в опубликованных Cloud-регионах; SpeechKit STT/TTS цены и billing units; содержание найденной редакции статей 18(5) и 10 152-ФЗ; исходное определение врачебной тайны; активная release-активность обоих проектов.
- **Unverified:** официальный post-2025 consolidated status 152-ФЗ; текущая полная статья 13 323-ФЗ и все исключения; SpeechKit data-flow/residency/SLA; end-to-end production/TCO suitability российского контура.
- **Disputed:** нет прямого противоречия по смысловым операционным или ценовым claims; спорным остается только превращение conditional baseline в окончательный default без spike и договорных подтверждений.
- **Overturned:** версии LiveKit `v1.9.12` и Pipecat `v1.7.0` как текущие snapshots.

**Decision:** сохранить гибридный российский контур как приоритетный вариант для portability/load/residency spike; не маркировать его production-default до проверки адаптера, SIP, data-flow/SLA, failure drills и TCO.
