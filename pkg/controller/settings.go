/*
 * Copyright 2024 InfAI (CC SES)
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

func (this *Controller) GetSettings(token auth.Token) (settings model.Settings, err error, errCode int) {
	settings, err, errCode = this.db.ReadSettings(token.GetUserId())
	if err != nil && errCode == http.StatusNotFound {
		settings = model.Settings{
			EmailsEnabled: true,
		}
		return this.SetSettings(token, settings)
	}
	return
}

func (this *Controller) SetSettings(token auth.Token, settings model.Settings) (result model.Settings, err error, errCode int) {
	settings.UserId = token.GetUserId()
	err, errCode = this.db.SetSettings(settings)
	return settings, err, errCode
}
