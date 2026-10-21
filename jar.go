package tspdnative

import "strings"

// cookieJar is a minimal ordered cookie store. Insertion order is preserved
// so the Cookie header remains browser-stable, and deletions use swap-remove
// to match the reference implementation.
//
// A jar is owned by a single solve and is not safe for concurrent use.
type cookieJar struct {
	items []Cookie
	index map[string]int
}

func newCookieJar() *cookieJar {
	return &cookieJar{index: make(map[string]int, 8)}
}

// Len returns the number of stored cookies.
func (j *cookieJar) Len() int { return len(j.items) }

// Snapshot returns a copy of the jar in insertion order.
func (j *cookieJar) Snapshot() []Cookie {
	out := make([]Cookie, len(j.items))
	copy(out, j.items)
	return out
}

// Set inserts or replaces a cookie while preserving its position.
func (j *cookieJar) Set(name, value string) {
	if position, ok := j.index[name]; ok {
		j.items[position].Value = value
		return
	}
	j.index[name] = len(j.items)
	j.items = append(j.items, Cookie{Name: name, Value: value})
}

// Get returns the value of a cookie and whether it is present.
func (j *cookieJar) Get(name string) (string, bool) {
	position, ok := j.index[name]
	if !ok {
		return "", false
	}
	return j.items[position].Value, true
}

// Remove deletes a cookie using swap-remove.
func (j *cookieJar) Remove(name string) {
	position, ok := j.index[name]
	if !ok {
		return
	}
	last := len(j.items) - 1
	if position != last {
		j.items[position] = j.items[last]
		j.index[j.items[position].Name] = position
	}
	j.items = j.items[:last]
	delete(j.index, name)
}

// Header renders the Cookie request header value.
func (j *cookieJar) Header() string {
	if len(j.items) == 0 {
		return ""
	}
	var builder strings.Builder
	builder.Grow(len(j.items) * 64)
	for i, item := range j.items {
		if i > 0 {
			builder.WriteString("; ")
		}
		builder.WriteString(item.Name)
		builder.WriteByte('=')
		builder.WriteString(item.Value)
	}
	return builder.String()
}

func (j *cookieJar) applySetCookies(headers []string) {
	for _, header := range headers {
		j.applySetCookie(header)
	}
}

// applySetCookie consumes one Set-Cookie header value. A header carrying an
// expiry in 1970 deletes the cookie, mirroring browser behaviour.
func (j *cookieJar) applySetCookie(header string) {
	pair, _, _ := strings.Cut(header, ";")
	name, value, ok := strings.Cut(pair, "=")
	if !ok {
		return
	}
	name = strings.TrimSpace(name)
	value = strings.TrimSpace(value)
	if name == "" {
		return
	}
	if strings.Contains(header, "1970") {
		j.Remove(name)
		return
	}
	j.Set(name, value)
}
