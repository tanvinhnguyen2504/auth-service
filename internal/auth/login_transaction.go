package auth

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/nguyentanvinh/auth-be/backend/internal/secret"
)

// loginTxAAD domain-separates this ciphertext from everything else sealed with
// the same master key, so an oauth_tx blob can never open as an API key.
const loginTxAAD = "oauth_tx"

// sealLoginTx turns a transaction into the opaque oauth_tx cookie value. The
// browser carries it but must not choose it, which GCM's tag guarantees.
func sealLoginTx(sealer *secret.Box, tx LoginTransaction) (string, error) {
	payload, err := json.Marshal(tx)
	if err != nil {
		return "", fmt.Errorf("auth: marshal login transaction: %w", err)
	}

	sealed, err := sealer.Seal(payload, []byte(loginTxAAD))
	if err != nil {
		return "", fmt.Errorf("auth: seal login transaction: %w", err)
	}
	return sealed, nil
}

// openLoginTx reverses sealLoginTx. Expiry comes from the sealed ExpiresAt, not
// the cookie's Max-Age, which the browser alone decides to honour.
func openLoginTx(sealer *secret.Box, sealed string) (LoginTransaction, error) {
	payload, err := sealer.Open(sealed, []byte(loginTxAAD))
	if err != nil {
		return LoginTransaction{}, fmt.Errorf("%w: open oauth_tx: %w", ErrLoginRejected, err)
	}

	var tx LoginTransaction
	if err := json.Unmarshal(payload, &tx); err != nil {
		return LoginTransaction{}, fmt.Errorf("%w: decode oauth_tx: %w", ErrLoginRejected, err)
	}

	if !tx.ExpiresAt.After(time.Now()) {
		return LoginTransaction{}, fmt.Errorf("%w: oauth_tx expired", ErrLoginRejected)
	}
	return tx, nil
}
