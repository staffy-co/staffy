#!/usr/bin/env python3
# step-03 patch F: web/templates/case.html — the accommodation section, the
# accuracy qualifier select, the transfer card words, the policy
# alternatives, the reply qualifier, the continuation hint
import io, sys

def patch(path, anchors):
    with io.open(path, encoding="utf-8") as f:
        text = f.read()
    for old, new in anchors:
        if text.count(old) != 1:
            if new in text:
                print("SKIP (already patched)", path)
                continue
            print("FAIL anchor (count=%d) in %s:\n%r" % (text.count(old), path, old[:150]))
            sys.exit(1)
        text = text.replace(old, new)
    with io.open(path, "w", encoding="utf-8", newline="") as f:
        f.write(text)
    print("patched", path)

# 1. The transfer request card: the preference words visible BEFORE the ack
transfer_old = """<caption>Запрос передачи: цель, дедлайн, remaining — назначено (assigned) ≠ подтверждено (acknowledged)</caption>"""
transfer_new = """<caption>Запрос передачи: цель, дедлайн, remaining — назначено (assigned) ≠ подтверждено (acknowledged)</caption>
{{if .View.Ownership.AccommodationWords}}<p class="next-note" role="note">Предпочтения связи клиента (видны принимающему до подтверждения): {{.View.Ownership.AccommodationWords}}.</p>{{end}}"""

# 2. The sales reply form: the accuracy qualifier select WITHOUT preselect
reply_old = """<label for="reply-commitment">Связь обязательства (только sales-класс: предъявить материальные условия offered-версии)</label>"""
reply_new = """<label for="reply-qualifier">Ограничение точности (обязательно, когда действующий исход несёт активные альтернативы — sales-класс; выбирайте осознанно)</label>
<select id="reply-qualifier" name="accuracy_qualifier">
<option value="">— без ограничения (альтернатив в исходе нет) —</option>
{{range $.View.AccuracyQualifierOptions}}<option value="{{.Value}}">{{.Text}}</option>{{end}}
</select>
<label for="reply-commitment">Связь обязательства (только sales-класс: предъявить материальные условия offered-версии)</label>"""

# 3. The policy current outcome card: the active alternatives with the honest limitation texts
policy_old = """{{if .Current.NextSteps}}<p>Следующие шаги: {{range $i, $step := .Current.NextSteps}}{{if $i}}; {{end}}{{$step}}{{end}}.</p>{{end}}"""
policy_new = """{{if .Current.NextSteps}}<p>Следующие шаги: {{range $i, $step := .Current.NextSteps}}{{if $i}}; {{end}}{{$step}}{{end}}.</p>{{end}}
{{if .Current.Alternatives}}<p>Активные альтернативы (честные пути без media): {{range $i, $alt := .Current.Alternatives}}{{if $i}}; {{end}}<strong>{{$alt.WordText}}</strong> — {{$alt.LimitationText}}{{end}}. Продажный ответ обязан заявить одно из этих ограничений точности (accuracy_qualifier).</p>{{end}}"""

# 4. The reply rows: the qualifier word of the honest limitation
row_old = """<thead><tr><th scope="col">Ответ</th><th scope="col">Канал</th><th scope="col">Класс</th><th scope="col">Текст</th><th scope="col">Состояние</th><th scope="col">Epoch</th><th scope="col">Создан</th></tr></thead>"""
row_new = """<thead><tr><th scope="col">Ответ</th><th scope="col">Канал</th><th scope="col">Класс</th><th scope="col">Текст</th><th scope="col">Ограничение точности</th><th scope="col">Состояние</th><th scope="col">Epoch</th><th scope="col">Создан</th></tr></thead>"""
row_body_old = """{{range .View.Ownership.ReplyRows}}<tr><th scope="row"><span class="digest-block">{{.ReplyID}}</span></th><td>{{.Channel}}</td><td>{{.ClassText}}</td><td>{{.Content}}</td><td>{{if .State}}<span class="alert" role="status">{{.StateText}}</span>{{else}}{{.StateText}}{{end}}</td><td>{{.Epoch}}</td><td>{{.CreatedAt}}</td></tr>{{end}}"""
row_body_new = """{{range .View.Ownership.ReplyRows}}<tr><th scope="row"><span class="digest-block">{{.ReplyID}}</span></th><td>{{.Channel}}</td><td>{{.ClassText}}</td><td>{{.Content}}</td><td>{{if .QualifierText}}{{.QualifierText}}{{else}}—{{end}}</td><td>{{if .State}}<span class="alert" role="status">{{.StateText}}</span>{{else}}{{.StateText}}{{end}}</td><td>{{.Epoch}}</td><td>{{.CreatedAt}}</td></tr>{{end}}"""

# 5. The new «Предпочтения связи» section before the Work Control Card
section_old = """<h2>Work Control Card — рабочие элементы</h2>"""
section_new = """<h2>Предпочтения связи — равноправные пути обслуживания</h2>
<p class="next-note" id="accommodations-panel-note" aria-live="polite">{{.View.AccommodationsPanelText}}</p>
<section class="panel" aria-label="Предпочтения связи">
<p id="accommodations-current" aria-live="polite">{{if .View.AccommodationCurrentRows}}Текущий набор: {{range $i, $pref := .View.AccommodationCurrentRows}}{{if $i}}; {{end}}<strong>{{$pref.KeyText}}</strong>{{if $pref.Value}} = {{$pref.Value}}{{end}} (ревизия {{$pref.Revision}}){{end}}.{{else}}Записанных предпочтений нет.{{end}}</p>
{{if .View.HasAccommodations}}
<details>
<summary>История ревизий ({{len .View.AccommodationHistoryRows}}) — append-only, включая снятия</summary>
<div class="datamap-table-wrap" tabindex="0" role="group" aria-label="История предпочтений связи">
<table class="datamap-table">
<caption>Ревизии: ключ, номер, значение, снятие, заметка, участник, время</caption>
<thead><tr><th scope="col">Ключ</th><th scope="col">Ревизия</th><th scope="col">Значение</th><th scope="col">Снятие</th><th scope="col">Заметка</th><th scope="col">Участник</th><th scope="col">Время</th></tr></thead>
<tbody>
{{range .View.AccommodationHistoryRows}}<tr><th scope="row">{{.KeyText}}</th><td>{{.Revision}}</td><td>{{if .Value}}{{.Value}}{{else}}— (флаг) —{{end}}</td><td>{{if .Cleared}}снято (ключ покидает текущий набор){{else}}действует{{end}}</td><td>{{if .Note}}{{.Note}}{{else}}—{{end}}</td><td><span class="digest-block">{{.ActorID}}</span></td><td>{{.RecordedAt}}</td></tr>{{end}}
</tbody>
</table>
</div>
</details>
{{end}}
{{if .View.CanManageWork}}
<details class="connector-connect"><summary>Записать предпочтение связи (record_accommodation)</summary>
<form method="post" action="/app/tenants/{{$.TenantID}}/cases/{{$.View.CaseID}}/accommodations">
<label for="accommodation-key">Ключ предпочтения (словарь фиксирован)</label>
<select id="accommodation-key" name="accommodation_key" required>
<option value="">— выберите ключ —</option>
{{range $.View.AccommodationKeyOptions}}<option value="{{.Value}}">{{.Text}}</option>{{end}}
</select>
<label for="accommodation-value">Значение (только preferred_channel — слово словаря каналов; available_time — строка 1..100 рун; флагам значение не нужно)</label>
<input id="accommodation-value" name="value" type="text" maxlength="100">
<label><input type="checkbox" name="cleared"> Снять предпочтение (своей ревизией — одновременно со значением нельзя)</label>
<label for="accommodation-note">Заметка оператора (до 500 символов; остаётся в строке — не покидает таблицу)</label>
<textarea id="accommodation-note" name="note" rows="2" maxlength="500"></textarea>
<input type="hidden" name="expected_accommodation_records" value="{{.View.AccommodationRecordsAnchor}}">
<button class="primary" type="submit">Записать ревизию</button>
</form>
<p class="next-note">Повторная запись того же ключа — новая ревизия (история не правится); снятие убирает ключ из текущего набора. Предпочтения — операционная настройка: не факт, не диагноз, не доказательство.</p>
</details>
{{end}}
</section>

<h2>Work Control Card — рабочие элементы</h2>"""

# 6. The continuation issue form: the honest no_web/no_voice hint
cont_old = """<button class="primary" type="submit">Выдать ссылку (одноразовый URL будет показан выше)</button>
</form>
<p class="next-note">Токен генерируется сервером один раз; в базе хранится только хеш. Потерянный URL не восстанавливается: отзовите ссылку и выдайте новую.</p>"""
cont_new = """<button class="primary" type="submit">Выдать ссылку (одноразовый URL будет показан выше)</button>
</form>
<p class="next-note">Токен генерируется сервером один раз; в базе хранится только хеш. Потерянный URL не восстанавливается: отзовите ссылку и выдайте новую.</p>
{{if $.View.ContinuationSurfaceHint}}<p class="next-note" role="note">{{$.View.ContinuationSurfaceHint}}</p>{{end}}"""

patch("web/templates/case.html", [
    (transfer_old, transfer_new),
    (reply_old, reply_new),
    (policy_old, policy_new),
    (row_old, row_new),
    (row_body_old, row_body_new),
    (section_old, section_new),
    (cont_old, cont_new),
])

print("ALL PATCHES OK")
