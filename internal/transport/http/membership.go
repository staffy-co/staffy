package httptransport

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/artemiosu/staffy/internal/identity/api"
	"github.com/artemiosu/staffy/internal/identity/domain"
	"github.com/artemiosu/staffy/internal/platform/correlation"
	"github.com/artemiosu/staffy/internal/platform/id"
)

type memberInput struct {
	Email   string            `json:"email"`
	Roles   []domain.Role     `json:"roles"`
	Scopes  []domain.Scope    `json:"scopes"`
	Version int64             `json:"version"`
	Reason  domain.ReasonCode `json:"reason"`
}

func (s *Server) members(w http.ResponseWriter, r *http.Request) (api.MembershipService, domain.Principal, id.UUID, bool) {
	actor, ok := s.principal(w, r)
	if !ok {
		return nil, domain.Principal{}, "", false
	}
	tenant, err := id.Parse(r.PathValue("tenant_id"))
	if err != nil {
		s.notFound(w, r, actor)
		return nil, domain.Principal{}, "", false
	}
	service, ok := s.identity.(api.MembershipService)
	if !ok {
		s.memberProblem(w, r, domain.ErrForbidden)
		return nil, domain.Principal{}, "", false
	}
	return service, actor, tenant, true
}

func (s *Server) memberProblem(w http.ResponseWriter, r *http.Request, err error) {
	status, code, message := http.StatusInternalServerError, "internal_error", "Не удалось выполнить действие."
	switch {
	case errors.Is(err, domain.ErrNotFound):
		status, code, message = http.StatusNotFound, "not_found", "Ресурс не найден."
	case errors.Is(err, domain.ErrForbidden):
		status, code, message = http.StatusForbidden, "forbidden", "Недостаточно полномочий."
	case errors.Is(err, domain.ErrStepUpRequired):
		status, code, message = http.StatusForbidden, "step_up_required", "Для действия требуется повторное подтверждение личности."
	case errors.Is(err, domain.ErrInvalidMembership):
		status, code, message = http.StatusBadRequest, "invalid_membership", "Проверьте роли, области и данные участника."
	case errors.Is(err, domain.ErrConflict):
		status, code, message = http.StatusConflict, "version_conflict", "Данные изменились. Обновите список."
	case errors.Is(err, domain.ErrLastOwner):
		status, code, message = http.StatusConflict, "last_owner", "У компании должен остаться хотя бы один Owner."
	}
	if status == http.StatusForbidden || status == http.StatusNotFound {
		s.logMemberDenial(r, code)
	}
	s.problem(w, r, status, code, message, status >= 500, nil)
}

func (s *Server) logMemberDenial(r *http.Request, code string) {
	actor, err := s.resolver.Resolve(r.Context())
	if err != nil || !actor.ID.Valid() {
		return
	}
	correlationID, _ := correlation.FromContext(r.Context())
	s.logger.WarnContext(r.Context(), "membership access denied", "code", code, "actor_id", actor.ID.String(), "correlation_id", correlationID.String())
}

func decodeMember(w http.ResponseWriter, r *http.Request, out any) error {
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/json" {
		return domain.ErrInvalidMembership
	}
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10))
	d.DisallowUnknownFields()
	if err = d.Decode(out); err != nil {
		return domain.ErrInvalidMembership
	}
	if err = d.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return domain.ErrInvalidMembership
	}
	return nil
}

func (s *Server) listMembersAPI(w http.ResponseWriter, r *http.Request) {
	svc, actor, tenant, ok := s.members(w, r)
	if !ok {
		return
	}
	items, err := svc.ListMembers(r.Context(), actor, tenant)
	if err != nil {
		s.memberProblem(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"members": items})
}

func (s *Server) inviteMemberAPI(w http.ResponseWriter, r *http.Request) {
	svc, actor, tenant, ok := s.members(w, r)
	if !ok {
		return
	}
	var input memberInput
	if err := decodeMember(w, r, &input); err != nil {
		s.memberProblem(w, r, err)
		return
	}
	corr, _ := correlation.FromContext(r.Context())
	m, token, err := svc.InviteMember(r.Context(), api.MembershipCommand{Actor: actor, TenantID: tenant, Email: input.Email, Roles: input.Roles, Scopes: input.Scopes, CorrelationID: corr})
	if err != nil {
		s.memberProblem(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]any{"membership": m, "invitation_url": "/app/invitations/accept?token=" + url.QueryEscape(token)})
}

func (s *Server) acceptInvitationAPI(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.principal(w, r)
	if !ok {
		return
	}
	svc, ok := s.identity.(api.MembershipService)
	if !ok {
		s.memberProblem(w, r, domain.ErrForbidden)
		return
	}
	var input struct {
		Token string `json:"token"`
	}
	if err := decodeMember(w, r, &input); err != nil {
		s.memberProblem(w, r, err)
		return
	}
	corr, _ := correlation.FromContext(r.Context())
	m, err := svc.AcceptInvitation(r.Context(), actor, input.Token, corr)
	if err != nil {
		s.memberProblem(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(m)
}

func (s *Server) changeMemberAPI(w http.ResponseWriter, r *http.Request) {
	svc, actor, tenant, ok := s.members(w, r)
	if !ok {
		return
	}
	target, err := id.Parse(r.PathValue("principal_id"))
	if err != nil {
		s.notFound(w, r, actor)
		return
	}
	var input memberInput
	if err = decodeMember(w, r, &input); err != nil {
		s.memberProblem(w, r, err)
		return
	}
	corr, _ := correlation.FromContext(r.Context())
	m, err := svc.ChangeMember(r.Context(), api.MembershipCommand{Actor: actor, TenantID: tenant, TargetID: target, Roles: input.Roles, Scopes: input.Scopes, ExpectedVersion: input.Version, Reason: input.Reason, CorrelationID: corr})
	if err != nil {
		s.memberProblem(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(m)
}

func (s *Server) revokeMemberAPI(w http.ResponseWriter, r *http.Request) {
	svc, actor, tenant, ok := s.members(w, r)
	if !ok {
		return
	}
	target, err := id.Parse(r.PathValue("principal_id"))
	if err != nil {
		s.notFound(w, r, actor)
		return
	}
	var input memberInput
	if err = decodeMember(w, r, &input); err != nil {
		s.memberProblem(w, r, err)
		return
	}
	corr, _ := correlation.FromContext(r.Context())
	err = svc.RevokeMember(r.Context(), api.MembershipCommand{Actor: actor, TenantID: tenant, TargetID: target, ExpectedVersion: input.Version, Reason: input.Reason, CorrelationID: corr})
	if err != nil {
		s.memberProblem(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) people(w http.ResponseWriter, r *http.Request) {
	svc, actor, tenant, ok := s.members(w, r)
	if !ok {
		return
	}
	items, err := svc.ListMembers(r.Context(), actor, tenant)
	if err != nil {
		s.memberProblem(w, r, err)
		return
	}
	s.render(w, "people", map[string]any{"TenantID": tenant, "Members": items, "StepUp": actor.StepUpVerified})
}

func (s *Server) inviteMemberForm(w http.ResponseWriter, r *http.Request) {
	svc, actor, tenant, ok := s.members(w, r)
	if !ok {
		return
	}
	if !validFormOrigin(r) {
		s.memberProblem(w, r, domain.ErrForbidden)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	if err := r.ParseForm(); err != nil {
		s.memberProblem(w, r, domain.ErrInvalidMembership)
		return
	}
	var roles []domain.Role
	for _, v := range r.Form["roles"] {
		roles = append(roles, domain.Role(v))
	}
	var scopes []domain.Scope
	for _, v := range r.Form["scopes"] {
		scopes = append(scopes, domain.Scope(v))
	}
	corr, _ := correlation.FromContext(r.Context())
	m, token, err := svc.InviteMember(r.Context(), api.MembershipCommand{Actor: actor, TenantID: tenant, Email: r.FormValue("email"), Roles: roles, Scopes: scopes, CorrelationID: corr})
	items, listErr := svc.ListMembers(r.Context(), actor, tenant)
	if listErr != nil {
		s.memberProblem(w, r, listErr)
		return
	}
	data := map[string]any{"TenantID": tenant, "Members": items, "StepUp": actor.StepUpVerified}
	if err != nil {
		_, code, message := memberError(err)
		if errors.Is(err, domain.ErrForbidden) || errors.Is(err, domain.ErrNotFound) || errors.Is(err, domain.ErrStepUpRequired) {
			s.logMemberDenial(r, code)
		}
		data["Error"] = message
		s.renderStatus(w, memberStatus(err), "people", data)
		return
	}
	data["Notice"] = "Приглашение создано. Скопируйте ссылку и передайте получателю безопасным способом."
	data["InvitationURL"] = "/app/invitations/accept?token=" + url.QueryEscape(token)
	data["InvitedEmail"] = m.Email
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	s.render(w, "people", data)
}

func memberError(err error) (int, string, string) {
	switch {
	case errors.Is(err, domain.ErrStepUpRequired):
		return 403, "step_up_required", "Для этой роли требуется повторное подтверждение личности. Пока провайдер недоступен, назначение заблокировано."
	case errors.Is(err, domain.ErrInvalidMembership):
		return 400, "invalid_membership", "Проверьте email, роли и области."
	case errors.Is(err, domain.ErrConflict):
		return 409, "conflict", "Данные изменились или участник уже активен. Обновите список."
	case errors.Is(err, domain.ErrLastOwner):
		return 409, "last_owner", "У компании должен остаться хотя бы один Owner."
	case errors.Is(err, domain.ErrNotFound):
		return 404, "not_found", "Участник или пространство недоступны."
	case errors.Is(err, domain.ErrForbidden):
		return 403, "forbidden", "Недостаточно полномочий."
	default:
		return 500, "internal_error", "Не удалось выполнить действие. Повторите позднее."
	}
}
func memberStatus(err error) int { status, _, _ := memberError(err); return status }

func (s *Server) acceptInvitationPage(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.principal(w, r); !ok {
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	s.render(w, "accept_invitation", map[string]any{"Token": r.URL.Query().Get("token")})
}

func (s *Server) acceptInvitationForm(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.principal(w, r)
	if !ok {
		return
	}
	svc, ok := s.identity.(api.MembershipService)
	if !ok {
		s.memberProblem(w, r, domain.ErrForbidden)
		return
	}
	if !validFormOrigin(r) {
		s.memberProblem(w, r, domain.ErrForbidden)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	if err := r.ParseForm(); err != nil {
		s.memberProblem(w, r, domain.ErrInvalidMembership)
		return
	}
	corr, _ := correlation.FromContext(r.Context())
	m, err := svc.AcceptInvitation(r.Context(), actor, r.FormValue("token"), corr)
	if err != nil {
		w.Header().Set("Cache-Control", "no-store")
		s.renderStatus(w, http.StatusNotFound, "accept_invitation", map[string]any{"Error": "Приглашение недоступно или истекло."})
		return
	}
	http.Redirect(w, r, "/app/tenants/"+m.TenantID.String(), http.StatusSeeOther)
}

func (s *Server) memberFormCommand(w http.ResponseWriter, r *http.Request, actor domain.Principal, tenant id.UUID) (api.MembershipCommand, error) {
	if !validFormOrigin(r) {
		return api.MembershipCommand{}, domain.ErrForbidden
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	if err := r.ParseForm(); err != nil {
		return api.MembershipCommand{}, domain.ErrInvalidMembership
	}
	target, err := id.Parse(r.FormValue("principal_id"))
	if err != nil {
		return api.MembershipCommand{}, domain.ErrInvalidMembership
	}
	version, err := strconv.ParseInt(r.FormValue("version"), 10, 64)
	if err != nil {
		return api.MembershipCommand{}, domain.ErrInvalidMembership
	}
	roles := []domain.Role{}
	for _, v := range r.Form["roles"] {
		roles = append(roles, domain.Role(v))
	}
	scopes := []domain.Scope{}
	for _, v := range r.Form["scopes"] {
		scopes = append(scopes, domain.Scope(v))
	}
	corr, _ := correlation.FromContext(r.Context())
	return api.MembershipCommand{Actor: actor, TenantID: tenant, TargetID: target, ExpectedVersion: version, Roles: roles, Scopes: scopes, Reason: domain.ReasonCode(r.FormValue("reason")), CorrelationID: corr}, nil
}

func (s *Server) memberFormResult(w http.ResponseWriter, r *http.Request, svc api.MembershipService, actor domain.Principal, tenant id.UUID, actionErr error) {
	items, err := svc.ListMembers(r.Context(), actor, tenant)
	if err != nil {
		s.memberProblem(w, r, err)
		return
	}
	data := map[string]any{"TenantID": tenant, "Members": items, "StepUp": actor.StepUpVerified}
	if actionErr != nil {
		if errors.Is(actionErr, domain.ErrForbidden) || errors.Is(actionErr, domain.ErrNotFound) || errors.Is(actionErr, domain.ErrStepUpRequired) {
			_, code, _ := memberError(actionErr)
			s.logMemberDenial(r, code)
		}
		_, _, message := memberError(actionErr)
		data["Error"] = message
		s.renderStatus(w, memberStatus(actionErr), "people", data)
		return
	}
	data["Notice"] = "Доступ участника обновлён."
	s.render(w, "people", data)
}

func (s *Server) changeMemberForm(w http.ResponseWriter, r *http.Request) {
	svc, actor, tenant, ok := s.members(w, r)
	if !ok {
		return
	}
	command, err := s.memberFormCommand(w, r, actor, tenant)
	if err == nil {
		_, err = svc.ChangeMember(r.Context(), command)
	}
	s.memberFormResult(w, r, svc, actor, tenant, err)
}

func (s *Server) revokeMemberForm(w http.ResponseWriter, r *http.Request) {
	svc, actor, tenant, ok := s.members(w, r)
	if !ok {
		return
	}
	command, err := s.memberFormCommand(w, r, actor, tenant)
	if err == nil && r.FormValue("confirm_revoke") != "yes" {
		err = domain.ErrInvalidMembership
	}
	if err == nil {
		err = svc.RevokeMember(r.Context(), command)
	}
	s.memberFormResult(w, r, svc, actor, tenant, err)
}

func validFormOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return false
	}
	parsed, err := url.Parse(origin)
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.User == nil && parsed.Path == "" && parsed.RawQuery == "" && parsed.Fragment == "" && strings.EqualFold(parsed.Host, r.Host)
}
