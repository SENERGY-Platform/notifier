/*
 * Copyright 2025 InfAI (CC SES)
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
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestParseAndValidateToken(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	pub, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	pubB64 := base64.StdEncoding.EncodeToString(pub)
	sign := func(method jwt.SigningMethod, signKey any, claims jwt.MapClaims) string {
		s, err := jwt.NewWithClaims(method, claims).SignedString(signKey)
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	future := time.Now().Add(time.Hour).Unix()
	past := time.Now().Add(-time.Hour).Unix()
	otherKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("valid", func(t *testing.T) {
		tok, err := ParseAndValidateToken("Bearer "+sign(jwt.SigningMethodRS256, key, jwt.MapClaims{"sub": "u", "exp": future, "realm_access": map[string]any{"roles": []string{"admin"}}}), pubB64)
		if err != nil {
			t.Fatal(err)
		}
		if tok.GetUserId() != "u" || !tok.IsAdmin() || tok.IsExpired() || tok.Token == "" {
			t.Errorf("unexpected token %+v", tok)
		}
	})
	t.Run("expired token parses and reports IsExpired", func(t *testing.T) {
		tok, err := ParseAndValidateToken(sign(jwt.SigningMethodRS256, key, jwt.MapClaims{"sub": "u", "exp": past}), pubB64)
		if err != nil {
			t.Fatal(err)
		}
		if !tok.IsExpired() {
			t.Error("expected IsExpired")
		}
	})
	t.Run("missing subject", func(t *testing.T) {
		if _, err := ParseAndValidateToken(sign(jwt.SigningMethodRS256, key, jwt.MapClaims{"exp": future}), pubB64); err == nil {
			t.Error("expected error")
		}
	})
	t.Run("wrong key", func(t *testing.T) {
		if _, err := ParseAndValidateToken(sign(jwt.SigningMethodRS256, otherKey, jwt.MapClaims{"sub": "u"}), pubB64); err == nil {
			t.Error("expected error")
		}
	})
	t.Run("hmac with the public key as secret", func(t *testing.T) {
		if _, err := ParseAndValidateToken(sign(jwt.SigningMethodHS256, pub, jwt.MapClaims{"sub": "u"}), pubB64); err == nil {
			t.Error("expected error")
		}
	})
	t.Run("rsa-pss", func(t *testing.T) {
		if _, err := ParseAndValidateToken(sign(jwt.SigningMethodPS256, key, jwt.MapClaims{"sub": "u"}), pubB64); err == nil {
			t.Error("expected error")
		}
	})
	t.Run("alg none", func(t *testing.T) {
		if _, err := ParseAndValidateToken(sign(jwt.SigningMethodNone, jwt.UnsafeAllowNoneSignatureType, jwt.MapClaims{"sub": "u"}), pubB64); err == nil {
			t.Error("expected error")
		}
	})
}
