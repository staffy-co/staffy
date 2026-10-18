package domain

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// The unit matrix of the control domain (story 3-2 step-01): every
// dictionary word, every rank, every window branch, the intersection
// probes in both directions, the corruption fail-closed reading and the
// reason bounds — by name. The honest branches carry the honesty probe:
// a narrower permission never cancels a wider blocking; damage never
// expands the allowance; an empty route is accept, not an error.

func refTime() time.Time { return time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC) }

func ptrTime(t time.Time) *time.Time { return &t }

func activeControl(kind, ref, mode, reason string) ScopedControl {
	return ScopedControl{ScopeKind: kind, ScopeRef: ref, Mode: mode, Reason: reason, State: ControlStateActive}
}

// ---------------------------------------------------------------------------
// The dictionaries
// ---------------------------------------------------------------------------

func TestControlModesExactAD12Words(t *testing.T) {
	want := []string{"accept", "intake_only/no_promises", "human_only", "paused"}
	if len(ControlModes) != len(want) {
		t.Fatalf("словарь режимов не из четырёх слов: %v", ControlModes)
	}
	for i, word := range want {
		if ControlModes[i] != word {
			t.Fatalf("слово %d: %q, ожидалось %q", i, ControlModes[i], word)
		}
	}
}

func TestIntakeOnlySlashIsOneWord(t *testing.T) {
	// The slash is part of the word (FR-32/AD-12 verbatim); renaming to
	// intake_only is a mirror-discipline violation. Pinned here at unit
	// level AND by the OpenAPI contract test of step-03.
	if !WordIn(ControlModes, "intake_only/no_promises") {
		t.Fatal("слово intake_only/no_promises отсутствует в словаре режимов")
	}
	if WordIn(ControlModes, "intake_only") {
		t.Fatal("переименование intake_only БЕЗ /no_promises просочилось в словарь — запрещено зеркальной дисциплиной")
	}
}

func TestControlModeRankLadder(t *testing.T) {
	cases := []struct {
		mode string
		rank int
	}{
		{ControlModeAccept, 0},
		{ControlModeIntakeOnly, 1},
		{ControlModeHumanOnly, 2},
		{ControlModePaused, 3},
	}
	for _, c := range cases {
		if got := ControlModeRank(c.mode); got != c.rank {
			t.Fatalf("ранг %q = %d, ожидался %d", c.mode, got, c.rank)
		}
	}
}

func TestControlModeRankOutsideDictionaryIsPaused(t *testing.T) {
	// Fail-closed by reading: an out-of-dictionary value carries the rank
	// of paused — the most safe. Corruption never expands the allowance.
	for _, mode := range []string{"", "intake_only", "PAUSED", "no_such_mode", "accept "} {
		if got := ControlModeRank(mode); got != 3 {
			t.Fatalf("вне-словарное %q получило ранг %d, ожидался ранг paused (3)", mode, got)
		}
	}
}

func TestSafeReadModeFailClosed(t *testing.T) {
	for _, mode := range ControlModes {
		if got := SafeReadMode(mode); got != mode {
			t.Fatalf("словарное слово %q прочиталось как %q", mode, got)
		}
	}
	for _, mode := range []string{"", "garbage", "Accept", "intake_only"} {
		if got := SafeReadMode(mode); got != ControlModePaused {
			t.Fatalf("испорченное значение %q читается как %q, ожидался paused", mode, got)
		}
	}
}

func TestScopeKindsSevenWords(t *testing.T) {
	want := []string{"tenant", "connector", "channel", "service", "policy_outcome", "autonomy_class", "campaign"}
	if len(ScopeKinds) != len(want) {
		t.Fatalf("словарь видов скоупа не из семи слов: %v", ScopeKinds)
	}
	for i, word := range want {
		if ScopeKinds[i] != word {
			t.Fatalf("вид %d: %q, ожидался %q", i, ScopeKinds[i], word)
		}
	}
}

func TestControlStatesTwoWords(t *testing.T) {
	want := []string{"active", "removed"}
	for i, word := range want {
		if ControlStates[i] != word {
			t.Fatalf("состояние %d: %q, ожидалось %q", i, ControlStates[i], word)
		}
	}
}

// ---------------------------------------------------------------------------
// ValidScopeRef — the honest boundaries per kind
// ---------------------------------------------------------------------------

func TestValidScopeRefTenantEquality(t *testing.T) {
	// AC-1: an object of a foreign tenant is materially unselectable.
	if err := ValidScopeRef(ScopeKindTenant, "t-tenant-1", "t-tenant-1"); err != nil {
		t.Fatalf("собственный тенент отказан: %v", err)
	}
	err := ValidScopeRef(ScopeKindTenant, "t-tenant-2", "t-tenant-1")
	if err == nil {
		t.Fatal("чужой тенент принят — объект другого тенента обязан быть невыбираем")
	}
	var rule RuleError
	if !errors.As(err, &rule) || rule.Rule != "scope_ref" {
		t.Fatalf("отказ чужого тенента не RuleError scope_ref: %v", err)
	}
	if err := ValidScopeRef(ScopeKindTenant, "t-tenant-1", ""); err == nil {
		t.Fatal("скоуп tenant без личности тенента принят")
	}
}

func TestValidScopeRefConnectorShape(t *testing.T) {
	// The lexical half only: the LIVE registry resolution is the app
	// port of step-02 (fail-closed there). Here the shape must hold.
	if err := ValidScopeRef(ScopeKindConnector, "staffy-synthetic-telephony", "t-tenant-1"); err != nil {
		t.Fatalf("живая форма connector-ID отказана: %v", err)
	}
	if err := ValidScopeRef(ScopeKindConnector, "novofon-sip.v2_0", "t-tenant-1"); err != nil {
		t.Fatalf("charset [a-z0-9._-] отказан: %v", err)
	}
	refusals := []string{"", "UPPER", "с-кириллицей", strings.Repeat("a", 121)}
	for _, ref := range refusals {
		if err := ValidScopeRef(ScopeKindConnector, ref, "t-tenant-1"); err == nil {
			t.Fatalf("connector-реф %q принят вопреки форме", ref)
		}
	}
	if err := ValidScopeRef(ScopeKindConnector, strings.Repeat("a", 120), "t-tenant-1"); err != nil {
		t.Fatalf("connector-реф ровно 120 рун отказан: %v", err)
	}
}

func TestValidScopeRefChannelDictionary(t *testing.T) {
	for _, channel := range ChannelWords {
		if err := ValidScopeRef(ScopeKindChannel, channel, "t-tenant-1"); err != nil {
			t.Fatalf("канал %q отказан: %v", channel, err)
		}
	}
	if err := ValidScopeRef(ScopeKindChannel, "sms", "t-tenant-1"); err == nil {
		t.Fatal("неизвестный канал принят")
	}
}

func TestValidScopeRefPolicyOutcomeDictionary(t *testing.T) {
	for _, outcome := range PolicyOutcomeWords {
		if err := ValidScopeRef(ScopeKindPolicyOutcome, outcome, "t-tenant-1"); err != nil {
			t.Fatalf("класс исхода %q отказан: %v", outcome, err)
		}
	}
	if err := ValidScopeRef(ScopeKindPolicyOutcome, "commercial", "t-tenant-1"); err == nil {
		t.Fatal("неизвестный класс исхода принят")
	}
}

func TestValidScopeRefAutonomyClassDictionary(t *testing.T) {
	for _, autonomy := range AutonomyClassWords {
		if err := ValidScopeRef(ScopeKindAutonomyClass, autonomy, "t-tenant-1"); err != nil {
			t.Fatalf("класс автономии %q отказан: %v", autonomy, err)
		}
	}
	if err := ValidScopeRef(ScopeKindAutonomyClass, "full_auto", "t-tenant-1"); err == nil {
		t.Fatal("неизвестный класс автономии принят")
	}
}

func TestValidScopeRefServiceCampaignHonestBoundary(t *testing.T) {
	// service/campaign carry NO registry in this slice: the shape is the
	// honest boundary — documented in code and on the page. The
	// impossibility of resolution never expands the allowance.
	for _, kind := range []string{ScopeKindService, ScopeKindCampaign} {
		if err := ValidScopeRef(kind, "montagio.services.v2", "t-tenant-1"); err != nil {
			t.Fatalf("валидный %s-реф отказан: %v", kind, err)
		}
		if err := ValidScopeRef(kind, "", "t-tenant-1"); err == nil {
			t.Fatalf("пустой %s-реф принят", kind)
		}
		if err := ValidScopeRef(kind, "Wide Space", "t-tenant-1"); err == nil {
			t.Fatalf("мусорный %s-реф принят", kind)
		}
	}
}

func TestValidScopeRefUnknownKind(t *testing.T) {
	err := ValidScopeRef("zone", "whatever", "t-tenant-1")
	if err == nil {
		t.Fatal("неизвестный вид скоупа принят")
	}
	var rule RuleError
	if !errors.As(err, &rule) || rule.Rule != "scope_kind" {
		t.Fatalf("отказ вида скоупа не RuleError scope_kind: %v", err)
	}
}

// ---------------------------------------------------------------------------
// ValidReason — the mandatory reason
// ---------------------------------------------------------------------------

func TestValidReasonBounds(t *testing.T) {
	if err := ValidReason("Рискованный маршрут telephony: инцидент поддержки"); err != nil {
		t.Fatalf("нормальная причина отказана: %v", err)
	}
	if err := ValidReason("   причина с полями   "); err != nil {
		t.Fatalf("причина с surrounding-пробелами отказана: %v", err)
	}
	for _, bad := range []string{"", "   ", "\n\t"} {
		if err := ValidReason(bad); err == nil {
			t.Fatalf("пустая причина %q принята — причина обязательна", bad)
		}
	}
	if err := ValidReason(strings.Repeat("р", 500)); err != nil {
		t.Fatalf("причина ровно 500 рун отказана: %v", err)
	}
	if err := ValidReason(strings.Repeat("р", 501)); err == nil {
		t.Fatal("причина 501 рун принята вопреки границе")
	}
}

// ---------------------------------------------------------------------------
// The read-time window
// ---------------------------------------------------------------------------

func TestInfluencingWindowBranches(t *testing.T) {
	now := refTime()
	cases := []struct {
		name    string
		control ScopedControl
		want    bool
	}{
		{"активный без окна", activeControl(ScopeKindConnector, "c", ControlModePaused, "r"), true},
		{"снятый", ScopedControl{ScopeKind: ScopeKindConnector, ScopeRef: "c", Mode: ControlModePaused, State: ControlStateRemoved}, false},
		{"запланированный в будущем", ScopedControl{ScopeKind: ScopeKindConnector, ScopeRef: "c", Mode: ControlModePaused, State: ControlStateActive, ScheduledStartAt: ptrTime(now.Add(time.Hour))}, false},
		{"запланированный ровно сейчас", ScopedControl{ScopeKind: ScopeKindConnector, ScopeRef: "c", Mode: ControlModePaused, State: ControlStateActive, ScheduledStartAt: ptrTime(now)}, true},
		{"запланированный в прошлом", ScopedControl{ScopeKind: ScopeKindConnector, ScopeRef: "c", Mode: ControlModePaused, State: ControlStateActive, ScheduledStartAt: ptrTime(now.Add(-time.Hour))}, true},
		{"истекающий в будущем", ScopedControl{ScopeKind: ScopeKindConnector, ScopeRef: "c", Mode: ControlModePaused, State: ControlStateActive, ExpiresAt: ptrTime(now.Add(time.Hour))}, true},
		{"истекающий ровно сейчас", ScopedControl{ScopeKind: ScopeKindConnector, ScopeRef: "c", Mode: ControlModePaused, State: ControlStateActive, ExpiresAt: ptrTime(now)}, false},
		{"истекший", ScopedControl{ScopeKind: ScopeKindConnector, ScopeRef: "c", Mode: ControlModePaused, State: ControlStateActive, ExpiresAt: ptrTime(now.Add(-time.Minute))}, false},
		{"полное окно покрывает", ScopedControl{ScopeKind: ScopeKindConnector, ScopeRef: "c", Mode: ControlModePaused, State: ControlStateActive, ScheduledStartAt: ptrTime(now.Add(-time.Hour)), ExpiresAt: ptrTime(now.Add(time.Hour))}, true},
	}
	for _, c := range cases {
		if got := c.control.Influencing(now); got != c.want {
			t.Fatalf("%s: Influencing=%v, ожидалось %v", c.name, got, c.want)
		}
	}
}

// ---------------------------------------------------------------------------
// ComputeEffective — the most-restrictive intersection
// ---------------------------------------------------------------------------

func TestComputeEffectiveEmptyIsAccept(t *testing.T) {
	got := ComputeEffective(refTime(), nil)
	if got.Mode != ControlModeAccept || len(got.Influencing) != 0 || got.Reason != "" || got.Corrupted {
		t.Fatalf("пустой маршрут = %v, ожидался честный accept без провенанса", got)
	}
}

func TestComputeEffectiveFullyFilteredOutIsAccept(t *testing.T) {
	// Expired, scheduled-future and removed controls all drop out: the
	// effective mode is recomputed over the REMAINING — the idempotent
	// expiry by construction.
	controls := []ScopedControl{
		{ScopeKind: ScopeKindConnector, ScopeRef: "c", Mode: ControlModePaused, State: ControlStateActive, ExpiresAt: ptrTime(refTime().Add(-time.Minute))},
		{ScopeKind: ScopeKindChannel, ScopeRef: "phone", Mode: ControlModeHumanOnly, State: ControlStateActive, ScheduledStartAt: ptrTime(refTime().Add(time.Hour))},
		{ScopeKind: ScopeKindTenant, ScopeRef: "t", Mode: ControlModeIntakeOnly, State: ControlStateRemoved},
	}
	got := ComputeEffective(refTime(), controls)
	if got.Mode != ControlModeAccept || len(got.Influencing) != 0 {
		t.Fatalf("полностью отфильтрованный маршрут = %v, ожидался accept", got)
	}
}

func TestComputeEffectiveSingleControl(t *testing.T) {
	got := ComputeEffective(refTime(), []ScopedControl{activeControl(ScopeKindConnector, "c", ControlModeHumanOnly, "инцидент поддержки")})
	if got.Mode != ControlModeHumanOnly || len(got.Influencing) != 1 || got.Reason != "инцидент поддержки" {
		t.Fatalf("единственный контроль = %v", got)
	}
	if len(got.Intersection) != 1 || got.Intersection[0] != ControlModeHumanOnly {
		t.Fatalf("пересечение единственного контроля: %v", got.Intersection)
	}
}

func TestComputeEffectivePausedWinsOverAccept(t *testing.T) {
	// AC-3 probe: tenant=paused + connector=accept → paused; the
	// narrower permission (accept on the connector) does NOT cancel the
	// wider blocking (paused on the tenant).
	tenant := ScopedControl{ScopeKind: ScopeKindTenant, ScopeRef: "t", Mode: ControlModePaused, Reason: "аварийный останов тенента", State: ControlStateActive}
	connector := ScopedControl{ScopeKind: ScopeKindConnector, ScopeRef: "c", Mode: ControlModeAccept, Reason: "точечно разрешено", State: ControlStateActive}
	got := ComputeEffective(refTime(), []ScopedControl{tenant, connector})
	if got.Mode != ControlModePaused {
		t.Fatalf("итог = %q, ожидался paused (ранг-максимум)", got.Mode)
	}
	if len(got.Influencing) != 2 {
		t.Fatalf("провенанс обязан нести ОБЕ повлиявшие точки: %v", got.Influencing)
	}
	if got.Reason != "аварийный останов тенента" {
		t.Fatalf("причина = %q, ожидалась причина самого ограничивающего правила", got.Reason)
	}
}

func TestComputeEffectiveHumanOnlyWinsOverIntakeOnly(t *testing.T) {
	// The second direction of the AC-3 ladder probe:
	// connector=human_only + tenant=intake_only/no_promises → human_only.
	connector := ScopedControl{ScopeKind: ScopeKindConnector, ScopeRef: "c", Mode: ControlModeHumanOnly, Reason: "коннектор подозревается", State: ControlStateActive}
	tenant := ScopedControl{ScopeKind: ScopeKindTenant, ScopeRef: "t", Mode: ControlModeIntakeOnly, Reason: "приём без обещаний", State: ControlStateActive}
	forward := ComputeEffective(refTime(), []ScopedControl{tenant, connector})
	backward := ComputeEffective(refTime(), []ScopedControl{connector, tenant})
	if forward.Mode != ControlModeHumanOnly || backward.Mode != ControlModeHumanOnly {
		t.Fatalf("human_only против intake_only: прямой %q, обратный %q — ожидался human_only в обе стороны", forward.Mode, backward.Mode)
	}
}

func TestComputeEffectiveCorruptedReadsPaused(t *testing.T) {
	// A direct-DB garbage probe (the integration matrix writes it with a
	// raw INSERT): the corrupted value READS as paused — fail-closed.
	corrupted := ScopedControl{ScopeKind: ScopeKindConnector, ScopeRef: "c", Mode: "totally_bogus", Reason: "загрязнение", State: ControlStateActive}
	got := ComputeEffective(refTime(), []ScopedControl{corrupted})
	if got.Mode != ControlModePaused {
		t.Fatalf("испорченное значение прочитано как %q, ожидался paused", got.Mode)
	}
	if !got.Corrupted {
		t.Fatal("пометка повреждения не выставлена — чтение обязано быть честным о порче")
	}
	if len(got.Influencing) != 1 {
		t.Fatalf("повреждённая точка обязана остаться в провенансе: %v", got.Influencing)
	}
}

func TestComputeEffectiveCorruptedNeverExpandsAllowance(t *testing.T) {
	// Corruption next to an accept: the damaged value pulls the route
	// DOWN to paused, never UP to accept.
	corrupted := ScopedControl{ScopeKind: ScopeKindConnector, ScopeRef: "c", Mode: "bogus", Reason: "загрязнение", State: ControlStateActive}
	accepting := ScopedControl{ScopeKind: ScopeKindTenant, ScopeRef: "t", Mode: ControlModeAccept, Reason: "разрешено", State: ControlStateActive}
	got := ComputeEffective(refTime(), []ScopedControl{accepting, corrupted})
	if got.Mode != ControlModePaused || !got.Corrupted {
		t.Fatalf("повреждение рядом с accept: итог %q corrupted=%v — ожидался paused с пометкой", got.Mode, got.Corrupted)
	}
}

func TestComputeEffectiveExpiredIdempotent(t *testing.T) {
	// The honest expiry: the control with expires in the past drops out;
	// reading again produces the SAME answer (nothing materializes at
	// the boundary — recomputation only).
	expired := ScopedControl{ScopeKind: ScopeKindConnector, ScopeRef: "c", Mode: ControlModePaused, Reason: "старый останов", State: ControlStateActive, ExpiresAt: ptrTime(refTime().Add(-time.Minute))}
	first := ComputeEffective(refTime(), []ScopedControl{expired})
	second := ComputeEffective(refTime(), []ScopedControl{expired})
	if first.Mode != ControlModeAccept || second.Mode != ControlModeAccept {
		t.Fatalf("истёкший контроль действует: первый %q, второй %q", first.Mode, second.Mode)
	}
	if len(first.Influencing) != 0 || len(second.Influencing) != 0 {
		t.Fatal("истёкший контроль попал в провенанс")
	}
}

func TestComputeEffectiveScheduledFutureNotActing(t *testing.T) {
	future := ScopedControl{ScopeKind: ScopeKindChannel, ScopeRef: "phone", Mode: ControlModePaused, Reason: "запланированный останов", State: ControlStateActive, ScheduledStartAt: ptrTime(refTime().Add(time.Hour))}
	got := ComputeEffective(refTime(), []ScopedControl{future})
	if got.Mode != ControlModeAccept {
		t.Fatalf("запланированный в будущем контроль действует: итог %q", got.Mode)
	}
}

func TestComputeEffectiveDeterministicTieBreak(t *testing.T) {
	// Two equal-rank controls: the winner (and thus the carried reason)
	// must be the same regardless of the input order — the total order
	// is (rank desc, kind order asc, ref asc).
	a := ScopedControl{ScopeKind: ScopeKindChannel, ScopeRef: "phone", Mode: ControlModePaused, Reason: "причина канала", State: ControlStateActive}
	b := ScopedControl{ScopeKind: ScopeKindConnector, ScopeRef: "novofon-sip", Mode: ControlModePaused, Reason: "причина коннектора", State: ControlStateActive}
	first := ComputeEffective(refTime(), []ScopedControl{a, b})
	second := ComputeEffective(refTime(), []ScopedControl{b, a})
	if first.Reason != second.Reason || first.Mode != second.Mode {
		t.Fatalf("равноранговый итог зависит от порядка входа: %q/%q против %q/%q", first.Reason, first.Mode, second.Reason, second.Mode)
	}
	// The connector kind precedes the channel kind in the dictionary —
	// the connector's reason is the deterministic winner.
	if first.Reason != "причина коннектора" {
		t.Fatalf("детерминизм нарушил порядок словаря видов: победила %q", first.Reason)
	}
}

func TestComputeEffectiveIntersectionLadderOrder(t *testing.T) {
	// The intersection words follow the ladder order (accept first,
	// paused last), deduplicated.
	controls := []ScopedControl{
		activeControl(ScopeKindTenant, "t", ControlModePaused, "останов"),
		activeControl(ScopeKindConnector, "c", ControlModeIntakeOnly, "приём"),
		activeControl(ScopeKindChannel, "phone", ControlModeAccept, "разрешение"),
		activeControl(ScopeKindService, "s", ControlModePaused, "ещё останов"),
	}
	got := ComputeEffective(refTime(), controls)
	want := []string{ControlModeAccept, ControlModeIntakeOnly, ControlModePaused}
	if len(got.Intersection) != len(want) {
		t.Fatalf("пересечение %v, ожидалось %v", got.Intersection, want)
	}
	for i, word := range want {
		if got.Intersection[i] != word {
			t.Fatalf("пересечение %v не в порядке лестницы: позиция %d = %q", got.Intersection, i, got.Intersection[i])
		}
	}
}

func TestComputeEffectiveDefensiveCopy(t *testing.T) {
	rows := []ScopedControl{activeControl(ScopeKindConnector, "c", ControlModeHumanOnly, "причина")}
	got := ComputeEffective(refTime(), rows)
	got.Influencing[0].Reason = "изменённая причина"
	if rows[0].Reason != "причина" {
		t.Fatalf("мутация результата тронула строки вызывающего: %q", rows[0].Reason)
	}
}

// ---------------------------------------------------------------------------
// GuardValidateSet — dictionary refusals BEFORE the transaction
// ---------------------------------------------------------------------------

func TestGuardValidateSetGreenBranch(t *testing.T) {
	start := refTime().Add(-time.Minute)
	end := refTime().Add(time.Hour)
	err := GuardValidateSet(ScopeKindConnector, "staffy-synthetic-telephony", "t-tenant-1", ControlModeHumanOnly, "инцидент поддержки", &start, &end)
	if err != nil {
		t.Fatalf("валидная установка отказана: %v", err)
	}
}

func TestGuardValidateSetRefusalsByName(t *testing.T) {
	start := refTime().Add(-time.Minute)
	end := refTime().Add(time.Hour)
	cases := []struct {
		name   string
		kind   string
		ref    string
		mode   string
		reason string
	}{
		{"вид скоупа вне словаря", "zone", "ref", ControlModePaused, "причина"},
		{"режим вне словаря", ScopeKindConnector, "c", "blocked", "причина"},
		{"пустая причина", ScopeKindConnector, "c", ControlModePaused, "  "},
		{"чужой тенент", ScopeKindTenant, "t-tenant-2", ControlModePaused, "причина"},
	}
	for _, c := range cases {
		if err := GuardValidateSet(c.kind, c.ref, "t-tenant-1", c.mode, c.reason, &start, &end); err == nil {
			t.Fatalf("%s: установка принята", c.name)
		}
	}
}

func TestGuardValidateSetWindowSanity(t *testing.T) {
	// [start, expires) must be a non-empty forward window: an empty or
	// inverted window would never influence — refused honestly.
	inverted := refTime().Add(time.Hour)
	earlier := refTime().Add(-time.Hour)
	if err := GuardValidateSet(ScopeKindConnector, "c", "t", ControlModePaused, "причина", &inverted, &earlier); err == nil {
		t.Fatal("перевёрнутое окно принято")
	}
	if err := GuardValidateSet(ScopeKindConnector, "c", "t", ControlModePaused, "причина", &inverted, &inverted); err == nil {
		t.Fatal("пустое окно (start == expires) принято")
	}
	if err := GuardValidateSet(ScopeKindConnector, "c", "t", ControlModePaused, "причина", nil, nil); err != nil {
		t.Fatalf("отсутствие окна (бессрочная точка) отказано: %v", err)
	}
}

// ---------------------------------------------------------------------------
// The typed errors
// ---------------------------------------------------------------------------

func TestRuleErrorUnwrap(t *testing.T) {
	err := ValidReason("")
	if !errors.Is(err, ErrInvalid) {
		t.Fatal("RuleError не разворачивается в ErrInvalid")
	}
}

func TestConflictErrorTyped(t *testing.T) {
	current := ScopedControl{ScopeKind: ScopeKindConnector, ScopeRef: "c", Mode: ControlModePaused, Generation: 3, State: ControlStateActive}
	err := ConflictError{ScopeKind: ScopeKindConnector, ScopeRef: "c", Expected: 2, Current: current}
	if !errors.Is(err, ErrGenerationConflict) {
		t.Fatal("ConflictError не разворачивается в ErrGenerationConflict")
	}
	text := err.Error()
	if !strings.Contains(text, "2") || !strings.Contains(text, "3") {
		t.Fatalf("текст конфликта не называет оба поколения: %q", text)
	}
	var conflict ConflictError
	if !errors.As(err, &conflict) || conflict.Current.Generation != 3 {
		t.Fatalf("материал актуального состояния не извлекается: %v", err)
	}
}
