package service

import (
	"context"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
)

// emailDomainDNSCheck captures the DNS / web fingerprint of an email domain.
// Every field defaults to false; callers decide which signals to reject on.
type emailDomainDNSCheck struct {
	MXRecord         bool   // at least one MX record exists (domain can receive mail)
	MXHost           string // first MX host, lowercase, trailing dot trimmed
	ARecord          bool   // at least one A/AAAA record resolves
	PrivateARecord   bool   // resolved A points to loopback/private/link-local (parked placeholder)
	WebsiteReachable bool   // an HTTP(S) server answered on port 80/443
	MajorProviderMX  bool   // MX host belongs to a mainstream mail provider (gmail/outlook/qq/...)
	DisposableMX     bool   // MX host matches known disposable/temp-mail infrastructure
}

type emailDomainDNSCheckerFunc func(domain string) emailDomainDNSCheck

// registrationEmailDNSChecker is a test seam: unit tests swap in a stub so the
// pure policy logic never depends on live DNS. Production uses checkEmailDomainDNS.
var registrationEmailDNSChecker emailDomainDNSCheckerFunc = checkEmailDomainDNS

const (
	emailDomainDNSLookupTimeout = 3 * time.Second
	emailDomainDNSCacheTTL      = 6 * time.Hour
)

type emailDomainDNSCacheEntry struct {
	at    time.Time
	check emailDomainDNSCheck
}

var emailDomainDNSCache sync.Map // domain (lowercase) -> emailDomainDNSCacheEntry

// checkEmailDomainDNS inspects a normalized, lowercase email domain. It is
// fail-open by design: on any resolver/network error it returns a "pass-everything"
// result and logs, so a DNS outage can never lock out legitimate registrations.
// Results are cached per node for emailDomainDNSCacheTTL (multi-node safe: the
// check is read-only and deterministic; each node computing its own copy is fine).
func checkEmailDomainDNS(domain string) emailDomainDNSCheck {
	domain = strings.ToLower(strings.TrimSpace(domain))
	if domain == "" {
		return emailDomainDNSCheck{MXRecord: true, ARecord: true, WebsiteReachable: true}
	}
	if v, ok := emailDomainDNSCache.Load(domain); ok {
		entry := v.(emailDomainDNSCacheEntry)
		if time.Since(entry.at) < emailDomainDNSCacheTTL {
			return entry.check
		}
	}
	check := probeEmailDomainDNS(domain)
	emailDomainDNSCache.Store(domain, emailDomainDNSCacheEntry{at: time.Now(), check: check})
	return check
}

func probeEmailDomainDNS(domain string) emailDomainDNSCheck {
	check := emailDomainDNSCheck{}
	ctx, cancel := context.WithTimeout(context.Background(), emailDomainDNSLookupTimeout)
	defer cancel()

	// MX: primary signal — without MX, verification mail cannot be delivered.
	mx, mxErr := net.DefaultResolver.LookupMX(ctx, domain)
	if mxErr != nil {
		common.SysLog("registration dns check: LookupMX failed for " + domain + ": " + mxErr.Error())
		// Fail-open: treat the domain as able to receive mail so a resolver hiccup
		// does not block registration.
		check.MXRecord = true
		check.ARecord = true
		check.WebsiteReachable = true
		return check
	}
	check.MXRecord = len(mx) > 0 && strings.TrimSpace(mx[0].Host) != ""
	if check.MXRecord {
		check.MXHost = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(mx[0].Host), "."))
		check.MajorProviderMX = majorMailProviderMX(check.MXHost)
		check.DisposableMX = disposableMailInfraMX(check.MXHost)
	}

	// A/AAAA: parked placeholder domains often resolve to loopback/private ranges.
	ips, aErr := net.DefaultResolver.LookupHost(ctx, domain)
	if aErr != nil {
		common.SysLog("registration dns check: LookupHost failed for " + domain + ": " + aErr.Error())
	} else {
		check.ARecord = len(ips) > 0
		for _, ipStr := range ips {
			ip := net.ParseIP(ipStr)
			if ip == nil {
				continue
			}
			if isPlaceholderIP(ip) {
				check.PrivateARecord = true
				break
			}
		}
	}

	// Website reachability: most bulk temp-mail domains never serve a website,
	// while every mainstream provider does. Pure best-effort signal.
	check.WebsiteReachable = emailDomainWebsiteReachable(domain)
	return check
}

// isPlaceholderIP reports whether an A record points at a loopback/private/
// link-local/CGNAT address — a classic parked or placeholder domain pattern.
func isPlaceholderIP(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsUnspecified() || ip.Equal(net.ParseIP("100.64.0.0"))
}

// majorMailProviderMX reports whether the MX host belongs to a mainstream mail
// provider. Used to exempt otherwise-legit email-only domains (no public website)
// that are nevertheless real mailboxes (corporate Google/Outlook/etc.).
func majorMailProviderMX(mxHost string) bool {
	mxHost = strings.ToLower(strings.TrimSuffix(mxHost, "."))
	for _, suffix := range majorMailProviderMXSuffixes {
		if mxHost == suffix || strings.HasSuffix(mxHost, "."+suffix) {
			return true
		}
	}
	return false
}

var majorMailProviderMXSuffixes = []string{
	"google.com", "googlemail.com",
	"outlook.com", "microsoft.com", "office365.com", "hotmail.com", "live.com",
	"protection.outlook.com",
	"qq.com", "163.com", "126.com", "yeah.net",
	"icloud.com", "yahoo.com", "yahoodns.net", "aol.com",
	"protonmail.com", "protonmail.ch", "zoho.com", "fastmail.fm", "fastmail.com",
	"messagingengine.com", "yandex.com", "yandex.net", "gmx.com", "gmx.net", "gmx.de",
	"mail.ru", "mxhichina.com", "aliyun.com",
}

// disposableMailInfraMX reports whether the MX host is served by known
// disposable/temp-mail infrastructure (substring match on the MX hostname).
func disposableMailInfraMX(mxHost string) bool {
	mxHost = strings.ToLower(strings.TrimSuffix(mxHost, "."))
	for _, fragment := range disposableMailInfraMXFragments {
		if strings.Contains(mxHost, fragment) {
			return true
		}
	}
	return false
}

var disposableMailInfraMXFragments = []string{
	"mail.tm", "fex.plus", "oneb.net", "in.mail.gw", "icodetensor",
	"guerrillamail", "mailinator", "yopmail", "10mail", "mailto.plus", "mailpwr",
	"fexbox", "fexpost", "fextemp", "merepost", "emltmp", "emlpro", "emlhub",
	"trashmail", "sharklasers", "dropmail", "getnada", "tempmail", "mailsac",
	"maildrop", "spam4", "mailnesia", "dispostable", "moakt", "emailondeck",
}

var emailDomainHTTPClient = &http.Client{
	Timeout: 3 * time.Second,
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse // do not follow redirects; one hop is enough
	},
}

// emailDomainWebsiteReachable returns true if an HTTP(S) server answers on the
// domain (any status code counts — a real site exists). TLS failure falls back
// to plain HTTP. No request body or sensitive headers are sent.
func emailDomainWebsiteReachable(domain string) bool {
	for _, scheme := range []string{"https", "http"} {
		ctx, cancel := context.WithTimeout(context.Background(), 2500*time.Millisecond)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, scheme+"://"+domain+"/", nil)
		if err != nil {
			cancel()
			continue
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; FlatkeyDomainCheck/1.0)")
		resp, err := emailDomainHTTPClient.Do(req)
		cancel()
		if err == nil {
			resp.Body.Close()
			return true
		}
	}
	return false
}
