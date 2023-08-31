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

package vault

import (
	"context"
	"errors"
	"github.com/SENERGY-Platform/notifier/pkg/configuration"
	"github.com/SENERGY-Platform/notifier/pkg/model"
	"github.com/google/uuid"
	"github.com/senergy-platform/vault-jwt-go/vault"
	"net/http"
	"strings"
)

type FcmTokenManager struct {
	vault *vault.Vault
}

type FcmTokenWithVaultId struct {
	Id string
	model.FcmToken
}

func NewFcmTokenManager(conf configuration.Config) (*FcmTokenManager, error) {
	v, err := vault.NewVault(context.Background(), conf.VaultUrl,
		conf.VaultRole, conf.KeycloakUrl,
		conf.KeycloakRealm, conf.KeycloakClientId, conf.KeycloakClientSecret, conf.VaultEngineFcm)

	if err != nil {
		return nil, err
	}
	return &FcmTokenManager{vault: v}, nil
}

func (manager *FcmTokenManager) Save(fcmToken *model.FcmToken) error {
	if fcmToken == nil {
		return errors.New("got nil fcmToken")
	}
	tokens, err := manager.GetFcmTokens(fcmToken.UserId)
	if err != nil {
		return err
	}
	for _, token := range tokens {
		if token.FcmToken.Token == fcmToken.Token {
			err = manager.vault.WriteInterface(token.Id, fcmToken)
			if err != nil {
				return err
			}
			metadata, err := manager.vault.GetMetadata(token.Id)
			if err != nil {
				return err
			}
			version := metadata.Version - 1 //vault starts at version 1
			old := make([]int, version)
			for i := 0; i < version; i++ {
				old[i] = i + 1 //vault starts at version 1
			}
			return manager.vault.DestroyVersions(token.Id, old)
		}
	}
	return manager.vault.WriteInterface(fcmToken.UserId+"_"+uuid.NewString(), fcmToken)
}

func (manager *FcmTokenManager) Delete(fcmToken *model.FcmToken) (err error, errorCode int) {
	if fcmToken == nil {
		return errors.New("got nil fcmToken"), http.StatusInternalServerError
	}
	tokens, err := manager.GetFcmTokens(fcmToken.UserId)
	if err != nil {
		return err, http.StatusBadGateway
	}
	for _, token := range tokens {
		if token.FcmToken.Token == fcmToken.Token {
			return manager.vault.Purge(token.Id), http.StatusOK
		}
	}

	return errors.New("not found"), http.StatusNotFound
}

func (manager *FcmTokenManager) ListKeys() ([]string, error) {
	return manager.vault.ListKeys()
}

func (manager *FcmTokenManager) GetFcmTokens(userId string) (tokens []FcmTokenWithVaultId, err error) {
	keys, err := manager.ListKeys()
	if err != nil {
		return nil, err
	}
	tokens = []FcmTokenWithVaultId{}
	for _, key := range keys {
		if strings.HasPrefix(key, userId+"_") {
			var token FcmTokenWithVaultId
			err := manager.vault.ReadInterface(key, &token)
			if err != nil {
				return nil, err
			}
			token.Id = key
			tokens = append(tokens, token)
		}
	}
	return
}
