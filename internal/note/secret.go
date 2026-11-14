package note

import (
	"regexp"
	"strings"
)

// secrets are credential formats with a fixed prefix or shape. Every match
// of a pattern contains one of its hints, so Secret runs a pattern only when a
// hint is in the text. A substring test is far cheaper than the regular
// expression, and most notes contain no hint at all. The password-in-a-URL
// pattern skips a password that starts with $, <, { or *, because that marks
// a placeholder such as $PASSWORD.
var secrets = []struct {
	kind  string
	hints []string
	re    *regexp.Regexp
}{
	{"private key", []string{"PRIVATE KEY"}, regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY`)},
	{"AWS access key", []string{"AKIA", "ASIA"}, regexp.MustCompile(`\b(?:AKIA|ASIA)[0-9A-Z]{16}\b`)},
	{"GitHub token", []string{"ghp_", "gho_", "ghu_", "ghs_", "ghr_", "github_pat_"}, regexp.MustCompile(`\b(?:gh[pousr]_[A-Za-z0-9]{20,}|github_pat_[A-Za-z0-9_]{22,})`)},
	{"GitLab token", []string{"glpat-"}, regexp.MustCompile(`\bglpat-[A-Za-z0-9_-]{20,}`)},
	{"Slack token", []string{"xox"}, regexp.MustCompile(`\bxox[abposr]-[A-Za-z0-9-]{10,}`)},
	{"Google API key", []string{"AIza"}, regexp.MustCompile(`\bAIza[0-9A-Za-z_-]{35}`)},
	{"Stripe key", []string{"k_live_", "k_test_"}, regexp.MustCompile(`\b[rs]k_(?:live|test)_[0-9A-Za-z]{16,}`)},
	{"API key", []string{"sk-"}, regexp.MustCompile(`\bsk-(?:ant-|proj-)?[A-Za-z0-9_-]{20,}`)},
	{"JWT", []string{"eyJ"}, regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{10,}\.eyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}`)},
	{"password in a URL", []string{"://"}, regexp.MustCompile(`[a-zA-Z][a-zA-Z0-9+.-]*://[^\s:/@]+:[^\s/@$<{*][^\s/@]{2,}@`)},
}

// assignedHints are the lowercase words that every match of assigned contains.
var assignedHints = []string{"key", "secret", "token", "passw", "pwd", "credential"}

// assigned matches a secret-like name given a value of 12 or more characters,
// such as API_KEY=a1b2c3d4e5f6. Secret counts a match only when the value holds
// both a digit and a letter, so prose such as "token: refresh_tokens_are_cached"
// passes.
var assigned = regexp.MustCompile(`(?i)(?:api[_-]?key|secret|token|passw(?:or)?d|pwd|credential)s?["']?\s*[:=]\s*["']?([A-Za-z0-9/+_.=-]{12,})`)

var digit, letter = regexp.MustCompile(`[0-9]`), regexp.MustCompile(`[A-Za-z]`)

// Secret reports whether s holds text shaped like a credential, and names the
// kind. Callers print the kind, never the matched text, so gm does not echo a
// secret.
func Secret(s string) (string, bool) {
	for _, x := range secrets {
		if containsAny(s, x.hints) && x.re.MatchString(s) {
			return x.kind, true
		}
	}
	if !containsAny(strings.ToLower(s), assignedHints) {
		return "", false
	}
	for _, m := range assigned.FindAllStringSubmatch(s, -1) {
		if digit.MatchString(m[1]) && letter.MatchString(m[1]) {
			return "credential assignment", true
		}
	}
	return "", false
}

// containsAny reports whether s contains any of subs.
func containsAny(s string, subs []string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
