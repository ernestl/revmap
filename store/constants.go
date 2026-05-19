package store

const (
	// StoreDashboardURL is the base URL for the Snap Store dashboard API.
	StoreDashboardURL = "https://dashboard.snapcraft.io/"

	// UbuntuOneLocation is the hostname used as the macaroon caveat location.
	UbuntuOneLocation = "login.ubuntu.com"

	// UbuntuOneAPIBase is the base URL for Ubuntu One SSO API v2.
	UbuntuOneAPIBase = "https://login.ubuntu.com/api/v2"

	// MacaroonACLAPI is the endpoint to request an ACL macaroon from the store.
	MacaroonACLAPI = StoreDashboardURL + "dev/api/acl/"

	// DischargeAPI is the SSO endpoint to discharge a macaroon.
	DischargeAPI = UbuntuOneAPIBase + "/tokens/discharge"

	// RefreshDischargeAPI is the SSO endpoint to refresh a discharge macaroon.
	RefreshDischargeAPI = UbuntuOneAPIBase + "/tokens/refresh"

	// AppName is the application name used for credential storage.
	AppName = "revmap"

	// CredentialsEnvVar is the environment variable for credential override.
	CredentialsEnvVar = "REVMAP_STORE_CREDENTIALS"
)
