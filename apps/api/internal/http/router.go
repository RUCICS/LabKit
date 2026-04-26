package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"

	v2http "labkit.local/apps/api/internal/http/v2"
	"labkit.local/apps/api/internal/http/middleware"
	authsvc "labkit.local/apps/api/internal/service/auth"
	labsvc "labkit.local/apps/api/internal/service/labs"
	websession "labkit.local/apps/api/internal/service/websession"
)

type Router struct {
	handler http.Handler
}

type RouterOption func(*routerConfig)

type routerConfig struct {
	authService        *authsvc.Service
	labsService        LabsService
	leaderboardService LeaderboardService
	submissionsService SubmissionsService
	personalService    PersonalService
	adminService       AdminService
	adminToken         string
	devMode            bool
	webSessionService  *websession.Service
}

func WithAuthService(service *authsvc.Service) RouterOption {
	return func(cfg *routerConfig) {
		cfg.authService = service
	}
}

func WithLabsService(service LabsService) RouterOption {
	return func(cfg *routerConfig) {
		cfg.labsService = service
	}
}

func WithAdminToken(token string) RouterOption {
	return func(cfg *routerConfig) {
		cfg.adminToken = token
	}
}

func WithSubmissionsService(service SubmissionsService) RouterOption {
	return func(cfg *routerConfig) {
		cfg.submissionsService = service
	}
}

func WithLeaderboardService(service LeaderboardService) RouterOption {
	return func(cfg *routerConfig) {
		cfg.leaderboardService = service
	}
}

func WithPersonalService(service PersonalService) RouterOption {
	return func(cfg *routerConfig) {
		cfg.personalService = service
	}
}

func WithAdminService(service AdminService) RouterOption {
	return func(cfg *routerConfig) {
		cfg.adminService = service
	}
}

func WithWebSessionService(service *websession.Service) RouterOption {
	return func(cfg *routerConfig) {
		cfg.webSessionService = service
	}
}

func WithDevMode(enabled bool) RouterOption {
	return func(cfg *routerConfig) {
		cfg.devMode = enabled
	}
}

func NewRouter(options ...RouterOption) *Router {
	cfg := routerConfig{}
	for _, option := range options {
		if option != nil {
			option(&cfg)
		}
	}

	mux := http.NewServeMux()
	authHandler := &AuthHandler{Service: cfg.authService}
	verifyHandler := &DeviceVerifyHandler{Service: cfg.authService, BrowserSessionSecure: !cfg.devMode}
	devDeviceHandler := &DevDeviceHandler{Service: cfg.authService}
	labsHandler := &LabsHandler{Service: cfg.labsService}
	leaderboardHandler := &LeaderboardHandler{Service: cfg.leaderboardService, Personal: cfg.personalService}
	submissionsHandler := &SubmissionsHandler{Service: cfg.submissionsService, Personal: cfg.personalService}
	historyHandler := &HistoryHandler{Service: cfg.personalService}
	profileHandler := &ProfileHandler{Service: cfg.personalService}
	keysHandler := &KeysHandler{Service: cfg.personalService}
	adminHandler := &AdminHandler{Service: cfg.adminService}
	webSessionService := cfg.webSessionService
	if webSessionService == nil {
		webSessionService = websession.NewService()
	}
	webSessionHandler := &WebSessionHandler{
		Personal:             cfg.personalService,
		Service:              webSessionService,
		BrowserSessionSecure: !cfg.devMode,
	}
	adminGuard := adminAuthMiddleware(cfg.adminToken)

	mux.Handle("GET /healthz", &HealthHandler{})

	registerAuthRoutes(mux, authHandler, verifyHandler, webSessionHandler)
	if cfg.devMode {
		registerDevRoutes(mux, devDeviceHandler)
	}

	// v1 API: existing production surface.
	registerV1APIRoutes(mux, "/api", labsHandler, leaderboardHandler, submissionsHandler, historyHandler, profileHandler, keysHandler, adminHandler, adminGuard)
	// Versioned alias for v1, so clients can opt into explicit versioning.
	registerV1APIRoutes(mux, "/api/v1", labsHandler, leaderboardHandler, submissionsHandler, historyHandler, profileHandler, keysHandler, adminHandler, adminGuard)

	// v2 API: new stable JSON contract (lowercase keys), implemented incrementally.
	registerV2Routes(mux, cfg.labsService)

	return &Router{
		handler: middleware.RequestIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			handler, pattern := mux.Handler(req)
			if pattern != "" {
				mux.ServeHTTP(w, req)
				return
			}

			if pattern == "" {
				rr := httptest.NewRecorder()
				handler.ServeHTTP(rr, req)

				if rr.Code == http.StatusMethodNotAllowed {
					if allow := rr.Header().Get("Allow"); allow != "" {
						w.Header().Set("Allow", allow)
					}
					middleware.WriteError(w, req, http.StatusMethodNotAllowed, "method_not_allowed", http.StatusText(http.StatusMethodNotAllowed))
					return
				}

				if rr.Code == http.StatusNotFound {
					middleware.WriteError(w, req, http.StatusNotFound, "not_found", http.StatusText(http.StatusNotFound))
					return
				}

				copyHeaders(w.Header(), rr.Header())
				w.WriteHeader(rr.Code)
				_, _ = w.Write(rr.Body.Bytes())
				return
			}
		})),
	}
}

func registerAuthRoutes(
	mux *http.ServeMux,
	authHandler *AuthHandler,
	verifyHandler *DeviceVerifyHandler,
	webSessionHandler *WebSessionHandler,
) {
	mux.HandleFunc("POST /api/device/authorize", authHandler.CreateDeviceAuthorizationRequest)
	mux.HandleFunc("POST /api/device/poll", authHandler.PollDeviceAuthorizationRequest)
	mux.Handle("GET /api/device/verify", verifyHandler)
	mux.HandleFunc("POST /api/web/session-ticket", webSessionHandler.CreateSessionTicket)
	mux.HandleFunc("GET /auth/session", webSessionHandler.ServeSessionShell)
	mux.HandleFunc("POST /auth/session/exchange", webSessionHandler.ExchangeSessionTicket)
}

func registerDevRoutes(mux *http.ServeMux, devDeviceHandler *DevDeviceHandler) {
	mux.HandleFunc("POST /api/dev/device/bind", devDeviceHandler.BindDevice)
}

func registerV1APIRoutes(
	mux *http.ServeMux,
	apiPrefix string,
	labsHandler *LabsHandler,
	leaderboardHandler *LeaderboardHandler,
	submissionsHandler *SubmissionsHandler,
	historyHandler *HistoryHandler,
	profileHandler *ProfileHandler,
	keysHandler *KeysHandler,
	adminHandler *AdminHandler,
	adminGuard func(http.Handler) http.Handler,
) {
	mux.HandleFunc("GET "+apiPrefix+"/labs", labsHandler.ListLabs)
	mux.HandleFunc("GET "+apiPrefix+"/labs/{labID}", labsHandler.GetLab)
	mux.HandleFunc("GET "+apiPrefix+"/labs/{labID}/board", leaderboardHandler.GetBoard)
	mux.HandleFunc("GET "+apiPrefix+"/labs/{labID}/submit/precheck", submissionsHandler.GetSubmitPrecheck)
	mux.HandleFunc("POST "+apiPrefix+"/labs/{labID}/submit", submissionsHandler.CreateSubmission)
	mux.HandleFunc("POST "+apiPrefix+"/labs/{labID}/submissions", submissionsHandler.CreateSubmission)
	mux.HandleFunc("GET "+apiPrefix+"/labs/{labID}/history", historyHandler.ListHistory)
	mux.HandleFunc("GET "+apiPrefix+"/labs/{labID}/submissions/{submissionID}", historyHandler.GetSubmissionDetail)
	mux.HandleFunc("GET "+apiPrefix+"/profile", profileHandler.GetProfile)
	mux.HandleFunc("PUT "+apiPrefix+"/profile", profileHandler.UpdateProfile)
	mux.HandleFunc("PUT "+apiPrefix+"/labs/{labID}/nickname", profileHandler.UpdateNickname)
	mux.HandleFunc("PUT "+apiPrefix+"/labs/{labID}/track", profileHandler.UpdateTrack)
	mux.HandleFunc("GET "+apiPrefix+"/keys", keysHandler.ListKeys)
	mux.HandleFunc("DELETE "+apiPrefix+"/keys/{keyID}", keysHandler.RevokeKey)
	mux.Handle("POST "+apiPrefix+"/admin/labs", adminGuard(http.HandlerFunc(labsHandler.RegisterLab)))
	mux.Handle("PUT "+apiPrefix+"/admin/labs/{labID}", adminGuard(http.HandlerFunc(labsHandler.UpdateLab)))
	mux.Handle("GET "+apiPrefix+"/admin/labs/{labID}/grades", adminGuard(http.HandlerFunc(adminHandler.ExportGrades)))
	mux.Handle("POST "+apiPrefix+"/admin/labs/{labID}/reeval", adminGuard(http.HandlerFunc(adminHandler.Reevaluate)))
	mux.Handle("GET "+apiPrefix+"/admin/labs/{labID}/queue", adminGuard(http.HandlerFunc(adminHandler.GetQueueStatus)))
}

type v2LabsServiceAdapter struct {
	v1 LabsService
}

func (a v2LabsServiceAdapter) ListPublicLabs(ctx context.Context) ([]labsvc.Lab, error) {
	return a.v1.ListPublicLabs(ctx)
}

func (a v2LabsServiceAdapter) GetPublicLab(ctx context.Context, labID string) (labsvc.Lab, error) {
	return a.v1.GetPublicLab(ctx, labID)
}

func registerV2Routes(mux *http.ServeMux, labsService LabsService) {
	if mux == nil {
		return
	}
	v2Labs := &v2http.LabsHandler{Service: v2LabsServiceAdapter{v1: labsService}}
	mux.HandleFunc("GET /api/v2/labs", v2Labs.ListLabs)
	mux.HandleFunc("GET /api/v2/labs/{labID}", v2Labs.GetLab)
}

func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if r == nil || r.handler == nil {
		http.NotFound(w, req)
		return
	}
	r.handler.ServeHTTP(w, req)
}

func copyHeaders(dst, src http.Header) {
	for key, values := range src {
		dst.Del(key)
		for _, value := range values {
			dst.Add(key, value)
		}
	}
}

func adminAuthMiddleware(token string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if next == nil {
				middleware.WriteError(w, r, http.StatusInternalServerError, "internal_server_error", http.StatusText(http.StatusInternalServerError))
				return
			}
			if strings.TrimSpace(token) == "" {
				middleware.WriteError(w, r, http.StatusUnauthorized, "unauthorized", http.StatusText(http.StatusUnauthorized))
				return
			}
			scheme, credential, ok := strings.Cut(strings.TrimSpace(r.Header.Get("Authorization")), " ")
			if !ok || !strings.EqualFold(scheme, "Bearer") || credential != token {
				middleware.WriteError(w, r, http.StatusUnauthorized, "unauthorized", http.StatusText(http.StatusUnauthorized))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
