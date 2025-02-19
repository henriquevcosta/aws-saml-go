package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	_ "embed"

	saml2 "github.com/russellhaering/gosaml2"
)

//go:embed files/authed.html
var htmlSuccess string

var stderrlogger *slog.Logger

func main() {
	port := *flag.Int("port", 35002, "Port that you have configured in your IDP for the callback (default 35002)")
	hostAndPort := fmt.Sprintf("127.0.0.1:%d", port)

	debugMode := *flag.Bool("debug", false, "Enable verbose logging - WILL INCLUDE CREDENTIALS IN LOG!")
	var loggerOpts *slog.HandlerOptions
	if debugMode {
		loggerOpts = &slog.HandlerOptions{
			Level: slog.LevelDebug,
		}
	} else {
		loggerOpts = &slog.HandlerOptions{}
	}
	stderrlogger = slog.New(slog.NewTextHandler(os.Stderr, loggerOpts))

	profileName := flag.Arg(0)

	configs, err := readConfig(profileName)
	if err != nil {
		stderrlogger.Error("Could not obtain profile information", "profile", profileName, "error", err)
		panic(err)
	}

	authOptions := &AuthOptions{
		RoleARN: configs.roleARN,
		Region:  configs.awsRegion,
	}

	serviceProvider := &saml2.SAMLServiceProvider{
		SkipSignatureValidation:     true,
		AssertionConsumerServiceURL: fmt.Sprintf("http://%s", hostAndPort),
		AudienceURI:                 "urn:amazon:webservices:cli",
	}

	mux := http.NewServeMux()
	srv := http.Server{
		Handler: mux,
	}

	writer := &CredentialsProcessOutputWriter{}
	authenticator := &AWSAuthenticator{
		AuthOptions: authOptions,
		Writer:      writer,
	}

	handler := &SAMLResponseHandler{
		server:          &srv,
		serviceProvider: serviceProvider,
		authenticator:   authenticator,
		profileName:     profileName,
	}
	// Start the HTTP server
	mux.HandleFunc("/", handler.handleSAMLResponse)
	stderrlogger.Info("Starting server", "port", port)
	l, err := net.Listen("tcp", hostAndPort)
	if err != nil {
		log.Fatal(err)
	}

	stderrlogger.Info("Opening URL", "url", configs.idpCallURI)
	openbrowser(configs.idpCallURI)

	if err := srv.Serve(l); err != http.ErrServerClosed {
		stderrlogger.Error("Error running server", "error", err)
		panic(err)
	}
}

type SAMLResponseHandler struct {
	server          *http.Server
	serviceProvider *saml2.SAMLServiceProvider
	authenticator   *AWSAuthenticator
	profileName     string
}

func openbrowser(url string) {
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

func (h *SAMLResponseHandler) handleSAMLResponse(rw http.ResponseWriter, req *http.Request) {
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
		assertionInfo, err := h.serviceProvider.RetrieveAssertionInfo(encodedSAMLResponse)
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

		expiration, err := h.authenticator.doAuth(samlData)
		if err != nil {
			http.Error(rw, fmt.Sprintf("Error processing authentication: %s", err), http.StatusInternalServerError)
			return
		}

		var htmlComposed string
		htmlComposed = strings.ReplaceAll(htmlSuccess, "__REPLACED_DATE_HERE__", expiration.Format(time.RFC3339))
		htmlComposed = strings.ReplaceAll(htmlComposed, "__REPLACED_PROFILE_NAME_HERE__", h.profileName)
		rw.Header().Set("Content-type", "text/html")
		rw.Write([]byte(htmlComposed))

	} else {
		http.Error(rw, "Invalid request method", http.StatusMethodNotAllowed)
	}
	go h.server.Shutdown(context.Background())
}
