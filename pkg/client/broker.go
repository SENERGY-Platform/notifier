/*
 * Copyright 2026 InfAI (CC SES)
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

package client

import (
	"fmt"

	"github.com/SENERGY-Platform/notifier/pkg/model"
	"github.com/SENERGY-Platform/notifier/pkg/persistence"
)

func (c *Client) ListBrokers(token *string, options persistence.ListOptions) (result model.BrokerList, err error) {
	queryParams := map[string]string{
		"limit":  fmt.Sprintf("%d", options.Limit),
		"offset": fmt.Sprintf("%d", options.Offset),
	}
	return do[model.BrokerList]("GET", c.notifierUrl+"/brokers", token, nil, queryParams)
}
func (c *Client) ReadBroker(token *string, id string) (result model.Broker, err error) {
	return do[model.Broker]("GET", c.notifierUrl+"/brokers/"+id, token, nil, nil)

}
func (c *Client) CreateBroker(token *string, broker model.Broker) (result model.Broker, err error) {
	return do[model.Broker]("POST", c.notifierUrl+"/brokers", token, broker, nil)
}
func (c *Client) SetBroker(token *string, broker model.Broker) (result model.Broker, err error) {
	return do[model.Broker]("PUT", c.notifierUrl+"/brokers/"+broker.Id, token, broker, nil)
}
func (c *Client) DeleteMultipleBrokers(token *string, ids []string) (err error) {
	_, err = do[any]("DELETE", c.notifierUrl+"/brokers", token, ids, nil)
	return err
}
