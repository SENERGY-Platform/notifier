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

package controller

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/SENERGY-Platform/notifier/pkg/auth"
)

type KeycloakUser struct {
	ID               string `json:"id"`
	CreatedTimestamp int64  `json:"createdTimestamp"`
	Username         string `json:"username"`
	Enabled          bool   `json:"enabled"`
	Totp             bool   `json:"totp"`
	EmailVerified    bool   `json:"emailVerified"`
	FirstName        string `json:"firstName"`
	LastName         string `json:"lastName"`
	Email            string `json:"email"`
	Attributes       struct {
		Locale []string `json:"locale"`
	} `json:"attributes"`
	DisableableCredentialTypes []any `json:"disableableCredentialTypes"`
	RequiredActions            []any `json:"requiredActions"`
	NotBefore                  int   `json:"notBefore"`
	Access                     struct {
		ManageGroupMembership bool `json:"manageGroupMembership"`
		View                  bool `json:"view"`
		MapRoles              bool `json:"mapRoles"`
		Impersonate           bool `json:"impersonate"`
		Manage                bool `json:"manage"`
	} `json:"access"`
}

func (c *Controller) updateClientToken() (err error) {
	if c.clientToken != nil && c.clientToken.RequestTime.Add(time.Duration(c.clientToken.ExpiresIn)*time.Second).After(time.Now().Add(10*time.Second)) {
		return
	}
	now := time.Now()

	resp, err := http.PostForm(c.config.KeycloakUrl+"/auth/realms/"+c.config.KeycloakRealm+"/protocol/openid-connect/token", url.Values{
		"client_id":     {c.config.KeycloakClientId},
		"client_secret": {c.config.KeycloakClientSecret},
		"grant_type":    {"client_credentials"},
	})

	if err != nil {
		log.Println("ERROR: updateClientToken::PostForm()", err)
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Println("ERROR: updateClientToken()", resp.StatusCode, string(body))
		err = errors.New(string(body))
		return
	}
	/*
		body, _ := io.ReadAll(resp.Body)
		log.Println(string(body))
		return fmt.Errorf("not implemented")
	*/
	err = json.NewDecoder(resp.Body).Decode(c.clientToken)
	if err != nil {
		return err
	}
	c.clientToken.RequestTime = now
	return
}

func (c *Controller) createInternalUserToken(userid string) (token *auth.Token, err error) {
	err = c.updateClientToken()
	if err != nil {
		log.Println("ERROR: createInternalUserToken::updateClientToken()", err)
		return nil, err
	}
	req, err := http.NewRequest(http.MethodGet, c.config.KeycloakUrl+"/auth/admin/realms/"+c.config.KeycloakRealm+"/users/"+userid, nil)
	if err != nil {
		log.Println("ERROR: createInternalUserToken::NewRequest", err)
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.clientToken.AccessToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Println("ERROR: createInternalUserToken::Do", err)
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Println("ERROR: getOpenidToken()", resp.StatusCode, string(body))
		err = errors.New(string(body))
		return
	}
	var user KeycloakUser
	err = json.NewDecoder(resp.Body).Decode(&user)
	if err != nil {
		log.Println("ERROR: createInternalUserToken", err)
		return nil, err
	}
	return &auth.Token{
		Sub:           user.ID,
		Email:         user.Email,
		EmailVerified: user.EmailVerified,
	}, nil

}
