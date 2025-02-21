package main

import (
	"fmt"
	"os/user"
	"path/filepath"

	"gopkg.in/ini.v1"
)

type ProfileConfigs struct {
	IDPCallURI  string
	RoleARN     string
	AWSRegion   string
	DurationSec int
}

func ReadConfig(profile string) (*ProfileConfigs, error) {

	// Load the INI file and produce configs
	usr, _ := user.Current()
	homeDir := usr.HomeDir
	configFilePath := filepath.Join(homeDir, ".aws/config")
	cfg, err := ini.Load(configFilePath)
	if err != nil {
		stderrlogger.Error("Fail to read AWS config file", "error", err)
	}

	def := cfg.Section("default")

	var ps *ini.Section
	if profile != "" {
		ps, err = cfg.GetSection("profile " + profile)
		if err != nil {
			return nil, fmt.Errorf("no 'profile %s' section found in ~/.aws/config", profile)
		}
	} else {
		// Create a blank section just for processing flexibility
		ps = cfg.Section("profile " + profile)
	}

	var idpConf *ini.Section
	idp, err := getDefaultingToParent("saml.idp_config", ps, nil, def)
	if err == nil {
		s := idp.String()
		idpConf, err = cfg.GetSection("saml_idp " + s)
		if err != nil {
			return nil, fmt.Errorf("no 'saml_idp %s' section found in ~/.aws/config (referenced from 'saml.idp_config')", s)
		}
	} else {
		idpConf = nil
	}

	idpUrl, err := getDefaultingToParent("saml.idp_url", ps, idpConf, def)
	if err != nil {
		return nil, err
	}
	roleARN, err := getDefaultingToParent("saml.role_arn", ps, idpConf, def)
	if err != nil {
		return nil, err
	}
	region, err := getDefaultingToParent("region", ps, idpConf, def)
	if err != nil {
		return nil, err
	}
	duration, err := getDefaultingToParent("saml.duration", ps, idpConf, def)
	var durationS int
	if err != nil {
		durationS = 0
	} else {
		durationS = duration.MustInt(0)
	}

	configs := ProfileConfigs{
		IDPCallURI:  idpUrl.String(),
		RoleARN:     roleARN.String(),
		AWSRegion:   region.String(),
		DurationSec: durationS,
	}
	return &configs, nil
}

func getDefaultingToParent(k string, s *ini.Section, idp *ini.Section, def *ini.Section) (*ini.Key, error) {
	if s != nil && s.HasKey(k) {
		stderrlogger.Debug("found key in aws config", "key", k, "section", s.Name())
		return s.Key(k), nil
	} else if idp != nil && idp.HasKey(k) {
		stderrlogger.Debug("found key in aws config", "key", k, "section", idp.Name())
		return idp.Key(k), nil
	} else if def != nil && def.HasKey(k) {
		stderrlogger.Debug("found key in aws config", "key", k, "section", def.Name())
		return def.Key(k), nil
	} else {
		return nil, fmt.Errorf("configuration '%s' not found", k)
	}
}
