package auth

// LoginRedirect is what the HTTP layer needs to start a login: where to send the
// browser, and the sealed blob for the oauth_tx cookie.
type LoginRedirect struct {
	AuthURL string
	OAuthTx string
}

// CallbackInput is everything CompleteLogin needs from the request, so the
// service never touches *http.Request.
type CallbackInput struct {
	Code    string
	State   string
	OAuthTx string
}

// LoginResult is a completed login. SessionID becomes the sid cookie.
type LoginResult struct {
	SessionID string
	IsNewUser bool
	User      *User
}
