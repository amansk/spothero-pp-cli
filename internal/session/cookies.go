package session

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"
)

// playwrightStorageState models Playwright storage-state JSON (cookies array).
type playwrightStorageState struct {
	Cookies []struct {
		Name   string `json:"name"`
		Value  string `json:"value"`
		Domain string `json:"domain"`
	} `json:"cookies"`
}

// ParseSessionFile reads a cookies.json written by this CLI. v0.1.8 stored
// {"cookies":{name:value},"source","updated_at"}. Playwright storage state and
// the same file with a cookies array of {name,value,domain,path} are accepted
// too. A map has no domain because the old importer already kept only
// spothero.com cookies.
func ParseSessionFile(data []byte) (*Session, error) {
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(data, &probe); err != nil {
		return nil, fmt.Errorf("parse cookies file: %w", err)
	}
	raw, ok := probe["cookies"]
	if !ok {
		return nil, fmt.Errorf("cookies file has no cookies")
	}
	var meta struct {
		Source    string `json:"source"`
		UpdatedAt string `json:"updated_at"`
	}
	_ = json.Unmarshal(data, &meta)
	var asMap map[string]string
	if err := json.Unmarshal(raw, &asMap); err == nil {
		if len(asMap) == 0 {
			return nil, fmt.Errorf("cookies file has no cookies")
		}
		return &Session{Cookies: asMap, Source: meta.Source, UpdatedAt: meta.UpdatedAt}, nil
	}
	var asList []struct {
		Name   string `json:"name"`
		Value  string `json:"value"`
		Domain string `json:"domain"`
	}
	if err := json.Unmarshal(raw, &asList); err != nil {
		return nil, fmt.Errorf("parse cookies file: cookies is neither a map nor a list")
	}
	cookies := map[string]string{}
	for _, c := range asList {
		if c.Name == "" || c.Value == "" {
			continue
		}
		if c.Domain != "" && !strings.Contains(strings.ToLower(c.Domain), "spothero") {
			continue
		}
		cookies[c.Name] = c.Value
	}
	if len(cookies) == 0 {
		return nil, fmt.Errorf("no spothero.com cookies found in storage state")
	}
	return &Session{Cookies: cookies, Source: meta.Source, UpdatedAt: meta.UpdatedAt}, nil
}

// SafeHeader joins cookies into a Cookie header, dropping pairs net/http would
// reject. One junk value must not discard the session cookie.
func SafeHeader(cookies map[string]string) string {
	if len(cookies) == 0 {
		return ""
	}
	names := make([]string, 0, len(cookies))
	for name := range cookies {
		if name != "AUTH_TOKEN" && name != "csrftoken" {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	ordered := make([]string, 0, len(cookies))
	if _, ok := cookies["AUTH_TOKEN"]; ok {
		ordered = append(ordered, "AUTH_TOKEN")
	}
	if _, ok := cookies["csrftoken"]; ok {
		ordered = append(ordered, "csrftoken")
	}
	ordered = append(ordered, names...)
	parts := make([]string, 0, len(ordered))
	for _, name := range ordered {
		value := cookies[name]
		if value == "" || strings.ContainsAny(name, " ;=,") {
			continue
		}
		pair := name + "=" + value
		if _, err := http.ParseCookie(pair); err != nil {
			continue
		}
		parts = append(parts, pair)
	}
	return strings.Join(parts, "; ")
}

// ParseCookiesFile reads Playwright storage-state JSON or a raw Cookie header file.
func ParseCookiesFile(data []byte, source string) (*Session, error) {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		return nil, fmt.Errorf("cookies file is empty")
	}
	if strings.HasPrefix(trimmed, "{") {
		var state playwrightStorageState
		if err := json.Unmarshal(data, &state); err != nil {
			return nil, fmt.Errorf("parse storage state: %w", err)
		}
		cookies := map[string]string{}
		for _, c := range state.Cookies {
			if !strings.Contains(c.Domain, "spothero") {
				continue
			}
			if c.Name != "" && c.Value != "" {
				cookies[c.Name] = c.Value
			}
		}
		if len(cookies) == 0 {
			return nil, fmt.Errorf("no spothero.com cookies found in storage state")
		}
		return &Session{
			Cookies:   cookies,
			Source:    source,
			UpdatedAt: time.Now().UTC().Format(time.RFC3339),
		}, nil
	}
	return ParseCookieInput(trimmed, source)
}

// ParseCookieInput parses a raw Cookie header string.
func ParseCookieInput(raw, source string) (*Session, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("empty cookie input")
	}
	cookies := map[string]string{}
	for _, part := range strings.Split(raw, ";") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		i := strings.Index(part, "=")
		if i <= 0 {
			continue
		}
		cookies[part[:i]] = part[i+1:]
	}
	if len(cookies) == 0 {
		return nil, fmt.Errorf("no cookies parsed")
	}
	return &Session{
		RawCookieHeader: raw,
		Cookies:         cookies,
		Source:          source,
		UpdatedAt:       time.Now().UTC().Format(time.RFC3339),
	}, nil
}
