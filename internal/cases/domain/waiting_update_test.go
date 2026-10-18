package domain

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/artemiosu/staffy/internal/platform/id"
)

// waitingDistinctID builds a distinct identifier of the unit matrix
// (the uniqueness and anti-disclosure proofs need real distinct
// values — newID is a constant; the id space of the 2.16 matrix never
// collides with the 2.15 correction pool).
func waitingDistinctID(n int) id.UUID {
	value, err := id.Parse(fmt.Sprintf("019b3f6a-2000-7000-8000-%012d", n))
	if err != nil {
		panic(err)
	}
	return value
}

// waitingNow is the deterministic moment of the derivation fixtures.
var waitingNow = time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)

// waitingPromiseOf builds one promise row of the fixtures.
func waitingPromiseOf(word string, no int64, promisedAt time.Time) UpdatePromiseRecord {
	return UpdatePromiseRecord{
		TenantID: waitingDistinctID(1), CaseID: waitingDistinctID(2), PromiseID: waitingDistinctID(3),
		PromiseNo: no, RequestNo: 1,
		DeadlineWord: word, PromisedAt: promisedAt,
		ActorID:    waitingDistinctID(4),
		ObservedAt: waitingNow.Add(-time.Minute), ReceivedAt: waitingNow, CreatedAt: waitingNow,
	}
}

// The Story 2.16 unit matrix (the domain half): the dictionaries of
// the slice (the deadline words with the material unknown honesty, the
// promise state words, the client waiting statuses of the three AC-1
// distinctions), the deadline resolver (the known words compute, the
// unknown word never carries a timestamp), the record guards (the
// word fail-closed, the 2.2 time discipline), the round anchor set
// (the promise-accepting states), the read-time state derivation (the
// last promise speaks, the missed lateness never mutates, the
// superseded former rows, the closed rounds, the control branch of
// the zero view), the client waiting derivation with the
// anti-disclosure probe of AC-1 (no operator names, no epochs, no
// identifiers in the client texts) and the outbox perimeter (the
// identifiers and the deadline word only).

// TestUpdateDictionariesClosed pins the closure of the slice
// dictionaries: the deadline words, the promise state words and the
// client waiting statuses.
func TestUpdateDictionariesClosed(t *testing.T) {
	t.Run("deadline dictionary is exactly the four words in the canonical order", func(t *testing.T) {
		want := []string{UpdateDeadline15m, UpdateDeadline1h, UpdateDeadline4h, UpdateDeadlineUnknown}
		if len(UpdateDeadlines) != 4 || UpdateDeadlines[0] != want[0] || UpdateDeadlines[1] != want[1] || UpdateDeadlines[2] != want[2] || UpdateDeadlines[3] != want[3] {
			t.Fatalf("the deadline dictionary is exactly %v, got %v", want, UpdateDeadlines)
		}
		for _, word := range UpdateDeadlines {
			if !UpdateDeadlineInDictionary(word) {
				t.Fatalf("the deadline word %q must belong to the dictionary", word)
			}
		}
		for _, foreign := range []string{"update_30m", "15m", "unknown", "update", "ack_15m", ""} {
			if UpdateDeadlineInDictionary(foreign) {
				t.Fatalf("the foreign deadline word %q must not belong to the dictionary", foreign)
			}
		}
	})

	t.Run("deadline windows are 15m 1h 4h and the honest zero of unknown", func(t *testing.T) {
		if got := UpdateDeadlineWindow(UpdateDeadline15m); got != 15*time.Minute {
			t.Fatalf("the 15m window is 15 minutes, got %v", got)
		}
		if got := UpdateDeadlineWindow(UpdateDeadline1h); got != time.Hour {
			t.Fatalf("the 1h window is one hour, got %v", got)
		}
		if got := UpdateDeadlineWindow(UpdateDeadline4h); got != 4*time.Hour {
			t.Fatalf("the 4h window is four hours, got %v", got)
		}
		if got := UpdateDeadlineWindow(UpdateDeadlineUnknown); got != 0 {
			t.Fatalf("the unknown word carries no window, got %v", got)
		}
		if got := UpdateDeadlineWindow("update_2h"); got != 0 {
			t.Fatalf("the foreign word carries no window, got %v", got)
		}
	})

	t.Run("deadline texts are non-empty and the unknown text names the honesty", func(t *testing.T) {
		for _, word := range UpdateDeadlines {
			if UpdateDeadlineText(word) == "" {
				t.Fatalf("the deadline word %q carries an honest text", word)
			}
		}
		if got := UpdateDeadlineText(UpdateDeadline1h); got != "Обновление обещано: в течение часа." {
			t.Fatalf("the 1h text is the page line of the matrix, got %q", got)
		}
		if !strings.Contains(UpdateDeadlineText(UpdateDeadlineUnknown), "без выдуманного срока") {
			t.Fatalf("the unknown text names the no-fabricated-deadline honesty (FR-41), got %q", UpdateDeadlineText(UpdateDeadlineUnknown))
		}
	})

	t.Run("promise state dictionary is exactly the five words", func(t *testing.T) {
		want := []string{UpdatePromisePending, UpdatePromiseMissed, UpdatePromiseUnknown, UpdatePromiseSuperseded, UpdatePromiseClosed}
		if len(UpdatePromiseStates) != 5 {
			t.Fatalf("the state dictionary has exactly five words, got %v", UpdatePromiseStates)
		}
		for i, word := range want {
			if UpdatePromiseStates[i] != word {
				t.Fatalf("the state dictionary is exactly %v, got %v", want, UpdatePromiseStates)
			}
			if !UpdatePromiseStateInDictionary(word) {
				t.Fatalf("the state word %q must belong to the dictionary", word)
			}
			if UpdatePromiseStateText(word) == "" {
				t.Fatalf("the state word %q carries an honest text", word)
			}
		}
		for _, foreign := range []string{"late", "expired", "replaced", "done", ""} {
			if UpdatePromiseStateInDictionary(foreign) {
				t.Fatalf("the foreign state word %q must not belong to the dictionary", foreign)
			}
		}
	})

	t.Run("client waiting dictionary is exactly the six words with texts", func(t *testing.T) {
		want := []string{ClientWaitingSearching, ClientWaitingAssigned, ClientWaitingAccepted, ClientWaitingOverdue, ClientWaitingCancelled, ClientWaitingReturned}
		if len(ClientWaitingStatuses) != 6 {
			t.Fatalf("the waiting dictionary has exactly six words, got %v", ClientWaitingStatuses)
		}
		for i, word := range want {
			if ClientWaitingStatuses[i] != word {
				t.Fatalf("the waiting dictionary is exactly %v, got %v", want, ClientWaitingStatuses)
			}
			if !ClientWaitingStatusInDictionary(word) {
				t.Fatalf("the waiting word %q must belong to the dictionary", word)
			}
			if ClientWaitingStatusText(word) == "" {
				t.Fatalf("the waiting word %q carries an honest text", word)
			}
		}
		for _, foreign := range []string{"waiting", "searching", "assigned", ""} {
			if ClientWaitingStatusInDictionary(foreign) {
				t.Fatalf("the foreign waiting word %q must not belong to the dictionary", foreign)
			}
		}
	})
}

// TestUpdatePromiseDeadline pins the material honesty of the promised
// time: the known words compute the server truth (now + window), the
// unknown word NEVER carries a timestamp (the CHECK pair unknown ↔
// NULL — the zero time is the NULL of the store).
func TestUpdatePromiseDeadline(t *testing.T) {
	now := waitingNow
	cases := []struct {
		word   string
		window time.Duration
	}{
		{UpdateDeadline15m, 15 * time.Minute},
		{UpdateDeadline1h, time.Hour},
		{UpdateDeadline4h, 4 * time.Hour},
	}
	for _, tc := range cases {
		if got := UpdatePromiseDeadline(tc.word, now); !got.Equal(now.Add(tc.window)) {
			t.Fatalf("the word %q promises now+%v, got %v", tc.word, tc.window, got)
		}
	}
	if got := UpdatePromiseDeadline(UpdateDeadlineUnknown, now); !got.IsZero() {
		t.Fatalf("the unknown word never carries a promised time (the store keeps NULL), got %v", got)
	}
	if got := UpdatePromiseDeadline("update_2h", now); !got.IsZero() {
		t.Fatalf("the foreign word never carries a promised time, got %v", got)
	}
}

// TestGuardPromiseUpdate pins the word gate of the promise write: the
// dictionary is fail-closed, the honest unknown word passes.
func TestGuardPromiseUpdate(t *testing.T) {
	for _, word := range UpdateDeadlines {
		if err := GuardPromiseUpdate(word); err != nil {
			t.Fatalf("the dictionary word %q passes the guard, got %v", word, err)
		}
	}
	for _, foreign := range []string{"update_30m", "", "15m", "завтра"} {
		err := GuardPromiseUpdate(foreign)
		if err == nil {
			t.Fatalf("the foreign word %q refuses the guard", foreign)
		}
		if !strings.Contains(err.Error(), "не входит в словарь") {
			t.Fatalf("the refusal names the dictionary honestly, got %q", err.Error())
		}
	}
}

// TestGuardUpdatePromiseTimes pins the 2.2 time discipline of the
// promise act: the mandatory times and the 48h provider skew bound.
func TestGuardUpdatePromiseTimes(t *testing.T) {
	observed := waitingNow.Add(-time.Minute)
	received := waitingNow
	if err := GuardUpdatePromiseTimes(observed, received); err != nil {
		t.Fatalf("the honest past observation passes, got %v", err)
	}
	if err := GuardUpdatePromiseTimes(waitingNow, waitingNow); err != nil {
		t.Fatalf("the equal times pass, got %v", err)
	}
	if err := GuardUpdatePromiseTimes(waitingNow.Add(maxProviderClockSkew), waitingNow); err != nil {
		t.Fatalf("the exact 48h boundary passes, got %v", err)
	}
	if err := GuardUpdatePromiseTimes(waitingNow.Add(maxProviderClockSkew+time.Second), waitingNow); err == nil {
		t.Fatalf("the skew beyond 48h refuses")
	}
	if err := GuardUpdatePromiseTimes(time.Time{}, waitingNow); err == nil {
		t.Fatalf("the zero observation refuses")
	}
	if err := GuardUpdatePromiseTimes(waitingNow, time.Time{}); err == nil {
		t.Fatalf("the zero receive refuses")
	}
}

// TestUpdatePromiseRoundAccepts pins the honest anchor set of the
// promise: the open waiting accepts (the fresh ask, the named round
// ahead or late, the accepted case); the declined round refuses (the
// assignment round is consumed — the re-assignment decision belongs to
// the human); the closed rounds refuse.
func TestUpdatePromiseRoundAccepts(t *testing.T) {
	for _, state := range []string{HandoffStateRequested, HandoffStateAssigned, HandoffStateOverdue, HandoffStateAcknowledged} {
		if !UpdatePromiseRoundAccepts(state) {
			t.Fatalf("the round state %q accepts the promise", state)
		}
	}
	for _, state := range []string{HandoffStateDeclined, HandoffStateCancelled, HandoffStateReturned} {
		if UpdatePromiseRoundAccepts(state) {
			t.Fatalf("the round state %q refuses the promise", state)
		}
	}
	if UpdatePromiseRoundAccepts("paused") {
		t.Fatalf("the foreign state refuses the promise")
	}
}

// TestDeriveUpdatePromiseState pins the read-time derivation of the
// promise state: the control branch (no promises is the zero view),
// the last-speaks truth, the missed lateness (the moment decides, the
// row never mutates), the unknown word, the closed rounds and the
// honest ticking of the declined round.
func TestDeriveUpdatePromiseState(t *testing.T) {
	t.Run("no promises is the zero view (the control branch)", func(t *testing.T) {
		view := DeriveUpdatePromiseState(nil, HandoffStateRequested, waitingNow)
		if view.HasPromise || view.State != "" || view.ClientPromiseText() != "" {
			t.Fatalf("the empty history derives the zero view, got %+v", view)
		}
	})

	t.Run("the last promise speaks and the moment decides", func(t *testing.T) {
		promise := waitingPromiseOf(UpdateDeadline1h, 1, waitingNow.Add(time.Hour))
		before := DeriveUpdatePromiseState([]UpdatePromiseRecord{promise}, HandoffStateAssigned, waitingNow)
		if before.State != UpdatePromisePending {
			t.Fatalf("the promise before the deadline is pending, got %q", before.State)
		}
		after := DeriveUpdatePromiseState([]UpdatePromiseRecord{promise}, HandoffStateAssigned, waitingNow.Add(2*time.Hour))
		if after.State != UpdatePromiseMissed {
			t.Fatalf("the promise after the deadline is missed, got %q", after.State)
		}
		if !after.PromisedAt.Equal(promise.PromisedAt) {
			t.Fatalf("the missed derivation never mutates the row: the promised time stays %v", promise.PromisedAt)
		}
		if before.PromiseNo != 1 || after.PromiseNo != 1 || !before.HasPromise || !after.HasPromise {
			t.Fatalf("the view carries the promise number of the last row")
		}
	})

	t.Run("the second promise supersedes the former derivation-only", func(t *testing.T) {
		first := waitingPromiseOf(UpdateDeadline1h, 1, waitingNow.Add(time.Hour))
		second := waitingPromiseOf(UpdateDeadline4h, 2, waitingNow.Add(4*time.Hour))
		promises := []UpdatePromiseRecord{first, second}
		view := DeriveUpdatePromiseState(promises, HandoffStateAssigned, waitingNow)
		if view.PromiseNo != 2 || view.DeadlineWord != UpdateDeadline4h {
			t.Fatalf("the last promise speaks: №2 update_4h, got №%d %q", view.PromiseNo, view.DeadlineWord)
		}
		if view.State != UpdatePromisePending {
			t.Fatalf("the last promise is pending, got %q", view.State)
		}
		if got := DeriveUpdatePromiseRowState(false, first.DeadlineWord, first.PromisedAt, HandoffStateAssigned, waitingNow); got != UpdatePromiseSuperseded {
			t.Fatalf("the former promise derives superseded, got %q", got)
		}
		if first.PromiseNo != 1 || first.DeadlineWord != UpdateDeadline1h {
			t.Fatalf("the former row lives untouched by the supersede")
		}
	})

	t.Run("the unknown word derives unknown with no deadline to miss", func(t *testing.T) {
		promise := waitingPromiseOf(UpdateDeadlineUnknown, 1, time.Time{})
		view := DeriveUpdatePromiseState([]UpdatePromiseRecord{promise}, HandoffStateAcknowledged, waitingNow.Add(100*time.Hour))
		if view.State != UpdatePromiseUnknown {
			t.Fatalf("the unknown word never derives a lateness, got %q", view.State)
		}
		if !view.PromisedAt.IsZero() {
			t.Fatalf("the unknown promise carries no promised time, got %v", view.PromisedAt)
		}
	})

	t.Run("the closed rounds close the promise honestly", func(t *testing.T) {
		promise := waitingPromiseOf(UpdateDeadline1h, 1, waitingNow.Add(-time.Hour))
		for _, state := range []string{HandoffStateCancelled, HandoffStateReturned} {
			view := DeriveUpdatePromiseState([]UpdatePromiseRecord{promise}, state, waitingNow)
			if view.State != UpdatePromiseClosed {
				t.Fatalf("the %s round closes the promise (never a missed word over the ended waiting), got %q", state, view.State)
			}
		}
	})

	t.Run("the accepted and declined rounds keep the promise ticking", func(t *testing.T) {
		promise := waitingPromiseOf(UpdateDeadline15m, 1, waitingNow.Add(-time.Minute))
		for _, state := range []string{HandoffStateAcknowledged, HandoffStateDeclined, HandoffStateOverdue} {
			view := DeriveUpdatePromiseState([]UpdatePromiseRecord{promise}, state, waitingNow)
			if view.State != UpdatePromiseMissed {
				t.Fatalf("the %s round keeps the promise honest (the update is still owed), got %q", state, view.State)
			}
		}
	})

	t.Run("the known word with the zero time stays pending (the defensive branch)", func(t *testing.T) {
		promise := waitingPromiseOf(UpdateDeadline1h, 1, time.Time{})
		view := DeriveUpdatePromiseState([]UpdatePromiseRecord{promise}, HandoffStateAssigned, waitingNow.Add(100*time.Hour))
		if view.State != UpdatePromisePending {
			t.Fatalf("the zero time never derives a lateness (the store CHECK pair owns the invariant), got %q", view.State)
		}
	})

	t.Run("the client promise line composes the honest texts", func(t *testing.T) {
		pending := waitingPromiseOf(UpdateDeadline1h, 1, waitingNow.Add(time.Hour))
		view := DeriveUpdatePromiseState([]UpdatePromiseRecord{pending}, HandoffStateAssigned, waitingNow)
		if !strings.Contains(view.ClientPromiseText(), "Обновление обещано: в течение часа.") {
			t.Fatalf("the promise line carries the deadline word text, got %q", view.ClientPromiseText())
		}
		if !strings.Contains(view.ClientPromiseText(), UpdatePromiseStateText(UpdatePromisePending)) {
			t.Fatalf("the promise line carries the state text, got %q", view.ClientPromiseText())
		}
		missed := waitingPromiseOf(UpdateDeadline15m, 1, waitingNow.Add(-time.Minute))
		view = DeriveUpdatePromiseState([]UpdatePromiseRecord{missed}, HandoffStateAssigned, waitingNow)
		if !strings.Contains(view.ClientPromiseText(), "Обновление просрочено — решения ещё нет.") {
			t.Fatalf("the missed line names the lateness honestly, got %q", view.ClientPromiseText())
		}
		if !strings.Contains(view.ClientPromiseText(), UpdatePromiseMissedNextStepText) {
			t.Fatalf("the missed line carries the honest next step, got %q", view.ClientPromiseText())
		}
	})
}

// TestDeriveClientWaitingStatus pins the three AC-1 distinctions (plus
// the honest overdue) of the client waiting derivation over the 2.13
// round states.
func TestDeriveClientWaitingStatus(t *testing.T) {
	cases := []struct {
		state string
		want  string
	}{
		{HandoffStateRequested, ClientWaitingSearching},
		{HandoffStateDeclined, ClientWaitingSearching},
		{HandoffStateAssigned, ClientWaitingAssigned},
		{HandoffStateOverdue, ClientWaitingOverdue},
		{HandoffStateAcknowledged, ClientWaitingAccepted},
		{HandoffStateCancelled, ClientWaitingCancelled},
		{HandoffStateReturned, ClientWaitingReturned},
	}
	for _, tc := range cases {
		if got := DeriveClientWaitingStatus(tc.state); got != tc.want {
			t.Fatalf("the round state %q derives %q, got %q", tc.state, tc.want, got)
		}
	}
	if got := DeriveClientWaitingStatus("foreign"); got != ClientWaitingSearching {
		t.Fatalf("the foreign state derives the honest searching default, got %q", got)
	}
	if ClientWaitingStatusText(ClientWaitingSearching) != "Ищем специалиста." {
		t.Fatalf("the searching text is the first distinction of AC-1, got %q", ClientWaitingStatusText(ClientWaitingSearching))
	}
	if ClientWaitingStatusText(ClientWaitingAccepted) != "Специалист принял обращение." {
		t.Fatalf("the accepted text is the third distinction of AC-1, got %q", ClientWaitingStatusText(ClientWaitingAccepted))
	}
	if !strings.Contains(ClientWaitingStatusText(ClientWaitingAssigned), "ожидаем принятия") {
		t.Fatalf("the assigned text keeps «назначен ≠ принял» two words, got %q", ClientWaitingStatusText(ClientWaitingAssigned))
	}
}

// TestClientWaitingAntiDisclosure pins the AC-1 perimeter of the client
// texts: the operator names, the epochs and the identifiers NEVER
// enter the client material — every waiting text, every deadline text,
// every state text and the composed promise line stay pure dictionary
// words; the responsible actor of the record never leaks into the
// client line.
func TestClientWaitingAntiDisclosure(t *testing.T) {
	uuidPattern := regexp.MustCompile(`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)
	texts := make([]string, 0, 32)
	for _, word := range ClientWaitingStatuses {
		texts = append(texts, ClientWaitingStatusText(word))
	}
	for _, word := range UpdateDeadlines {
		texts = append(texts, UpdateDeadlineText(word))
	}
	for _, word := range UpdatePromiseStates {
		texts = append(texts, UpdatePromiseStateText(word))
	}
	texts = append(texts, UpdatePromiseMissedNextStepText)
	for _, text := range texts {
		if uuidPattern.MatchString(text) {
			t.Fatalf("the client text never carries an identifier, got %q", text)
		}
		if strings.Contains(text, "epoch") || strings.Contains(text, "@") {
			t.Fatalf("the client text never carries technical material, got %q", text)
		}
	}

	actor := waitingDistinctID(7)
	record := UpdatePromiseRecord{
		TenantID: waitingDistinctID(1), CaseID: waitingDistinctID(2), PromiseID: waitingDistinctID(3),
		PromiseNo: 1, RequestNo: 9,
		DeadlineWord: UpdateDeadline15m, PromisedAt: waitingNow.Add(-time.Minute),
		ActorID:    actor,
		ObservedAt: waitingNow, ReceivedAt: waitingNow, CreatedAt: waitingNow,
	}
	for _, state := range []string{HandoffStateAssigned, HandoffStateAcknowledged, HandoffStateOverdue, HandoffStateRequested} {
		view := DeriveUpdatePromiseState([]UpdatePromiseRecord{record}, state, waitingNow)
		line := view.ClientPromiseText()
		if strings.Contains(line, actor.String()) {
			t.Fatalf("the responsible actor never leaks into the client line, got %q", line)
		}
		if strings.Contains(line, record.PromiseID.String()) || strings.Contains(line, record.CaseID.String()) {
			t.Fatalf("the identifiers never leak into the client line, got %q", line)
		}
		if uuidPattern.MatchString(line) {
			t.Fatalf("the client line never carries an identifier, got %q", line)
		}
	}
}

// TestUpdatePromisedOutboxPerimeter pins the outbox envelope of the
// promise event: the identifiers and the deadline word only — the
// client texts never travel; the dedup key and the version constant.
func TestUpdatePromisedOutboxPerimeter(t *testing.T) {
	payload := UpdatePromisedPayload{
		TenantID: waitingDistinctID(1), CaseID: waitingDistinctID(2),
		RequestNo: 3, PromiseID: waitingDistinctID(4), PromiseNo: 5,
		DeadlineWord: UpdateDeadlineUnknown, ActorID: waitingDistinctID(6),
		OccurredAt: waitingNow,
	}
	raw, err := UpdatePromisedPayloadJSON(payload)
	if err != nil {
		t.Fatalf("the payload marshals, got %v", err)
	}
	var envelope map[string]any
	if err := json.Unmarshal(raw, &envelope); err != nil {
		t.Fatalf("the payload unmarshals, got %v", err)
	}
	wantKeys := []string{
		"tenant_id", "case_id", "request_no", "promise_id",
		"promise_no", "deadline_word", "actor_id", "occurred_at",
	}
	if len(envelope) != len(wantKeys) {
		t.Fatalf("the envelope carries exactly the identifier keys and the word, got %v", envelope)
	}
	for _, key := range wantKeys {
		if _, ok := envelope[key]; !ok {
			t.Fatalf("the envelope carries the key %q, got %v", key, envelope)
		}
	}
	if envelope["deadline_word"] != UpdateDeadlineUnknown {
		t.Fatalf("the deadline word travels as the dictionary word, got %v", envelope["deadline_word"])
	}
	if envelope["promise_no"] != float64(5) || envelope["request_no"] != float64(3) {
		t.Fatalf("the promise and request numbers travel, got %v", envelope)
	}
	for _, forbidden := range []string{"text", "status", "client", "state"} {
		if _, ok := envelope[forbidden]; ok {
			t.Fatalf("the client material never enters the envelope, found the key %q", forbidden)
		}
	}

	caseID := waitingDistinctID(2)
	wantKey := fmt.Sprintf("case.handoff_update_promise:%s:%d:%d", caseID.String(), 3, 5)
	if got := DedupKeyUpdatePromised(caseID, 3, 5); got != wantKey {
		t.Fatalf("the dedup key is %q, got %q", wantKey, got)
	}
	if OutboxEventUpdatePromised != "case.update_promised" {
		t.Fatalf("the outbox type word is case.update_promised, got %q", OutboxEventUpdatePromised)
	}
	if OutboxEventUpdatePromisedV1 != 1 {
		t.Fatalf("the outbox version is 1, got %d", OutboxEventUpdatePromisedV1)
	}
}
