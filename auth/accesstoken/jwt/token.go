package jwt

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/docker/libtrust"
	"github.com/golang-jwt/jwt/v4"

	"github.com/distribution-auth/auth/auth"
)

type claims struct {
	jwt.RegisteredClaims

	Access []auth.Scope `json:"access"`
}

// Issuer issues tokens according to the [Token Authentication Specification] and [Token Authentication Implementation].
//
// [Token Authentication Specification]: https://github.com/distribution/distribution/blob/main/docs/spec/auth/token.md
// [Token Authentication Implementation]: https://github.com/distribution/distribution/blob/main/docs/spec/auth/jwt.md
type Issuer struct {
	Issuer     string
	SigningKey libtrust.PrivateKey
	Expiration time.Duration
}

func (i Issuer) IssueAccessToken(subject auth.Subject, audience []string, grantedScopes []auth.Scope) (auth.AccessToken, error) {
	randomBytes := make([]byte, 15)
	_, err := io.ReadFull(rand.Reader, randomBytes)
	if err != nil {
		return auth.AccessToken{}, err
	}
	randomID := base64.URLEncoding.EncodeToString(randomBytes)

	now := time.Now()

	var alg jwt.SigningMethod
	switch i.SigningKey.KeyType() {
	case "RSA":
		alg = jwt.SigningMethodRS256
	case "EC":
		alg = jwt.SigningMethodES256
	default:
		panic(fmt.Errorf("unsupported signing key type %q", i.SigningKey.KeyType()))
	}

	exp := i.Expiration
	if exp == 0 {
		exp = 5 * time.Minute
	}

	claims := claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    i.Issuer,
			Subject:   subject.ID(),
			Audience:  audience,
			ExpiresAt: jwt.NewNumericDate(now.Add(exp)),
			NotBefore: jwt.NewNumericDate(now),
			IssuedAt:  jwt.NewNumericDate(now),
			ID:        randomID,
		},
		Access: grantedScopes,
	}

	token := jwt.NewWithClaims(alg, claims)

	if x5c := i.SigningKey.GetExtendedField("x5c"); x5c != nil {
		token.Header["x5c"] = x5c.([]string)
	} else {
		var jwkMessage json.RawMessage
		jwkMessage, err = i.SigningKey.PublicKey().MarshalJSON()
		if err != nil {
			return auth.AccessToken{}, err
		}
		token.Header["jwk"] = &jwkMessage
	}

	signedToken, err := token.SignedString(i.SigningKey.CryptoPrivateKey())
	if err != nil {
		return auth.AccessToken{}, err
	}

	return auth.AccessToken{
		Payload:   signedToken,
		ExpiresIn: i.Expiration,
		IssuedAt:  now,
	}, nil
}
