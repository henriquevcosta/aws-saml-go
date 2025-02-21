package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"strings"
	"time"

	_ "embed"

	saml2 "github.com/russellhaering/gosaml2"
)

//go:embed files/authed.html
var htmlSuccess string

type SAMLData struct {
	EncodedSAMLResponse string
	AssertionInfo       *saml2.AssertionInfo
}

type SAMLResponseHandler interface {
	handle(SAMLData) (*time.Time, error)
}

type SAMLEndpoint struct {
	ServerPort   int
	ProfileName  string
	IdpCallURI   string
	SAMLAudience string
	Handler      SAMLResponseHandler
	server       *http.Server
	sp           *saml2.SAMLServiceProvider
}

func (h *SAMLEndpoint) runSAMLFlow() error {
	hostAndPort := fmt.Sprintf("127.0.0.1:%d", h.ServerPort)

	h.sp = &saml2.SAMLServiceProvider{
		SkipSignatureValidation:     true,
		AssertionConsumerServiceURL: fmt.Sprintf("http://%s", hostAndPort),
		AudienceURI:                 h.SAMLAudience,
	}

	mux := http.NewServeMux()
	h.server = &http.Server{
		Handler: mux,
	}

	// Start the HTTP server
	mux.HandleFunc("/", h.handleSAMLResponse)
	stderrlogger.Info("Starting server", "port", h.ServerPort)
	l, err := net.Listen("tcp", hostAndPort)
	if err != nil {
		stderrlogger.Error("Could not open listening socket", "error", err)
		return err
	}

	stderrlogger.Info("Opening URL", "url", h.IdpCallURI)
	h.openbrowser(h.IdpCallURI)

	if err := h.server.Serve(l); err != http.ErrServerClosed {
		stderrlogger.Error("Error running server", "error", err)
		return err
	}
	return nil
}

func (h *SAMLEndpoint) handleSAMLResponse(rw http.ResponseWriter, req *http.Request) {
	if req.Method == http.MethodPost {
		// Process the POST request here
		stderrlogger.Debug("Received a POST request")
		err := req.ParseForm()
		if err != nil {
			stderrlogger.Error("Could not parse POST form", "error", err)
			rw.WriteHeader(http.StatusBadRequest)
			return
		}

		encodedSAMLResponse := req.FormValue("SAMLResponse")
		assertionInfo, err := h.sp.RetrieveAssertionInfo(encodedSAMLResponse)
		stderrlogger.Debug("SAML Response received", "SAML-response", encodedSAMLResponse)
		if err != nil {
			stderrlogger.Error("Could not parse SAML Assertion", "error", err)
			rw.WriteHeader(http.StatusForbidden)
			return
		}
		if assertionInfo.WarningInfo.InvalidTime {
			stderrlogger.Error("Invalid SAML assertion time")
			fmt.Println("invalid time")
			rw.WriteHeader(http.StatusForbidden)
			return
		}

		if assertionInfo.WarningInfo.NotInAudience {
			stderrlogger.Error("invalid audience")
			rw.WriteHeader(http.StatusForbidden)
			return
		}

		stderrlogger.Debug("NameID", "NameID", assertionInfo.NameID)
		stderrlogger.Debug("Assertions:")
		for key, val := range assertionInfo.Values {
			stderrlogger.Debug("", "key", key, "val", val)
		}
		stderrlogger.Debug("Warnings:")
		stderrlogger.Debug("", "warningInfo", assertionInfo.WarningInfo)

		samlData := SAMLData{
			EncodedSAMLResponse: encodedSAMLResponse,
			AssertionInfo:       assertionInfo,
		}

		expiration, err := h.Handler.handle(samlData)
		if err != nil {
			http.Error(rw, fmt.Sprintf("Error processing authentication: %s", err), http.StatusInternalServerError)
			return
		}

		var htmlComposed string
		htmlComposed = strings.ReplaceAll(htmlSuccess, "__REPLACED_DATE_HERE__", expiration.Format(time.RFC3339))
		htmlComposed = strings.ReplaceAll(htmlComposed, "__REPLACED_PROFILE_NAME_HERE__", h.ProfileName)
		rw.Header().Set("Content-type", "text/html")
		rw.Write([]byte(htmlComposed))

	} else {
		http.Error(rw, "Invalid request method", http.StatusMethodNotAllowed)
	}
	go h.server.Shutdown(context.Background())
}

func (h *SAMLEndpoint) openbrowser(url string) {
	var err error

	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		err = fmt.Errorf("unsupported platform")
	}
	if err != nil {
		log.Fatal(err)
	}
}
