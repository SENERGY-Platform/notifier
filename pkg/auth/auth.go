/*
 * Copyright 2021 InfAI (CC SES)
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *    http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package auth

import (
	"crypto/x509"
	"encoding/base64"
	"errors"
	"github.com/golang-jwt/jwt/v5"
	"net/http"
	"strings"
	"time"
)

var TimeNow = func() time.Time {
	return time.Now()
}

func GetAuthToken(req *http.Request) string {
	return req.Header.Get("Authorization")
}

func GetParsedToken(req *http.Request) (token Token, err error) {
	return Parse(GetAuthToken(req))
}

type Token struct {
	Token         string              `json:"-"`
	Sub           string              `json:"sub,omitempty"`
	RealmAccess   map[string][]string `json:"realm_access,omitempty"`
	Expiration    int64               `json:"exp"`
	Email         string              `json:"email"`
	EmailVerified bool                `json:"email_verified"`
}

func (this *Token) String() string {
	return this.Token
}

func (this *Token) Jwt() string {
	return this.Token
}

func (this *Token) Valid() error {
	if this.Sub == "" {
		return errors.New("missing subject")
	}
	return nil
}

// tokenClaims adapts Token to jwt.Claims without changing Token's exported API.
// The time getters return nil, so the library checks no exp/nbf/iat: expiry is checked by callers
// through IsExpired, and Validate keeps the only check the previous library version ran.
type tokenClaims Token

func (this *tokenClaims) GetExpirationTime() (*jwt.NumericDate, error) { return nil, nil }
func (this *tokenClaims) GetIssuedAt() (*jwt.NumericDate, error)       { return nil, nil }
func (this *tokenClaims) GetNotBefore() (*jwt.NumericDate, error)      { return nil, nil }
func (this *tokenClaims) GetIssuer() (string, error)                   { return "", nil }
func (this *tokenClaims) GetSubject() (string, error)                  { return this.Sub, nil }
func (this *tokenClaims) GetAudience() (jwt.ClaimStrings, error)       { return nil, nil }

func (this *tokenClaims) Validate() error {
	return (*Token)(this).Valid()
}

func Parse(token string) (claims Token, err error) {
	orig := token
	if strings.HasPrefix(strings.ToLower(token), "bearer ") {
		token = token[7:]
	}
	_, _, err = jwt.NewParser().ParseUnverified(token, (*tokenClaims)(&claims))
	if err == nil {
		claims.Token = orig
	}
	return
}

func ParseAndValidateToken(token string, pubRsaKey string) (claims Token, err error) {
	orig := token
	if strings.HasPrefix(strings.ToLower(token), "bearer ") {
		token = token[7:]
	}
	_, err = jwt.ParseWithClaims(token, (*tokenClaims)(&claims), func(token *jwt.Token) (interface{}, error) {
		//decode key base64 string to []byte
		b, err := base64.StdEncoding.DecodeString(pubRsaKey)
		if err != nil {
			return nil, err
		}
		//parse []byte key to go struct key (use most common encoding)
		return x509.ParsePKIXPublicKey(b)
	}, jwt.WithValidMethods([]string{"RS256", "RS384", "RS512"})) // the configured key is an RSA public key
	if err == nil {
		claims.Token = orig
	}
	return
}

func (this *Token) IsAdmin() bool {
	return contains(this.RealmAccess["roles"], "admin")
}

func (this *Token) GetUserId() string {
	return this.Sub
}

func (this *Token) IsExpired() bool {
	expiresIn := time.Unix(this.Expiration, 0).Sub(TimeNow())
	return expiresIn <= 0
}

func (this *Token) ExpiresBefore(buffer time.Duration) bool {
	expiresIn := time.Unix(this.Expiration, 0).Sub(TimeNow())
	return expiresIn <= time.Duration(buffer)
}

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}
