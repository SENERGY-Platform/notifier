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
	"github.com/SENERGY-Platform/notifier/pkg/model"
)

func (c *Client) GetPlatformBroker(token *string) (platformBroker model.PlatformBroker, err error) {
	return do[model.PlatformBroker]("GET", c.notifierUrl+"/platform-broker", token, nil, nil)
}
func (c *Client) SetPlatformBroker(token *string, platformBroker model.PlatformBroker) (result model.PlatformBroker, err error) {
	return do[model.PlatformBroker]("PUT", c.notifierUrl+"/platform-broker", token, platformBroker, nil)
}
func (c *Client) DeletePlatformBroker(token *string) (err error) {
	_, err = do[any]("DELETE", c.notifierUrl+"/platform-broker", token, nil, nil)
	return err
}
