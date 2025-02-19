package main

import (
	"os/user"
	"path/filepath"

	"gopkg.in/ini.v1"
)

type ProfileConfigs struct {
	idpCallURI string
	roleARN    string
	awsRegion  string
}

func readConfig(profile string) (ProfileConfigs, error) {

	// Load the INI file and produce configs
	usr, _ := user.Current()
	homeDir := usr.HomeDir
	configFilePath := filepath.Join(homeDir, ".aws/config") // TODO We might want to handle this more dynamically
	cfg, err := ini.Load(configFilePath)
	if err != nil {
		stderrlogger.Error("Fail to read AWS config file", "error", err)
	}
	configs := ProfileConfigs{
		roleARN:    "arn:aws:iam::113053657575:role/assumed-sec-engineer",
		awsRegion:  "ap-southeast-2",
		idpCallURI: "https://accounts.google.com/o/saml2/initsso?forceauthn=false&idpid=C01g1l5do&spid=705835944086",
	}
	// profileName := "qlmgt"
	// // Get the values from the desired section (e.g., "default")
	// defaultSection := cfg.Section("default")
	// baseURL := defaultSection.Key("base_url").String()
	// clientID := defaultSection.Key("client_id").String()
	// tenantID := defaultSection.Key("tenant_id").String()
	_ = cfg
	_ = profile

	return configs, nil
}
