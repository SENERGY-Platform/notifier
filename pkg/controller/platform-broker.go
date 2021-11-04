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

package controller

import (
	"github.com/SENERGY-Platform/notifier/pkg/auth"
	"github.com/SENERGY-Platform/notifier/pkg/model"
	"net/http"
)

func (this *Controller) GetPlatformBroker(token auth.Token) (platformBroker model.PlatformBroker, err error, errCode int) {
	platformBroker, err, errCode = this.db.ReadPlatformBroker(token.GetUserId())
	if err != nil && errCode == http.StatusNotFound {
		platformBroker = model.PlatformBroker{
			Enabled: false,
		}
		return this.SetPlatformBroker(token, platformBroker)
	}
	return
}

func (this *Controller) SetPlatformBroker(token auth.Token, platformBroker model.PlatformBroker) (result model.PlatformBroker, err error, errCode int) {
	platformBroker.UserId = token.GetUserId()
	err, errCode = this.db.SetPlatformBroker(platformBroker)
	return platformBroker, err, errCode
}

func (this *Controller) DeletePlatformBroker(token auth.Token) (err error, errCode int) {
	return this.db.RemovePlatformBroker(token.GetUserId())
}
