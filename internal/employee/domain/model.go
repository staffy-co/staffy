package domain

import (
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/staffy-co/staffy/internal/platform/id"
)

var (
	ErrForbidden   = errors.New("employee configuration forbidden")
	ErrNotFound    = errors.New("employee not found")
	ErrInvalid     = errors.New("invalid employee brief")
	ErrConflict    = errors.New("employee version conflict")
	ErrIdempotency = errors.New("employee idempotency conflict")
)

type Brief struct {
	Purpose         string       `json:"purpose"`
	Name            string       `json:"name"`
	Languages       []string     `json:"languages"`
	TimeZone        string       `json:"time_zone"`
	WorkingHours    WorkingHours `json:"working_hours"`
	DesiredChannels []string     `json:"desired_channels"`
	PrimaryTasks    []string     `json:"primary_tasks"`
}

type WorkingHours struct {
	Days  []string `json:"days"`
	Start string   `json:"start"`
	End   string   `json:"end"`
}

var allowedDays = map[string]bool{"mon": true, "tue": true, "wed": true, "thu": true, "fri": true, "sat": true, "sun": true}
var allowedChannels = map[string]bool{"phone": true, "web_chat": true, "telegram": true, "whatsapp": true, "email": true, "max": true, "jivo": true, "avito": true, "mobile_app_chat": true}

func validClock(value string) bool {
	if len(value) != 5 || value[2] != ':' {
		return false
	}
	for _, i := range []int{0, 1, 3, 4} {
		if value[i] < '0' || value[i] > '9' {
			return false
		}
	}
	return value[:2] < "24" && value[3:] < "60"
}
func (h WorkingHours) Valid() bool {
	if len(h.Days) == 0 || len(h.Days) > 7 || !validClock(h.Start) || !validClock(h.End) || h.Start >= h.End {
		return false
	}
	seen := map[string]bool{}
	for _, day := range h.Days {
		if !allowedDays[day] || seen[day] {
			return false
		}
		seen[day] = true
	}
	return true
}

func (b Brief) Valid() bool {
	if !length(b.Purpose, 1, 500) || !length(b.Name, 1, 120) || !b.WorkingHours.Valid() || len(b.Languages) == 0 || len(b.Languages) > 12 || len(b.DesiredChannels) > 12 || len(b.PrimaryTasks) == 0 || len(b.PrimaryTasks) > 20 {
		return false
	}
	if b.TimeZone == "Local" {
		return false
	}
	if _, err := time.LoadLocation(b.TimeZone); err != nil {
		return false
	}
	for _, values := range [][]string{b.Languages, b.DesiredChannels, b.PrimaryTasks} {
		for _, value := range values {
			if !length(value, 1, 160) {
				return false
			}
		}
	}
	seenChannels := map[string]bool{}
	for _, channel := range b.DesiredChannels {
		if !allowedChannels[channel] || seenChannels[channel] {
			return false
		}
		seenChannels[channel] = true
	}
	return true
}

func length(value string, min, max int) bool {
	n := utf8.RuneCountInString(strings.TrimSpace(value))
	return n >= min && n <= max
}

type Draft struct {
	TenantID    id.UUID   `json:"tenant_id"`
	EmployeeID  id.UUID   `json:"employee_id"`
	OwnerID     id.UUID   `json:"owner_id"`
	Status      string    `json:"status"`
	Readiness   string    `json:"readiness"`
	Version     int64     `json:"version"`
	CurrentStep string    `json:"current_step"`
	Brief       Brief     `json:"brief"`
	Pack        *PackPin  `json:"domain_pack,omitempty"`
	ReviewSlots []string  `json:"review_slots"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type PlanStep struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Reason string `json:"reason,omitempty"`
}

func LaunchPlan(d Draft) []PlanStep {
	brief := "incomplete"
	reason := "Заполните и сохраните Brief."
	if d.CurrentStep == "launch_plan" {
		brief, reason = "complete", ""
	}
	packStatus, packReason := "blocked", "Выберите Domain Pack для настройки."
	if d.Pack != nil {
		packStatus, packReason = "incomplete", "Пакет закреплён для настройки; runtime и проверки ещё заблокированы."
		if len(d.ReviewSlots) > 0 {
			packStatus = "requires review"
		}
	}
	dependentStatus := func(markers ...string) string {
		for _, marker := range markers {
			for _, slot := range d.ReviewSlots {
				if slot == marker {
					return "requires review"
				}
			}
		}
		return "blocked"
	}
	return []PlanStep{
		{Name: "Tenant", Status: "complete"},
		{Name: "Brief", Status: brief, Reason: reason},
		{Name: "Domain Pack", Status: packStatus, Reason: packReason},
		{Name: "Connectors", Status: dependentStatus("connectors"), Reason: "Подключения и capabilities ещё не проверены."},
		{Name: "Знания и поведение", Status: dependentStatus("knowledge", "behavior"), Reason: "Настройка пока недоступна."},
		{Name: "Полномочия и fallback", Status: "blocked", Reason: "Правила пока не утверждены."},
		{Name: "Eval и preview", Status: "blocked", Reason: "Проверки ещё не выполнены."},
		{Name: "Публикация", Status: "blocked", Reason: "Публикация возможна только после всех проверок."},
	}
}
