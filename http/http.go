package http

import (
	"bytes"
	"crypto/tls"
	"github.com/chihaya/bencode"
	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"mika/tracker"
	"net"
	"net/http"
	"strings"
	"time"
)

type AnnounceType string

// Announce types
const (
	STARTED   AnnounceType = "started"
	STOPPED   AnnounceType = "stopped"
	COMPLETED AnnounceType = "completed"
	ANNOUNCE  AnnounceType = ""
)

func parseAnnounceType(t string) AnnounceType {
	switch t {
	case "started":
		return STARTED
	case "stopped":
		return STOPPED
	case "completed":
		return COMPLETED
	default:
		return ANNOUNCE
	}
}

type errorResponse struct {
	FailReason string `bencode:"failure reason"`
}

type trackerErrCode int

const (
	msgInvalidReqType       trackerErrCode = 100
	msgMissingInfoHash      trackerErrCode = 101
	msgMissingPeerID        trackerErrCode = 102
	msgMissingPort          trackerErrCode = 103
	msgInvalidPort          trackerErrCode = 104
	msgInvalidInfoHash      trackerErrCode = 150
	msgInvalidPeerID        trackerErrCode = 151
	msgInvalidNumWant       trackerErrCode = 152
	msgOk                   trackerErrCode = 200
	msgInfoHashNotFound     trackerErrCode = 480
	msgInvalidAuth          trackerErrCode = 490
	msgClientRequestTooFast trackerErrCode = 500
	msgGenericError         trackerErrCode = 900
	msgMalformedRequest     trackerErrCode = 901
	msgQueryParseFail       trackerErrCode = 902
)

var (
	// Error code to message mappings
	responseStringMap = map[trackerErrCode]error{
		msgInvalidReqType:       errors.New("Invalid request type"),
		msgMissingInfoHash:      errors.New("info_hash missing from request"),
		msgMissingPeerID:        errors.New("peer_id missing from request"),
		msgMissingPort:          errors.New("port missing from request"),
		msgInvalidPort:          errors.New("Invalid port"),
		msgInvalidAuth:          errors.New("Invalid passkey supplied"),
		msgInvalidInfoHash:      errors.New("Torrent info hash must be 20 characters"),
		msgInvalidPeerID:        errors.New("Peer ID Invalid"),
		msgInvalidNumWant:       errors.New("num_want invalid"),
		msgInfoHashNotFound:     errors.New("Unknown infohash"),
		msgClientRequestTooFast: errors.New("Slow down there jimmy"),
		msgMalformedRequest:     errors.New("Malformed request"),
		msgGenericError:         errors.New("Generic Error"),
		msgQueryParseFail:       errors.New("Could not parse request"),
	}
)

func TrackerErr(code trackerErrCode) error {
	return responseStringMap[code]
}

// getIP Parses and returns a IP from a string
func getIP(q *query, c *gin.Context) (net.IP, error) {
	ipStr, found := q.Params[paramIP]
	if found {
		ip := net.ParseIP(ipStr)
		if ip != nil {
			return ip.To4(), nil
		}
	}
	// Look for forwarded ip in header then default to remote address
	forwardedIP := c.Request.Header.Get("X-Forwarded-For")
	if forwardedIP != "" {
		ip := net.ParseIP(forwardedIP)
		if ip != nil {
			return ip.To4(), nil
		}
		return ip, nil
	}
	s := strings.Split(c.Request.RemoteAddr, ":")
	ipReq, _ := s[0], s[1]
	ip := net.ParseIP(ipReq)
	if ip != nil {
		return ip.To4(), nil
	}
	return ip, nil
}

// oops will output a bencoded error code to the torrent client using
// a preset message code constant
func oops(ctx *gin.Context, errCode trackerErrCode) {
	msg, exists := responseStringMap[errCode]
	if !exists {
		msg = responseStringMap[msgGenericError]
	}
	ctx.String(int(errCode), responseError(msg.Error()))
	log.Errorf("Error in request from: %s (%d)", ctx.Request.RequestURI, errCode)
}

// handleTrackerErrors is used as the default error handler for tracker requests
// the error is returned to the client as a bencoded error string as defined in the
// bittorrent specs.
func handleTrackerErrors(ctx *gin.Context) {
	// Run request handler
	ctx.Next()

	// Handle any errors recorded
	errorReturned := ctx.Errors.Last()
	if errorReturned != nil {
		meta := errorReturned.JSON().(gin.H)

		status := msgGenericError
		customStatus, found := meta["status"]
		if found {
			status = customStatus.(trackerErrCode)
		}

		// TODO handle private/public errors separately, like sentry output for priv errors
		oops(ctx, status)
	}
}

// responseError generates a bencoded error response for the torrent client to
// parse and display to the user
//
// Note that this function does not generate or support a warning reason, which are rarely if
// ever used.
func responseError(message string) string {
	var buf bytes.Buffer
	encoder := bencode.NewEncoder(&buf)
	if err := encoder.Encode(bencode.Dict{
		"failure reason": message,
	}); err != nil {
		log.Errorf("Failed to encode error response: %s", err)
	}
	return buf.String()
}

// newRouter creates and returns a newly configured router instance using
// the default middleware handlers.
func newRouter() *gin.Engine {
	router := gin.New()
	router.Use(gin.Recovery())
	return router
}

// NewBitTorrentHandler configures a router to handle tracker announce/scrape requests
func NewBitTorrentHandler(tkr *tracker.Tracker) *gin.Engine {
	r := newRouter()
	r.Use(handleTrackerErrors)
	h := BitTorrentHandler{
		t: tkr,
	}
	r.GET("/:passkey/announce", h.announce)
	r.GET("/:passkey/scrape", h.scrape)
	return r
}

// NewAPIHandler configures a router to handle API requests
func NewAPIHandler(tkr *tracker.Tracker) *gin.Engine {
	r := newRouter()
	h := AdminAPI{
		t: tkr,
	}
	r.GET("/tracker/stats", h.Stats)
	r.DELETE("/torrent/:info_hash", h.torrentDelete)
	r.PATCH("/torrent/:info_hash", h.torrentUpdate)
	return r
}

// CreateServer will configure and return a *http.Server suitable for serving requests.
// This should be used over the default ListenAndServe options as they do not set certain
// parameters, notably timeouts, which can negatively effect performance.
func CreateServer(router http.Handler, addr string, useTLS bool) *http.Server {
	var tlsCfg *tls.Config
	if useTLS {
		tlsCfg = &tls.Config{
			MinVersion:               tls.VersionTLS12,
			CurvePreferences:         []tls.CurveID{tls.CurveP521, tls.CurveP384, tls.CurveP256},
			PreferServerCipherSuites: true,
			CipherSuites: []uint16{
				tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
				tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_RSA_WITH_AES_256_CBC_SHA,
			},
		}
	} else {
		tlsCfg = nil
	}
	srv := &http.Server{
		Addr:           addr,
		Handler:        router,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
		TLSConfig:      tlsCfg,
	}
	return srv
}
