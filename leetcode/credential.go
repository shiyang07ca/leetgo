package leetcode

import (
	"errors"
	"net/http"
	"net/url"

	"github.com/hashicorp/go-hclog"
	"github.com/j178/leetgo/config"
	"github.com/zellyn/kooky"
	_ "github.com/zellyn/kooky/browser/chrome"
)

type CredentialsProvider interface {
	AddCredentials(req *http.Request, c Client) error
	Reset()
}

type nonAuth struct{}

func NonAuth() CredentialsProvider {
	return &nonAuth{}
}

func (n *nonAuth) AddCredentials(req *http.Request, c Client) error {
	return nil
}

func (n *nonAuth) Reset() {}

type cookiesAuth struct {
	LeetcodeSession string
	CsrfToken       string
}

func NewCookiesAuth(session, csrftoken string) CredentialsProvider {
	return &cookiesAuth{LeetcodeSession: session, CsrfToken: csrftoken}
}

func (c *cookiesAuth) AddCredentials(req *http.Request, ct Client) error {
	req.AddCookie(&http.Cookie{Name: "LEETCODE_SESSION", Value: c.LeetcodeSession})
	req.AddCookie(&http.Cookie{Name: "csrftoken", Value: c.CsrfToken})
	req.Header.Add("x-csrftoken", c.CsrfToken)
	return nil
}

func (c *cookiesAuth) Reset() {}

func (c *cookiesAuth) hasAuth() bool {
	return c.LeetcodeSession != "" && c.CsrfToken != ""
}

type passwordAuth struct {
	cookiesAuth
	username string
	password string
}

func NewPasswordAuth(username, passwd string) CredentialsProvider {
	return &passwordAuth{username: username, password: passwd}
}

func (p *passwordAuth) AddCredentials(req *http.Request, c Client) error {
	if !p.hasAuth() {
		hclog.L().Info("logging in with username and password")
		resp, err := c.Login(p.username, p.password)
		if err != nil {
			return err
		}
		cookies := resp.Cookies()
		for _, cookie := range cookies {
			if cookie.Name == "LEETCODE_SESSION" {
				p.LeetcodeSession = cookie.Value
			}
			if cookie.Name == "csrftoken" {
				p.CsrfToken = cookie.Value
			}
		}
		if !p.hasAuth() {
			return errors.New("no credential found")
		}
	}
	return p.cookiesAuth.AddCredentials(req, c)
}

func (p *passwordAuth) Reset() {
	p.LeetcodeSession = ""
	p.CsrfToken = ""
}

type browserAuth struct {
	browsers []string
	cookiesAuth
}

func NewBrowserAuth(browsers ...string) CredentialsProvider {
	return &browserAuth{browsers: browsers}
}

func (b *browserAuth) AddCredentials(req *http.Request, c Client) error {
	if !b.hasAuth() {
		// FIXME 这里的 c 是 cnClient，暂时不知道怎么解决
		u, _ := url.Parse(c.BaseURI())
		domain := u.Host
		hclog.L().Info("reading credentials from browser", "domain", domain)
		session := kooky.ReadCookies(
			kooky.Valid,
			kooky.DomainContains(domain),
			kooky.Name("LEETCODE_SESSION"),
		)
		csrfToken := kooky.ReadCookies(
			kooky.Valid,
			kooky.DomainContains(domain),
			kooky.Name("csrftoken"),
		)
		if len(session) == 0 || len(csrfToken) == 0 {
			return errors.New("no cookie found in browser")
		}
		b.LeetcodeSession = session[0].Value
		b.CsrfToken = csrfToken[0].Value
		hclog.L().Debug("found credentials in browser")
	}

	return b.cookiesAuth.AddCredentials(req, c)
}

func (b *browserAuth) Reset() {
	b.LeetcodeSession = ""
	b.CsrfToken = ""
}

func CredentialsFromConfig() CredentialsProvider {
	cfg := config.Get()
	if cfg.LeetCode.Credentials.ReadFromBrowser != "" {
		return NewBrowserAuth(cfg.LeetCode.Credentials.ReadFromBrowser)
	}
	if cfg.LeetCode.Credentials.Session != "" {
		return NewCookiesAuth(cfg.LeetCode.Credentials.Session, cfg.LeetCode.Credentials.CsrfToken)
	}
	if cfg.LeetCode.Credentials.Username != "" {
		return NewPasswordAuth(cfg.LeetCode.Credentials.Username, cfg.LeetCode.Credentials.Password)
	}
	return NonAuth()
}