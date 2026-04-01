package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"labkit.local/apps/api/internal/http/middleware"
	websession "labkit.local/apps/api/internal/service/websession"
)

type WebSessionHandler struct {
	Personal             PersonalService
	Service              *websession.Service
	BrowserSessionSecure bool
}

type createWebSessionTicketRequest struct {
	RedirectPath string `json:"redirect_path"`
}

func (h *WebSessionHandler) CreateSessionTicket(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Service == nil {
		middleware.WriteError(w, r, http.StatusInternalServerError, "internal_server_error", http.StatusText(http.StatusInternalServerError))
		return
	}

	body, err := readRequestBody(r)
	if err != nil {
		middleware.WriteError(w, r, http.StatusBadRequest, "invalid_request", "invalid request body")
		return
	}
	var req createWebSessionTicketRequest
	if len(body) > 0 {
		if err := json.NewDecoder(bytes.NewReader(body)).Decode(&req); err != nil {
			middleware.WriteError(w, r, http.StatusBadRequest, "invalid_request", "invalid request body")
			return
		}
	}
	user, ok := authenticatePersonalRequest(w, r, h.Personal, body)
	if !ok {
		return
	}

	result, err := h.Service.CreateTicket(r.Context(), websession.CreateSessionTicketInput{
		UserID:       user.UserID,
		KeyID:        user.KeyID,
		RedirectPath: strings.TrimSpace(req.RedirectPath),
	})
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (h *WebSessionHandler) ServeSessionShell(w http.ResponseWriter, r *http.Request) {
	if h == nil {
		middleware.WriteError(w, r, http.StatusInternalServerError, "internal_server_error", http.StatusText(http.StatusInternalServerError))
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte(webSessionShellHTML))
}

func (h *WebSessionHandler) ExchangeSessionTicket(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Service == nil {
		middleware.WriteError(w, r, http.StatusInternalServerError, "internal_server_error", http.StatusText(http.StatusInternalServerError))
		return
	}

	if err := r.ParseForm(); err != nil {
		middleware.WriteError(w, r, http.StatusBadRequest, "invalid_request", "invalid request body")
		return
	}
	ticket := strings.TrimSpace(r.PostForm.Get("ticket"))
	if ticket == "" {
		middleware.WriteError(w, r, http.StatusBadRequest, "invalid_request", "missing session ticket")
		return
	}
	result, err := h.Service.ConsumeTicket(r.Context(), ticket)
	if err != nil {
		h.writeError(w, r, err)
		return
	}

	w.Header().Set("Cache-Control", "no-store")
	sessionToken, err := issueBrowserSession(result.UserID, result.KeyID, "")
	if err != nil {
		middleware.WriteError(w, r, http.StatusInternalServerError, "internal_server_error", "failed to create browser session")
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     browserSessionCookieName,
		Value:    sessionToken,
		Path:     "/",
		HttpOnly: true,
		Secure:   h.BrowserSessionSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(browserSessionTTL.Seconds()),
	})
	location := result.RedirectPath
	if strings.TrimSpace(location) == "" {
		location = "/auth/confirm"
	}
	if location == "/auth/confirm" {
		location = "/auth/confirm?mode=web-session"
	}
	http.Redirect(w, r, location, http.StatusFound)
}

func (h *WebSessionHandler) writeError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, websession.ErrInvalidTicket):
		middleware.WriteError(w, r, http.StatusBadRequest, "invalid_session_ticket", err.Error())
	case errors.Is(err, websession.ErrInvalidRedirectPath):
		middleware.WriteError(w, r, http.StatusBadRequest, "invalid_request", err.Error())
	case errors.Is(err, websession.ErrSessionTicketLimitReached):
		middleware.WriteError(w, r, http.StatusTooManyRequests, "session_ticket_limit_reached", err.Error())
	default:
		middleware.WriteError(w, r, http.StatusInternalServerError, "internal_server_error", http.StatusText(http.StatusInternalServerError))
	}
}

const webSessionShellHTML = `<!doctype html>
<html lang="en">
  <head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>LabKit session exchange</title>
  </head>
  <body>
    <noscript>JavaScript is required to exchange the session ticket.</noscript>
    <form id="exchange-form" action="/auth/session/exchange" method="post" hidden>
      <input id="ticket" name="ticket" type="hidden">
    </form>
    <script>
      (function () {
        var fragment = window.location.hash ? window.location.hash.slice(1) : "";
        var params = new URLSearchParams(fragment);
        var ticket = params.get("ticket");
        if (!ticket) {
          document.body.textContent = "Missing session ticket.";
          return;
        }
        window.history.replaceState(null, document.title, window.location.pathname + window.location.search);
        document.getElementById("ticket").value = ticket;
        document.getElementById("exchange-form").submit();
      })();
    </script>
  </body>
</html>`
