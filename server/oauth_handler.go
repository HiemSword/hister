// SPDX-License-Identifier: AGPL-3.0-or-later

package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"

	servererrors "github.com/asciimoo/hister/server/errors"
	"github.com/asciimoo/hister/server/model"
	"github.com/asciimoo/hister/server/oauth"

	"github.com/rs/zerolog/log"
)

const (
	oauthStateKey       = "oauth_state"
	oauthRequestTimeout = 10 * time.Second
)

func generateOAuthState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// serveOAuthRedirect starts the OAuth flow for a given provider.
// It stores a random state token in the session and redirects the user to the provider.
func serveOAuthRedirect(c *webContext) {
	if !c.Config.App.UserHandling {
		http.Error(c.Response, "user handling is disabled", http.StatusForbidden)
		return
	}
	providerName := c.Request.URL.Query().Get("provider")
	entry, ok := c.Config.Server.OAuth[providerName]
	if !ok || entry == nil {
		http.Error(c.Response, "unknown oauth provider", http.StatusBadRequest)
		return
	}
	provider, ok := oauth.NewProvider(providerName, entry.AuthURL, entry.TokenURL, entry.UserInfoURL)
	if !ok {
		http.Error(c.Response, "oauth provider not available", http.StatusBadRequest)
		return
	}
	if entry.ConfigurationURL != "" {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
		defer cancel()
		if err := provider.Prepare(ctx, oauth.NewPrepareRequest(entry.ConfigurationURL)); err != nil {
			log.Error().Err(err).Str("provider", providerName).Msg("oauth: failed to prepare provider")
			serve500(c)
			return
		}
	}
	state, err := generateOAuthState()
	if err != nil {
		serve500(c)
		return
	}
	session, err := sessionStore.Get(c.Request, storeName)
	if err != nil {
		serve500(c)
		return
	}
	session.Values[oauthStateKey] = state
	if err := session.Save(c.Request, c.Response); err != nil {
		serve500(c)
		return
	}
	callbackURL := c.Config.BaseURL("/api/oauth/callback") + "?provider=" + providerName
	redirectURL := provider.GetRedirectURL(oauth.NewRedirectURIRequest(entry.ClientID, callbackURL, state, entry.Scopes))
	http.Redirect(c.Response, c.Request, redirectURL, http.StatusFound)
}

// serveOAuthCallback handles the OAuth provider callback.
// It verifies the state, exchanges the code for a token, retrieves user info,
// and either finds or creates a local user account before establishing a session.
func serveOAuthCallback(c *webContext) {
	if !c.Config.App.UserHandling {
		http.Error(c.Response, "user handling is disabled", http.StatusForbidden)
		return
	}
	providerName := c.Request.URL.Query().Get("provider")
	entry, ok := c.Config.Server.OAuth[providerName]
	if !ok || entry == nil {
		http.Error(c.Response, "unknown oauth provider", http.StatusBadRequest)
		return
	}
	code := c.Request.URL.Query().Get("code")
	state := c.Request.URL.Query().Get("state")
	session, err := sessionStore.Get(c.Request, storeName)
	if err != nil {
		serve500(c)
		return
	}
	storedState, ok := session.Values[oauthStateKey].(string)
	if !ok || storedState == "" || storedState != state {
		http.Error(c.Response, "invalid oauth state", http.StatusBadRequest)
		return
	}
	delete(session.Values, oauthStateKey)
	provider, ok := oauth.NewProvider(providerName, entry.AuthURL, entry.TokenURL, entry.UserInfoURL)
	if !ok {
		serve500(c)
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), oauthRequestTimeout)
	defer cancel()
	if entry.ConfigurationURL != "" {
		if err := provider.Prepare(ctx, oauth.NewPrepareRequest(entry.ConfigurationURL)); err != nil {
			log.Error().Err(err).Str("provider", providerName).Msg("oauth: failed to prepare provider")
			serve500(c)
			return
		}
	}
	callbackURL := c.Config.BaseURL("/api/oauth/callback") + "?provider=" + providerName
	tokenResp, err := provider.GetToken(ctx, oauth.NewTokenRequest(
		entry.ClientID, entry.ClientSecret, code, callbackURL,
	))
	if err != nil {
		log.Error().Err(err).Str("provider", providerName).Msg("oauth: failed to exchange token")
		serve500(c)
		return
	}
	defer servererrors.LogCloseBody(tokenResp.Body)
	tokenBody, err := oauth.ReadTokenResponse(tokenResp)
	if err != nil {
		log.Error().Err(err).Msg("oauth: failed to read token response")
		serve500(c)
		return
	}
	userInfo, err := provider.GetUserInfo(ctx, tokenBody)
	if err != nil {
		log.Error().Err(err).Str("provider", providerName).Msg("oauth: failed to get user info")
		serve500(c)
		return
	}
	user, err := model.GetUserByOAuthID(userInfo.UID)
	if err != nil {
		username := userInfo.Username
		if username == "" {
			username = userInfo.Email
		}
		user, err = model.CreateOAuthUser(username, userInfo.UID)
		if err == model.ErrUserAlreadyExists {
			suffix := userInfo.UID
			if len(suffix) > 8 {
				suffix = suffix[len(suffix)-8:]
			}
			user, err = model.CreateOAuthUser(username+"-"+suffix, userInfo.UID)
		}
		if err != nil {
			log.Error().Err(err).Str("provider", providerName).Msg("oauth: failed to create user")
			serve500(c)
			return
		}
	}
	if err := sessionStore.Rotate(session); err != nil {
		serve500(c)
		return
	}
	sessionStore.authenticateUser(session, user.ID)
	if err := session.Save(c.Request, c.Response); err != nil {
		serve500(c)
		return
	}
	http.Redirect(c.Response, c.Request, c.Config.BaseURL("/"), http.StatusFound)
}
