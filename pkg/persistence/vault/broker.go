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
	"github.com/senergy-platform/vault-jwt-go/vault"
)

type BrokerManager struct {
	vault *vault.Vault
}

func New(conf configuration.Config) (*BrokerManager, error) {
	v, err := vault.NewVault(context.Background(), conf.VaultUrl,
		conf.VaultRole, conf.KeycloakUrl,
		conf.KeycloakRealm, conf.KeycloakClientId, conf.KeycloakClientSecret, conf.VaultEngineBroker)

	if err != nil {
		return nil, err
	}
	return &BrokerManager{vault: v}, nil
}

func (manager *BrokerManager) SaveAndStrip(broker *model.Broker) error {
	if broker == nil {
		return errors.New("got nil broker")
	}
	secret := secretBrokerDataFromBroker(broker)
	err := manager.vault.WriteInterface(broker.Id, secret)
	if err != nil {
		return err
	}
	stripBroker(broker)
	return nil
}

func (manager *BrokerManager) Fill(broker *model.Broker) error {
	// possible improvement: cache secret data in memory
	if broker == nil {
		return errors.New("got nil broker")
	}
	var secret SecretBrokerData
	err := manager.vault.ReadInterface(broker.Id, &secret)
	if err != nil {
		return err
	}
	secret.fillModel(broker)
	return nil
}

func (manager *BrokerManager) FillList(brokers []model.Broker) (list []model.Broker, err error) {
	list = []model.Broker{}
	for _, broker := range brokers {
		err = manager.Fill(&broker)
		if err != nil {
			return nil, err
		}
		list = append(list, broker)
	}
	return list, nil
}

func (manager *BrokerManager) Delete(ids []string) (err error) {
	for _, id := range ids {
		// This could result in inconsistency, but there is no better way. Consistency is ensured at startup
		err = manager.vault.Purge(id)
		if err != nil {
			return err
		}
	}
	return nil
}

func (manager *BrokerManager) ListKeys() ([]string, error) {
	return manager.vault.ListKeys()
}
