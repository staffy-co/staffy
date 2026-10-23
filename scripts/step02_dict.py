#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""step-02 surgical patch 1/3:
a) tests/integration/tenant_test.go — регистрация миграций 00020/00021 (урок 3-3/00017);
b) internal/cases/domain/human_handoff.go — HandoffReasons +1 fitment_research + тексты;
c) internal/cases/domain/fitment.go — переключение на публичную константу (прецедент 4d076b5)."""
import io

# --- a) TestMain: после 00019 идёт action-модуль; вставляем 00020/00021 СРАЗУ после 00019
P = 'tests/integration/tenant_test.go'
s = io.open(P, encoding='utf-8').read()
anchor = '''	if _, err = adminPool.Exec(ctx, strings.Split(string(casesPartRequestReasonsMigration), "-- +goose Down")[0]); err != nil {
		panic(err)
	}
'''
assert anchor in s, 'tenant_test.go: 00019 anchor not found'
insert = anchor + '''	// Story 4.4: the fitment evidence table + the reason dictionary word.
	// The registration stays in this hard list (the 3.3 lesson of 00017: a
	// migration outside the list never applies its tables and the whole
	// contour tests a world that does not exist).
	casesFitmentMigration, err := os.ReadFile("../../migrations/cases/00020_fitment_evidences.sql")
	if err != nil {
		panic(err)
	}
	if _, err = adminPool.Exec(ctx, strings.Split(string(casesFitmentMigration), "-- +goose Down")[0]); err != nil {
		panic(err)
	}
	casesFitmentReasonMigration, err := os.ReadFile("../../migrations/cases/00021_fitment_research_reason.sql")
	if err != nil {
		panic(err)
	}
	if _, err = adminPool.Exec(ctx, strings.Split(string(casesFitmentReasonMigration), "-- +goose Down")[0]); err != nil {
		panic(err)
	}
'''
s = s.replace(anchor, insert, 1)
io.open(P, 'w', encoding='utf-8').write(s)
print('tenant_test.go: +00020/+00021 registered')

# --- b) human_handoff.go: словарь +1 слово + тексты
P2 = 'internal/cases/domain/human_handoff.go'
s2 = io.open(P2, encoding='utf-8').read()
old_consts = '''	HandoffReasonClientRequest           = "client_request"
	HandoffReasonDiagnosticUncertainty   = "diagnostic_uncertainty"
	HandoffReasonSafetyCritical          = "safety_critical"
	HandoffReasonUnresolvedContradiction = "unresolved_contradiction"
'''
new_consts = '''	HandoffReasonClientRequest           = "client_request"
	HandoffReasonDiagnosticUncertainty   = "diagnostic_uncertainty"
	HandoffReasonSafetyCritical          = "safety_critical"
	HandoffReasonUnresolvedContradiction = "unresolved_contradiction"
	// HandoffReasonFitmentResearch is the Story 4.4 research task word:
	// the fitment panel recommends the manager's catalog research — the
	// decision-ready task of 2.14 with the recorded material (the SQL
	// mirror 00021 admits it from this spec on; the automatic catalog
	// request is never performed — OQ-5/AG-4).
	HandoffReasonFitmentResearch         = "fitment_research"
'''
assert old_consts in s2, 'human_handoff.go: consts anchor not found'
s2 = s2.replace(old_consts, new_consts, 1)

old_dict = '''var HandoffReasons = []string{
	HandoffReasonClientRequest,
	HandoffReasonDiagnosticUncertainty,
	HandoffReasonSafetyCritical,
	HandoffReasonUnresolvedContradiction,
}'''
new_dict = '''var HandoffReasons = []string{
	HandoffReasonClientRequest,
	HandoffReasonDiagnosticUncertainty,
	HandoffReasonSafetyCritical,
	HandoffReasonUnresolvedContradiction,
	// The Story 4.4 word — the fifth: the dictionary extension is the new
	// spec version, the SQL mirror 00021 backstops the same list.
	HandoffReasonFitmentResearch,
}'''
assert old_dict in s2, 'human_handoff.go: dictionary anchor not found'
s2 = s2.replace(old_dict, new_dict, 1)

old_texts = '''	HandoffReasonUnresolvedContradiction: "Неустранённое противоречие источников: Staffy не выбирает вероятный вариант — материал передан человеку.",
}'''
new_texts = '''	HandoffReasonUnresolvedContradiction: "Неустранённое противоречие источников: Staffy не выбирает вероятный вариант — материал передан человеку.",
	HandoffReasonFitmentResearch:          "Каталогическое исследование применимости: доказательства нет либо оно устарело — менеджеру задача исследовать каталог и записать результат.",
}'''
assert old_texts in s2, 'human_handoff.go: texts anchor not found'
s2 = s2.replace(old_texts, new_texts, 1)
io.open(P2, 'w', encoding='utf-8').write(s2)
print('human_handoff.go: +fitment_research word, dictionary 5, text')

# --- c) fitment.go: переключение локального слова на публичную константу
P3 = 'internal/cases/domain/fitment.go'
s3 = io.open(P3, encoding='utf-8').read()
old_word = '''// FitmentResearchWord is the manager task reason word of the fitment
// research recommendation (the 2.13 dictionary extension of Story 4.4 — the
// SQL mirror 00021 admits it from step-02 on; the write path guard of the
// handoff reason stays 4+client_request words until then: the word lives
// here first exactly as the 4.3 precedent — the part boundary words lived
// in part.go first, then switched onto the public human_handoff constants
// with the dictionary extension step).
const FitmentResearchWord = "fitment_research"'''
new_word = '''// FitmentResearchWord is the manager task reason word of the fitment
// research recommendation — the PUBLIC dictionary constant of 2.13 (the
// Story 4.4 extension, the SQL mirror 00021; the step-02 switch onto the
// public constant follows the 4.3 precedent of the part boundary words).
const FitmentResearchWord = HandoffReasonFitmentResearch'''
assert old_word in s3, 'fitment.go: word anchor not found'
s3 = s3.replace(old_word, new_word, 1)
io.open(P3, 'w', encoding='utf-8').write(s3)
print('fitment.go: FitmentResearchWord -> public constant')
