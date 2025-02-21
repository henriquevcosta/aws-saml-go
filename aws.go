package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sts"
)

type AuthOptions struct {
	SessionDuration int
	RoleARN         string
	Region          string
	ProfileName     string
	IdpCallURI      string
}

type CredentialsOutputWriter interface {
	WriteOutput(*sts.AssumeRoleWithSAMLOutput) error
}

// A credentialProcessResponse is the AWS credentials format that must be
// returned when executing an external credential_process.
// This type is copied from the aws go sdk
type credentialProcessResponse struct {
	// As of this writing, the Version key must be set to 1. This might
	// increment over time as the structure evolves.
	Version int

	// The access key ID that identifies the temporary security credentials.
	AccessKeyID string `json:"AccessKeyId"`

	// The secret access key that can be used to sign requests.
	SecretAccessKey string

	// The token that users must pass to the service API to use the temporary credentials.
	SessionToken string

	// The date on which the current credentials expire.
	Expiration *time.Time
}

type CredentialsProcessOutputWriter struct{}

func (auth *AWSAuthenticator) writeOutput(c *Credentials) error {
	forJSON := &credentialProcessResponse{
		Version:         1,
		AccessKeyID:     c.AccessKeyID,
		SecretAccessKey: c.SecretAccessKey,
		SessionToken:    c.SessionToken,
		Expiration:      c.Expiration,
	}
	jsonOutput, err := json.Marshal(forJSON)
	if err != nil {
		stderrlogger.Error("Could not marshall the credentials")
		return err
	}
	stderrlogger.Debug("Encoded JSON to return", "payload", jsonOutput)
	fmt.Printf("%s", jsonOutput)
	return nil
}

type AWSAuthenticator struct {
	AuthOptions       *AuthOptions
	ServerPort        int
	CredentialStorage CredentialStorage[Credentials]
}

type Credentials struct {
	// The access key ID that identifies the temporary security credentials.
	AccessKeyID string `json:"AccessKeyId"`

	// The secret access key that can be used to sign requests.
	SecretAccessKey string

	// The token that users must pass to the service API to use the temporary credentials.
	SessionToken string

	// The date on which the current credentials expire.
	Expiration *time.Time
}

func (auth *AWSAuthenticator) authenticate() error {
	// test if cached credentials exist, and if they are still valid
	c, found, err := auth.CredentialStorage.GetEntry(auth.AuthOptions.ProfileName)
	if err != nil {
		return err
	} else if found {
		if c.Expiration.After(time.Now()) {
			stderrlogger.Info("Using credential from cache")
			auth.writeOutput(c)
			return nil
		}
	}
	// if not,
	// read INI to get endpoint info

	// launch the server
	s := &SAMLEndpoint{
		SAMLAudience: "urn:amazon:webservices:cli",
		Handler:      auth,
		IdpCallURI:   auth.AuthOptions.IdpCallURI,
		ServerPort:   auth.ServerPort,
		ProfileName:  auth.AuthOptions.ProfileName,
	}
	s.runSAMLFlow()
	// err := s.startServer()
	// if err != nil {
	// 	return err
	// }

	return nil
}

func (auth *AWSAuthenticator) handle(samlData SAMLData) (*time.Time, error) {
	d := samlData.AssertionInfo.Values.Get("https://aws.amazon.com/SAML/Attributes/SessionDuration")
	sec, err := strconv.ParseInt(d, 10, 64)
	if err != nil {
		stderrlogger.Error("Could not parse session duration", "error", err)
		return nil, err
	}

	opt := int64(auth.AuthOptions.SessionDuration)
	if opt == 0 {
		opt = int64(3600) // 1 hour default
	}

	stderrlogger.Debug("will use lowest value as session duration", "from-idp", sec, "from-config-or-input", opt)
	sec = min(sec, opt)

	principalARN := ""
	if roles, foundRoles := samlData.AssertionInfo.Values["https://aws.amazon.com/SAML/Attributes/Role"]; foundRoles {
		found := false
		for _, val := range roles.Values {
			stderrlogger.Debug("", "role attribute", val)
			split := strings.Split(val.Value, ",")
			if split[0] == auth.AuthOptions.RoleARN {
				principalARN = split[1]
				found = true
				break
			}
		}
		if !found {
			stderrlogger.Error("SAML response didn't contain the requested role")
			return nil, errors.New("SAML response didn't contain the requested role")
		}
	} else {
		stderrlogger.Error("SAML response didn't contain the roles assertion")
		return nil, errors.New("SAML response didn't contain the roles assertion")
	}

	session := session.Must(session.NewSession())

	stsInput := &sts.AssumeRoleWithSAMLInput{
		RoleArn:         &auth.AuthOptions.RoleARN,
		PrincipalArn:    &principalARN,
		SAMLAssertion:   &samlData.EncodedSAMLResponse,
		DurationSeconds: &sec,
	}
	stsClient := sts.New(session, aws.NewConfig().WithRegion(auth.AuthOptions.Region))

	output, err := stsClient.AssumeRoleWithSAML(stsInput)
	if err != nil {
		// return nil, err
		stderrlogger.Error("Could not assume role", "error", err)
		panic(err)
	}
	stderrlogger.Debug("Session obtained", "output", output)

	c := &Credentials{
		AccessKeyID:     *output.Credentials.AccessKeyId,
		SecretAccessKey: *output.Credentials.SecretAccessKey,
		SessionToken:    *output.Credentials.SessionToken,
		Expiration:      output.Credentials.Expiration,
	}
	err = auth.writeOutput(c)
	if err != nil {
		return nil, err
	}
	err = auth.CredentialStorage.StoreEntry(auth.AuthOptions.ProfileName, c)
	if err != nil {
		return nil, err
	}

	return c.Expiration, nil
}
