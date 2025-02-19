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
	saml2 "github.com/russellhaering/gosaml2"
)

type AuthOptions struct {
	SessionDuration int64
	RoleARN         string
	Region          string
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

func (w *CredentialsProcessOutputWriter) WriteOutput(c *sts.AssumeRoleWithSAMLOutput) error {
	forJSON := &credentialProcessResponse{
		Version:         1,
		AccessKeyID:     *c.Credentials.AccessKeyId,
		SecretAccessKey: *c.Credentials.SecretAccessKey,
		SessionToken:    *c.Credentials.SessionToken,
		Expiration:      c.Credentials.Expiration,
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
	AuthOptions *AuthOptions
	Writer      CredentialsOutputWriter
}

type SAMLData struct {
	EncodedSAMLResponse string
	AssertionInfo       *saml2.AssertionInfo
}

func (auth *AWSAuthenticator) doAuth(samlData SAMLData) (*time.Time, error) {
	sessionDuration := samlData.AssertionInfo.Values.Get("https://aws.amazon.com/SAML/Attributes/SessionDuration")
	sessionDurationSeconds, err := strconv.ParseInt(sessionDuration, 10, 64)
	if err != nil {
		stderrlogger.Error("Could not parse session duration", "error", err)
		return nil, err
	}

	roleDurationSecondsOpt := int64(auth.AuthOptions.SessionDuration)
	if roleDurationSecondsOpt == 0 {
		roleDurationSecondsOpt = int64(3600) // 1 hour default
	}
	if roleDurationSecondsOpt < sessionDurationSeconds {
		sessionDurationSeconds = roleDurationSecondsOpt
	}

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
		DurationSeconds: &sessionDurationSeconds,
	}
	stsClient := sts.New(session, aws.NewConfig().WithRegion(auth.AuthOptions.Region))

	output, err := stsClient.AssumeRoleWithSAML(stsInput)
	if err != nil {
		// return nil, err
		stderrlogger.Error("Could not assume role", "error", err)
		panic(err)
	}
	stderrlogger.Debug("Session obtained", "output", output)

	err = auth.Writer.WriteOutput(output)
	if err != nil {
		return nil, err
	}
	err = storeEntry(auth.AuthOptions.RoleARN, output)
	if err != nil {
		return nil, err
	}

	return output.Credentials.Expiration, nil
}
