package auth

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"

	"bitbucket.org/davars/sohop/store"
	"github.com/gorilla/securecookie"
	"github.com/gorilla/sessions"
	"golang.org/x/oauth2"
)

type Authorizer interface {
	OAuthConfig() *oauth2.Config
	Authorize(code string) (string, error)
}

const (
	authorizedKey  = "auth"
	redirectURLKey = "redir"
	stateKey       = "state"
	userKey        = "user"
)

var (
	ErrMissingCode        = "Missing authorization code."
	ErrMissingState       = "Something unexpected happened.  Please try again."
	ErrUnauthorized       = "Unauthorized."
	ErrMissingRedirectURL = "Not sure where you were going."
)

type authState struct {
	session *sessions.Session
	auth    Authorizer
}

func (s *authState) login(w http.ResponseWriter, r *http.Request) {
	state := string(encodeBase64(securecookie.GenerateRandomKey(30)))
	s.session.Values[stateKey] = state
	err := s.session.Save(r, w)
	if checkServerError(err, w, r) {
		return
	}

	url := s.auth.OAuthConfig().AuthCodeURL(state, oauth2.AccessTypeOffline)
	http.Redirect(w, r, url, http.StatusFound)
}

func (s *authState) authorize(w http.ResponseWriter, r *http.Request) {
	delete(s.session.Values, authorizedKey)
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, ErrMissingCode, http.StatusBadRequest)
		return
	}

	state, ok := s.session.Values[stateKey].(string)
	delete(s.session.Values, stateKey)
	if !ok || state != r.URL.Query().Get("state") {
		checkServerError(errors.New(ErrMissingState), w, r)
		return
	}

	user, err := s.auth.Authorize(code)
	if err != nil {
		http.Error(w, ErrUnauthorized, http.StatusUnauthorized)
		return
	}
	s.session.Values[userKey] = user
	s.session.Values[authorizedKey] = true
	if checkServerError(err, w, r) {
		return
	}

	redirectURL, ok := s.session.Values[redirectURLKey].(string)
	if !ok {
		redirectURL = ""
	}
	delete(s.session.Values, redirectURLKey)
	if redirectURL == "" {
		checkServerError(errors.New(ErrMissingRedirectURL), w, r)
		return
	}

	s.session.Save(r, w)
	http.Redirect(w, r, redirectURL, http.StatusFound)
}

// Handler returns an http.Handler that implements whatever authorization steps are defined by the Authorizer
// (typically exchanging the OAuth2 code for an access token and using the token to identify the user).
func Handler(store store.Namer, auth Authorizer) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session, _ := store.Get(r, store.Name())
		state := &authState{session: session, auth: auth}
		state.authorize(w, r)
	})
}

// Middleware returns a middleware that checks if the requeset has been authorized.  If not, it generates a redirect
// to the configured Authorizer login URL.
func Middleware(store store.Namer, auth Authorizer) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			session, _ := store.Get(r, store.Name())
			if auth, ok := session.Values[authorizedKey].(bool); auth && ok {
				next.ServeHTTP(w, r)
				return
			}

			session.Values[redirectURLKey] = absoluteURL(r)
			state := &authState{session: session, auth: auth}
			state.login(w, r)
			session.Save(r, w)
		})
	}
}

// encodeBase64 returns value encoded in base64
func encodeBase64(value []byte) []byte {
	encoded := make([]byte, base64.URLEncoding.EncodedLen(len(value)))
	base64.URLEncoding.Encode(encoded, value)
	return encoded
}

// absoluteURL reconstructs the absolute URL string for the provided request
func absoluteURL(r *http.Request) string {
	proto := "http"
	if r.TLS != nil {
		proto = "https"
	}
	return fmt.Sprintf("%s://%s%s", proto, r.Host, r.RequestURI)
}

// checkServerError renders an http.StatusInternalServerError if the provided err is not nil
func checkServerError(err error, w http.ResponseWriter, r *http.Request) bool {
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return true
	}
	return false
}
