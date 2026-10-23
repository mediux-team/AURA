package routes_auth

import (
	"aura/config"
	"aura/logging"
	"aura/utils/httpx"
	"net/http"

	"github.com/alexedwards/argon2id"
)

type loginRequest struct {
	Password string `json:"password"`
}

type loginResponse struct {
	Authenticated bool `json:"authenticated"`
}

// Login godoc
// @Summary      Auth Login
// @Description  Authenticate with the admin password and start a browser session. On success, an HttpOnly session cookie is set - the response body does not contain a token. Intended for browser/UI use only; for programmatic access use an API key (see the X-Api-Key header on other endpoints).
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Param        req  body      loginRequest  true  "Login Request"
// @Success      200           {object}  httpx.JSONResponse{data=loginResponse}
// @Failure      500           {object}  httpx.JSONResponse "Internal Server Error"
// @Router       /api/login [post]
func AttemptLogin(w http.ResponseWriter, r *http.Request) {
	ctx, ld := logging.CreateLoggingContext(r.Context(), r.URL.Path)
	logAction := ld.AddAction("User Login", logging.LevelInfo)
	ctx = logging.WithCurrentAction(ctx, logAction)

	var req loginRequest
	var response loginResponse

	if !config.AuthEnabled() {
		httpx.SendResponse(w, ld, "Authentication is disabled")
		return
	}

	if !config.Current.Auth.PasswordEnabled {
		logAction.SetError("Password login is turned off", "Log in with Single Sign-On, or turn on the password in Settings > Authentication", nil)
		httpx.SendResponse(w, ld, response)
		return
	}

	if TokenAuth == nil {
		logAction.SetError("Authentication not configured", "The authentication system is not properly configured", nil)
		httpx.SendResponse(w, ld, response)
		return
	}

	Err := httpx.DecodeRequestBodyToJSON(ctx, r.Body, &req, "Login Request")
	if Err.Message != "" {
		httpx.SendResponse(w, ld, response)
		return
	}

	// Compare password
	ok, err := argon2id.ComparePasswordAndHash(req.Password, config.Current.Auth.Password)
	if err != nil || !ok {
		logAction.SetError("Invalid credentials", "The provided password is incorrect", map[string]any{
			"error": err,
		})
		httpx.SendResponse(w, ld, response)
		return
	}

	if err := IssueSessionCookie(w, r); err != nil {
		logAction.SetError("Failed to start session", "An error occurred while generating the session token", map[string]any{
			"error": err,
		})
		httpx.SendResponse(w, ld, response)
		return
	}

	logAction.AppendResult("session_issued", true)

	response.Authenticated = true
	httpx.SendResponse(w, ld, response)
}
