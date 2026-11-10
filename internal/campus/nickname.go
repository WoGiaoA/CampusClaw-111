package campus

import (
	"net/http"
	"strings"
	"unicode"
	"unicode/utf8"
)

func (s *Server) changeNickname(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	if !validCSRF(r, user) {
		problem(w, http.StatusForbidden, "请求校验失败")
		return
	}
	var input struct {
		Nickname *string `json:"nickname"`
	}
	if !decodeJSONRequest(w, r, &input, true) {
		return
	}
	if input.Nickname == nil {
		problem(w, http.StatusBadRequest, "需要提供昵称")
		return
	}
	nickname := strings.TrimSpace(*input.Nickname)
	if utf8.RuneCountInString(nickname) > 24 || strings.ContainsFunc(nickname, unicode.IsControl) {
		problem(w, http.StatusBadRequest, "昵称最多 24 个字符，不能包含控制字符")
		return
	}
	if _, err := s.DB.ExecContext(r.Context(), "UPDATE users SET nickname=? WHERE id=?", nickname, user.ID); err != nil {
		problem(w, http.StatusServiceUnavailable, "服务暂不可用")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"nickname": nickname})
}
