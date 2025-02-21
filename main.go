package main

import (
	"flag"
	"log/slog"
	"os"
)

var stderrlogger *slog.Logger

func main() {
	port := flag.Int("port", 35002, "Port that you have configured in your IDP for the callback (default 35002)")
	cache := flag.String("cache", "keychain", "Where to cache the credentials:\n\t'keychain': use the system keyring\n\t'file': add an entry in the ~/.aws/credentials file\n\t'none': no cache, used for debugging purposes only")
	d := flag.Bool("debug", false, "Enable verbose logging - WILL INCLUDE CREDENTIALS IN LOG!")
	profileName := flag.Arg(0)
	flag.Parse()

	var loggerOpts *slog.HandlerOptions
	if *d {
		loggerOpts = &slog.HandlerOptions{
			Level: slog.LevelDebug,
		}
	} else {
		loggerOpts = &slog.HandlerOptions{}
	}
	stderrlogger = slog.New(slog.NewTextHandler(os.Stderr, loggerOpts))

	configs, err := readConfig(profileName)
	if err != nil {
		stderrlogger.Error("Could not obtain profile information", "profile", profileName, "error", err)
		panic(err)
	}

	authOptions := &AuthOptions{
		RoleARN:     configs.roleARN,
		Region:      configs.awsRegion,
		ProfileName: profileName,
		IdpCallURI:  configs.idpCallURI,
	}

	var c CredentialStorage[Credentials]
	switch *cache {
	case "keychain":
		c = &KeyringStorage[Credentials]{}
	case "file":
		c = &KeyringStorage[Credentials]{} // TODO fix
	case "none":
		c = &NoopCredentialStorage[Credentials]{}
	default:
		stderrlogger.Error("Invalid value for the cache flag", "value", *cache)
		(flag.Usage)()
		os.Exit(1)
	}
	authenticator := &AWSAuthenticator{
		AuthOptions:       authOptions,
		ServerPort:        *port,
		CredentialStorage: c,
	}
	authenticator.authenticate()
}
