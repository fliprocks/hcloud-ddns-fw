package internal

import (
	"crypto/subtle"
	"fmt"
	"log/slog"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/fliprocks/hcloud-ddns-fw/internal/pubip"
)

// Values for the TRIGGER environment variable (compared case-insensitively).
const (
	TriggerInterval  = "interval"
	TriggerUpdateURL = "updateurl"
)

// Values for the IP_ADDRESS_SRC environment variable (compared case-insensitively).
const (
	IPSourceAPIURL    = "apiurl"
	IPSourceUpdateURL = "updateurl"
)

// RunInterval polls the IP API every interval and runs an update whenever an
// address changed or the previous update failed. It never returns.
func RunInterval(logger *slog.Logger, upd *Updater, ipc *pubip.IPChecker, interval time.Duration) error {
	success := false

	for {
		changed, err := ipc.CheckForChanges()
		if err != nil {
			return err
		}

		if changed || !success {
			logger.Info("Detected changed IP address. Starting update process")
			success = upd.Update()
		}

		logger.Debug("Sleeping", "duration", interval)
		time.Sleep(interval)
	}
}

// IPv6Options describe how to build the host's IPv6 address from the prefix
// parameter of a DDNS update URL.
type IPv6Options struct {
	PrefixLength int    // overrides the length of the prefix parameter if > 0
	PrefixID     string // optional hex subnet ID between prefix length and /64
	InterfaceID  string // lower 64 bits of the host address
}

// UpdateURLServer handles DDNS update requests such as
// /?user=<user>&pass=<pass>&ip4=<ip>&prefix=<ip6 prefix>. Other parameters are
// ignored.
type UpdateURLServer struct {
	logger   *slog.Logger
	upd      *Updater
	ipc      *pubip.IPChecker
	user     string
	pass     string
	ipSource string
	ipv6     IPv6Options

	mu      sync.Mutex
	success bool
}

func NewUpdateURLServer(logger *slog.Logger, upd *Updater, ipc *pubip.IPChecker, user, pass, ipSource string, ipv6 IPv6Options) *UpdateURLServer {
	return &UpdateURLServer{
		logger:   logger,
		upd:      upd,
		ipc:      ipc,
		user:     user,
		pass:     pass,
		ipSource: ipSource,
		ipv6:     ipv6,
	}
}

// InitialUpdate runs one update at startup so changes to config.yaml are
// applied without waiting for the next update URL call. With IP source
// updateUrl it uses the IPs persisted in IP_STATE_DIR and does nothing if none
// are known yet; with apiUrl it fetches the current IPs first.
func (s *UpdateURLServer) InitialUpdate() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.ipSource == IPSourceUpdateURL {
		if s.ipc.IPv4 == "" && s.ipc.IPv6 == "" {
			s.logger.Info("No known IP addresses yet. Skipping initial update, waiting for the first update URL call")
			return
		}
		s.logger.Info("Starting initial update with last known IP addresses", "IPv4", s.ipc.IPv4, "IPv6", s.ipc.IPv6)
	} else {
		if _, err := s.ipc.CheckForChanges(); err != nil {
			s.logger.Error("Skipping initial update, failed to determine IP addresses", "err", err)
			return
		}
		s.logger.Info("Starting initial update")
	}

	s.success = s.upd.Update()
}

// ListenAndServe starts the HTTP server on addr (e.g. ":8080").
func (s *UpdateURLServer) ListenAndServe(addr string) error {
	srv := &http.Server{
		Addr:              addr,
		Handler:           s,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	s.logger.Info("Waiting for DDNS update requests", "addr", addr)
	return srv.ListenAndServe()
}

// ServeHTTP answers with DynDNS-style status words: good, nochg, badauth,
// badparam, 911.
func (s *UpdateURLServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	q := r.URL.Query()

	// Never log the query string, it contains the password.
	if !s.authorized(q.Get("user"), q.Get("pass")) {
		s.logger.Warn("Rejected DDNS update request: invalid credentials", "remote", r.RemoteAddr)
		http.Error(w, "badauth", http.StatusUnauthorized)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	var changed bool
	var err error

	if s.ipSource == IPSourceUpdateURL {
		ipv4, ipv6, perr := s.ipsFromQuery(q)
		if perr != nil {
			s.logger.Error("Invalid DDNS update request", "err", perr)
			http.Error(w, "badparam", http.StatusBadRequest)
			return
		}
		s.logger.Debug("Received IPs from update URL", "IPv4", ipv4, "IPv6", ipv6)
		changed, err = s.ipc.SetFromExternal(ipv4, ipv6)
	} else {
		changed, err = s.ipc.CheckForChanges()
	}

	if err != nil {
		s.logger.Error("Failed to determine IP addresses", "err", err)
		http.Error(w, "911", http.StatusInternalServerError)
		return
	}

	if !changed && s.success {
		s.logger.Info("DDNS update request received, IP addresses unchanged")
		fmt.Fprintln(w, "nochg")
		return
	}

	s.logger.Info("DDNS update request received. Starting update process")
	s.success = s.upd.Update()
	if !s.success {
		http.Error(w, "911", http.StatusInternalServerError)
		return
	}

	fmt.Fprintln(w, "good")
}

func (s *UpdateURLServer) authorized(user, pass string) bool {
	userOK := subtle.ConstantTimeCompare([]byte(user), []byte(s.user)) == 1
	passOK := subtle.ConstantTimeCompare([]byte(pass), []byte(s.pass)) == 1
	return userOK && passOK
}

// ipsFromQuery extracts and validates the addresses from the request. Missing
// values are returned empty so the last known address is kept.
func (s *UpdateURLServer) ipsFromQuery(q url.Values) (ipv4, ipv6 string, err error) {
	if v := strings.TrimSpace(q.Get("ip4")); v != "" {
		a, err := netip.ParseAddr(v)
		if err != nil || !a.Is4() {
			return "", "", fmt.Errorf("invalid ip4 parameter %q", v)
		}
		ipv4 = a.String()
	}

	if prefix := strings.TrimSpace(q.Get("prefix")); prefix != "" {
		if s.ipv6.InterfaceID == "" {
			return "", "", fmt.Errorf("prefix parameter received but IPv6_INTERFACE_ID is not set")
		}
		a, err := pubip.BuildIPv6(prefix, s.ipv6.PrefixLength, s.ipv6.PrefixID, s.ipv6.InterfaceID)
		if err != nil {
			return "", "", err
		}
		ipv6 = a.String()
	}

	return ipv4, ipv6, nil
}
