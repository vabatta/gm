package note

import (
	"regexp"
)

// secrets are credential formats with a fixed prefix or shape. The
// password-in-a-URL pattern skips a password that starts with $, <, { or *,
// because that marks a placeholder such as $PASSWORD.
var secrets = []struct {
	kind string
	re   *regexp.Regexp
}{
	{"private key", regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY`)},
	{"AWS access key", regexp.MustCompile(`\b(?:AKIA|ASIA)[0-9A-Z]{16}\b`)},
	{"GitHub token", regexp.MustCompile(`\b(?:gh[pousr]_[A-Za-z0-9]{20,}|github_pat_[A-Za-z0-9_]{22,})`)},
	{"GitLab token", regexp.MustCompile(`\bglpat-[A-Za-z0-9_-]{20,}`)},
	{"Slack token", regexp.MustCompile(`\bxox[abposr]-[A-Za-z0-9-]{10,}`)},
	{"Google API key", regexp.MustCompile(`\bAIza[0-9A-Za-z_-]{35}`)},
	{"Stripe key", regexp.MustCompile(`\b[rs]k_(?:live|test)_[0-9A-Za-z]{16,}`)},
	{"API key", regexp.MustCompile(`\bsk-(?:ant-|proj-)?[A-Za-z0-9_-]{20,}`)},
	{"JWT", regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{10,}\.eyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}`)},
	{"password in a URL", regexp.MustCompile(`[a-zA-Z][a-zA-Z0-9+.-]*://[^\s:/@]+:[^\s/@$<{*][^\s/@]{2,}@`)},
}

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
		if x.re.MatchString(s) {
			return x.kind, true
		}
	}
	for _, m := range assigned.FindAllStringSubmatch(s, -1) {
		if digit.MatchString(m[1]) && letter.MatchString(m[1]) {
			return "credential assignment", true
		}
	}
	return "", false
}
