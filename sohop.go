package sohop

import (
	"fmt"
	"math/rand"
	"net/http"
	"time"

	"bitbucket.org/davars/sohop/auth"
	"github.com/gorilla/mux"
	"github.com/gorilla/securecookie"
	"github.com/gorilla/sessions"
)

type Config struct {
	Domain          string
	Backends        map[string]backend
	GithubAPI       oauthApp
	AuthorizedOrgID int
}

type backend struct {
	URL         string
	Auth        bool
	HealthCheck string
	WebSocket   string
}

type oauthApp struct {
	ClientID     string
	ClientSecret string
}

var (
	store       = sessions.NewCookieStore(securecookie.GenerateRandomKey(64))
	sessionName = sessionID()
	sessionAge  = 24 * time.Hour
)

func sessionID() string {
	return fmt.Sprintf("_s%d", rand.Uint32())
}

func (c *Config) authorizer() auth.Authorizer {
	return auth.GithubAuth{
		ClientID:     c.GithubAPI.ClientID,
		ClientSecret: c.GithubAPI.ClientSecret,
		OrgID:        c.AuthorizedOrgID,
	}
}

func Handler(conf *Config) (http.Handler, error) {
	store.Options.HttpOnly = true
	store.Options.Secure = true
	store.Options.Domain = conf.Domain
	store.Options.MaxAge = int(sessionAge / time.Second)

	router := mux.NewRouter()
	router.NotFoundHandler = http.HandlerFunc(notFound)

	router.
		Host(fmt.Sprintf("oauth.%s", conf.Domain)).
		Path("/authorized").
		Handler(
		auth.Handler(store, sessionName, conf.authorizer()))

	healthRouter := router.Host(fmt.Sprintf("health.%s", conf.Domain)).Subrouter()
	healthRouter.Path("/check").Handler(health(conf))

	proxyRouter := router.Host(fmt.Sprintf("{subdomain:[a-z]+}.%s", conf.Domain)).Subrouter()
	authentication := auth.Middleware(store, sessionName, conf.authorizer())
	proxy, err := conf.ProxyHandler()
	if err != nil {
		return nil, err
	}

	proxyRouter.MatcherFunc(requiresAuth(conf)).Handler(authentication(proxy))
	proxyRouter.PathPrefix("/").Handler(proxy)

	return logging(router), nil
}
