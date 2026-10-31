package connectorrefresolver

import (
	"context"
	"testing"

	"github.com/staffy-co/staffy/internal/platform/id"
)

// TestResolveAnswersPublishedConnectorWords — опубликованные слова живого
// каталога 1-8 резолвятся (любая версия: контроль адресует коннектор, не
// пин версии).
func TestResolveAnswersPublishedConnectorWords(t *testing.T) {
	adapter := New()
	for _, word := range []string{
		"staffy-synthetic-telephony",
		"novofon-sip",
		"mango-office-sip",
		"jivo-chat",
		"abcp-parts",
		"yandex-speechkit",
		"exolve-sip",
	} {
		resolved, err := adapter.Resolve(context.Background(), id.UUID("01994f64-6c00-7000-8000-000000000002"), word)
		if err != nil {
			t.Fatalf("слово %q: инфра-сбоя нет — каталог статичен, got %v", word, err)
		}
		if !resolved {
			t.Fatalf("опубликованный коннектор %q обязан резолвиться в живом каталоге 1-8", word)
		}
	}
}

// TestResolveRefusesUnknownConnectorWords — неизвестное слово и мусор —
// честный отказ (false, никогда не догадка и никогда не ошибка: отказ
// резолюции — это fail-closed ответ, а не инфра-сбой).
func TestResolveRefusesUnknownConnectorWords(t *testing.T) {
	adapter := New()
	for _, word := range []string{
		"unknown-connector",
		"",
		"staffy-synthetic-telephony-2",
		"telephony",
	} {
		resolved, err := adapter.Resolve(context.Background(), id.UUID("01994f64-6c00-7000-8000-000000000002"), word)
		if err != nil {
			t.Fatalf("слово %q: нерезолюция — не ошибка, got %v", word, err)
		}
		if resolved {
			t.Fatalf("слово %q не обязано резолвиться: честный отказ fail-closed", word)
		}
	}
}
