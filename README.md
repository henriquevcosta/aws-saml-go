> [!NOTE]
> Currently only macOS is supported, linux should come in the near future.

# aws-saml-go
Simple credential_process app that will use your browser session to get a SAML auth for AWS.

Assumes that your IdP is configured to redirect back to `http://127.0.0.1:35002/` after authentication and has only been
tested with Google's SAML assertions.

Configurations are done via the `~/.aws/config` file. Just place the binary downloaded from 
the [releases](https://github.com/henriquevcosta/aws-saml-go/releases) on your filesystem
and adjust the path in the config file.

The IDP-specific configs can be made explicit in a `saml_idp <name>` section of the config and then pointing to it from a `saml.idp_config=<name>` property.

```ini
[default]
region = ap-southeast-2
saml.idp_config=google

[saml_idp google]
# this is a google SAML example where the SP is configured in the Google Admin console as
# redirecting back to http://127.0.0.1:35002/
# IIIIIIIII and SSSSSSSSSS would be your IDP ID and SP ID respectively
saml.idp_url = https://accounts.google.com/o/saml2/initsso?idpid=IIIIIIIII&spid=SSSSSSSSSS&forceauthn=false
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
saml.idp_url = https://accounts.google.com/o/saml2/initsso?idpid=IIIIIIIII&spid=SSSSSSSSSS&forceauthn=false
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

## Usage

This tool conforms to AWS's `credential_process` format so you can use it in any way that you can use AWS profiles. You can just run `aws s3 ls --profile prof2` or you can use some more sofisticated tools like [Granted](https://www.granted.dev/) or [AWSume](https://awsu.me/general/quickstart.html).

# Credit

A lot of this tool's working and some of the code is inspired from https://github.com/bengieeee/aws-google-saml. I just built this because I wanted a binary that wasn't dependent on Python.
