# aws-saml-go
Simple credential_process app that will use your browser session to get a SAML auth for AWS

Configurations are done via the `~/.aws/config` file.

```ini
[default]
region = ap-southeast-2
saml.idp_config=google

[saml_idp google]
# this is a google SAML example where the SP is configured in the Google Admin console as
# redirecting back to http://127.0.0.1:35002/
# IIIIIIIII and SSSSSSSSSS would be your IDP ID and SP ID respectively
saml.idp_url = https://accounts.google.com/o/saml2/initsso?forceauthn=false&idpid=IIIIIIIII&spid=SSSSSSSSSS
saml.duration = 1000

[saml_idp another_idp]
# Some other generic IDP that returns the same structure of SAML assertion as google
saml.idp_url = https://some-url
saml.duration = 3000

[profile prof1]
# Will inherit the idp config from the default section
account = 1111111111
saml.role_arn = arn:aws:iam::1111111111:role/the-role
credential_process = /path/to/aws-google-go prof1

[profile prof2]
region = us-east-2
account = 1111111111
saml.role_arn = arn:aws:iam::1111111111:role/the-role
credential_process = /path/to/aws-google-go prof2
# Inline idp configs
saml.idp_url = https://accounts.google.com/o/saml2/initsso?forceauthn=false&idpid=IIIIIIIII&spid=SSSSSSSSSS
saml.duration = 1000

[profile prof3]
region = us-east-2
account = 1111111111
saml.idp_config=another_idp
# Overriding the default profile
saml.duration = 3600
saml.role_arn = arn:aws:iam::1111111111:role/the-role
credential_process = /path/to/aws-google-go prof3

```
