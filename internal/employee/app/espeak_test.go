package app

import (
	"bytes"
	"context"
	"github.com/staffy-co/staffy/internal/employee/domain"
	"os"
	"testing"
)

func TestESpeakNGSyntheticWAV(t *testing.T) {
	binary := os.Getenv("STAFFY_TEST_ESPEAK_NG_BIN")
	if binary == "" {
		t.Skip("установите STAFFY_TEST_ESPEAK_NG_BIN для отдельного dev/test движка")
	}
	e := ESpeakNG{Binary: binary, DataParent: os.Getenv("STAFFY_TEST_ESPEAK_DATA_PATH"), LibraryPath: os.Getenv("STAFFY_TEST_ESPEAK_LIBRARY_PATH")}
	config := domain.VoiceConfig{ProfileID: "ru-neutral-1", Language: "ru", Style: "neutral", Tempo: 100}
	if err := e.Capability(context.Background(), config); err != nil {
		t.Fatal(err)
	}
	audio, mime, err := e.Synthesize(context.Background(), config, "Привет, это тест.", false)
	if err != nil || mime != "audio/wav" || len(audio) <= 44 || !bytes.Equal(audio[:4], []byte("RIFF")) || !bytes.Equal(audio[8:12], []byte("WAVE")) {
		t.Fatalf("wav bytes=%d mime=%q err=%v", len(audio), mime, err)
	}
	if _, _, err = e.Synthesize(context.Background(), config, "<script>", false); err == nil {
		t.Fatal("небезопасный текст принят")
	}
	if _, _, err = e.Synthesize(context.Background(), config, string(bytes.Repeat([]byte("a"), 301)), false); err == nil {
		t.Fatal("текст длиннее лимита принят")
	}
	bad := e
	bad.Binary = "/not-present/espeak-ng"
	if err = bad.Capability(context.Background(), config); err == nil {
		t.Fatal("ложная capability")
	}
}
