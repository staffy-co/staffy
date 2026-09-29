package app

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/artemiosu/staffy/internal/employee/domain"
)

const maxTestAudioBytes = 2 << 20

// ESpeakNG invokes a separately installed executable only in development/test.
// It receives no credentials or shell, and generated audio is never persisted.
type ESpeakNG struct {
	Binary      string
	DataParent  string
	LibraryPath string
}

func (e ESpeakNG) environment() []string {
	return []string{"PATH=/usr/bin:/bin", "LANG=C.UTF-8", "LD_LIBRARY_PATH=" + e.LibraryPath, "ESPEAK_DATA_PATH=" + e.DataParent}
}

func (e ESpeakNG) Capability(ctx context.Context, c domain.VoiceConfig) error {
	if c.ProfileID != c.Language+"-neutral-1" || (c.Language != "ru" && c.Language != "en") || c.Style != "neutral" || c.Tempo < 80 || c.Tempo > 120 {
		return errors.New("профиль, стиль или темп не поддерживается test-TTS")
	}
	if !filepath.IsAbs(e.Binary) || !filepath.IsAbs(e.DataParent) || !filepath.IsAbs(e.LibraryPath) {
		return errors.New("путь offline test-TTS не настроен")
	}
	info, err := os.Stat(e.Binary)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&0111 == 0 {
		return errors.New("offline test-TTS не установлен")
	}
	if info, err = os.Stat(filepath.Join(e.DataParent, "espeak-ng-data", c.Language+"_dict")); err != nil || !info.Mode().IsRegular() {
		return errors.New("данные языка offline test-TTS отсутствуют")
	}
	probeCtx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	cmd := exec.CommandContext(probeCtx, e.Binary, "--version")
	cmd.Env = e.environment()
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		return errors.New("offline test-TTS недоступен")
	}
	return nil
}

func (e ESpeakNG) Synthesize(ctx context.Context, c domain.VoiceConfig, text string, slow bool) ([]byte, string, error) {
	if len([]rune(text)) == 0 || len([]rune(text)) > 300 || strings.ContainsAny(text, "<>\x00") {
		return nil, "", errors.New("недопустимый текст preview")
	}
	if err := e.Capability(ctx, c); err != nil {
		return nil, "", err
	}
	speed := 175 * c.Tempo / 100
	if slow {
		speed = speed * 3 / 4
	}
	runCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	cmd := exec.CommandContext(runCtx, e.Binary, "--stdout", "--stdin", "-v", c.Language, "-s", strconv.Itoa(speed))
	cmd.Env = e.environment()
	cmd.Stdin = strings.NewReader(text)
	cmd.Stderr = io.Discard
	output := &limitedAudioBuffer{limit: maxTestAudioBytes}
	cmd.Stdout = output
	if err := cmd.Run(); err != nil {
		if runCtx.Err() != nil {
			return nil, "", errors.New("offline test-TTS превысил время ожидания")
		}
		if output.exceeded {
			return nil, "", errors.New("offline test-TTS превысил лимит аудио")
		}
		return nil, "", errors.New("offline test-TTS не создал аудио")
	}
	audio := output.Bytes()
	if len(audio) < 44 || !bytes.Equal(audio[:4], []byte("RIFF")) || !bytes.Equal(audio[8:12], []byte("WAVE")) {
		return nil, "", errors.New("offline test-TTS вернул неверный WAV")
	}
	return audio, "audio/wav", nil
}

type limitedAudioBuffer struct {
	bytes.Buffer
	limit    int
	exceeded bool
}

func (b *limitedAudioBuffer) Write(p []byte) (int, error) {
	if b.Len()+len(p) > b.limit {
		b.exceeded = true
		return 0, errors.New("audio limit")
	}
	return b.Buffer.Write(p)
}
