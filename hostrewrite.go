// Package traefikhostrewrite provides a Traefik middleware that rewrites
// request hosts with configured regular-expression rules.
package traefikhostrewrite

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"regexp"
	"strings"
)

// Config contains the middleware configuration.
type Config struct {
	// Rules are evaluated in order. The first matching rule rewrites the host.
	Rules []Rule `json:"rules,omitempty"`

	// AllowedSuffixes restricts rewritten hosts to one of these suffixes.
	// Leave empty to allow any syntactically valid rewritten host.
	AllowedSuffixes []string `json:"allowedSuffixes,omitempty"`

	// PreserveForwardedHost stores the original public host in X-Forwarded-Host.
	PreserveForwardedHost bool `json:"preserveForwardedHost,omitempty"`

	// OriginalHostHeader stores the original Host value in this request header.
	// Leave empty to avoid adding a separate original-host header.
	OriginalHostHeader string `json:"originalHostHeader,omitempty"`
}

// Rule describes one host rewrite.
type Rule struct {
	// Pattern is matched against the incoming hostname, without the port.
	Pattern string `json:"pattern,omitempty"`

	// Replacement is passed to regexp.ReplaceAllString and supports $1, $name,
	// and the other Go regexp replacement forms.
	Replacement string `json:"replacement,omitempty"`
}

// CreateConfig creates the default plugin configuration.
func CreateConfig() *Config {
	return &Config{}
}

// New creates the middleware.
func New(_ context.Context, next http.Handler, config *Config, _ string) (http.Handler, error) {
	if next == nil {
		return nil, fmt.Errorf("next handler is required")
	}

	if config == nil {
		config = CreateConfig()
	}

	rules, err := compileRules(config.Rules)
	if err != nil {
		return nil, err
	}

	allowedSuffixes := normalizeSuffixes(config.AllowedSuffixes)

	return &hostRewrite{
		next:                  next,
		rules:                 rules,
		allowedSuffixes:       allowedSuffixes,
		preserveForwardedHost: config.PreserveForwardedHost,
		originalHostHeader:    strings.TrimSpace(config.OriginalHostHeader),
	}, nil
}

type hostRewrite struct {
	next                  http.Handler
	rules                 []compiledRule
	allowedSuffixes       []string
	preserveForwardedHost bool
	originalHostHeader    string
}

type compiledRule struct {
	pattern     *regexp.Regexp
	replacement string
}

func compileRules(rules []Rule) ([]compiledRule, error) {
	if len(rules) == 0 {
		return nil, fmt.Errorf("at least one rewrite rule is required")
	}

	compiled := make([]compiledRule, 0, len(rules))
	for i, rule := range rules {
		pattern := strings.TrimSpace(rule.Pattern)
		if pattern == "" {
			return nil, fmt.Errorf("rules[%d].pattern must not be empty", i)
		}

		replacement := strings.TrimSpace(rule.Replacement)
		if replacement == "" {
			return nil, fmt.Errorf("rules[%d].replacement must not be empty", i)
		}

		re, err := regexp.Compile(pattern)
		if err != nil {
			return nil, fmt.Errorf("rules[%d].pattern is invalid: %w", i, err)
		}

		compiled = append(compiled, compiledRule{
			pattern:     re,
			replacement: replacement,
		})
	}

	return compiled, nil
}

func normalizeSuffixes(suffixes []string) []string {
	normalized := make([]string, 0, len(suffixes))
	for _, suffix := range suffixes {
		suffix = strings.ToLower(strings.TrimSpace(suffix))
		if suffix != "" {
			normalized = append(normalized, suffix)
		}
	}
	return normalized
}

func (h *hostRewrite) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	originalHost := req.Host
	host, port := splitHostPort(originalHost)
	normalizedHost := strings.ToLower(host)

	for _, rule := range h.rules {
		if !rule.pattern.MatchString(normalizedHost) {
			continue
		}

		rewrittenHost := rule.pattern.ReplaceAllString(normalizedHost, rule.replacement)
		if !isValidHost(rewrittenHost) || !h.isAllowedHost(rewrittenHost) {
			h.next.ServeHTTP(rw, req)
			return
		}

		if port != "" {
			rewrittenHost = net.JoinHostPort(rewrittenHost, port)
		}

		if h.originalHostHeader != "" {
			req.Header.Set(h.originalHostHeader, originalHost)
		}

		if h.preserveForwardedHost {
			req.Header.Set("X-Forwarded-Host", originalHost)
		}

		req.Host = rewrittenHost
		req.Header.Set("Host", rewrittenHost)
		if req.URL != nil {
			req.URL.Host = rewrittenHost
		}

		h.next.ServeHTTP(rw, req)
		return
	}

	h.next.ServeHTTP(rw, req)
}

func (h *hostRewrite) isAllowedHost(host string) bool {
	if len(h.allowedSuffixes) == 0 {
		return true
	}

	host = strings.ToLower(host)
	for _, suffix := range h.allowedSuffixes {
		if strings.HasSuffix(host, suffix) {
			return true
		}
	}

	return false
}

func isValidHost(host string) bool {
	if host == "" {
		return false
	}
	if strings.ContainsAny(host, " \t\r\n/\\") {
		return false
	}
	return true
}

func splitHostPort(hostport string) (string, string) {
	hostport = strings.TrimSpace(hostport)
	if hostport == "" {
		return "", ""
	}

	host, port, err := net.SplitHostPort(hostport)
	if err == nil {
		return strings.Trim(host, "[]"), port
	}

	lastColon := strings.LastIndex(hostport, ":")
	if lastColon == -1 || strings.Count(hostport, ":") > 1 {
		return strings.Trim(hostport, "[]"), ""
	}

	host = hostport[:lastColon]
	port = hostport[lastColon+1:]
	if host == "" || port == "" {
		return strings.Trim(hostport, "[]"), ""
	}

	return strings.Trim(host, "[]"), port
}
