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
	"errors"
	"fmt"
	"net/http"
	"slices"
	"time"

	"github.com/SENERGY-Platform/notifier/pkg/auth"
	"github.com/SENERGY-Platform/notifier/pkg/model"
	"github.com/SENERGY-Platform/notifier/pkg/persistence"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func (this *Controller) ListNotifications(token auth.Token, options persistence.ListOptions, channel model.Channel) (result model.NotificationList, err error, errCode int) {
	result.Limit, result.Offset = options.Limit, options.Offset
	topics := []model.Topic(append(model.AllTopics(), model.TopicUnknown))
	if !slices.Contains(model.AllChannels(), channel) {
		return result, fmt.Errorf("unknown channel %s", channel), http.StatusBadRequest
	}
	if len(channel) > 0 {
		settings, err, code := this.GetSettings(token)
		if err != nil {
			return result, err, code
		}
		topics = settings.ChannelTopicConfig[channel]
	}
	result.Notifications, result.Total, err, errCode = this.db.ListNotifications(token.GetUserId(), options, topics)
	return
}

func (this *Controller) ReadNotification(token auth.Token, id string) (result model.Notification, err error, errCode int) {
	result, err, errCode = this.db.ReadNotification(token.GetUserId(), id)
	if err != nil {
		return model.Notification{}, err, errCode
	}
	return result, nil, http.StatusOK
}

func (this *Controller) SetNotification(token auth.Token, notification model.Notification) (result model.Notification, err error, errCode int) {
	_, err, errCode = this.db.ReadNotification(token.GetUserId(), notification.Id) // Check existence before set, this already checks for user id
	if err != nil {
		return model.Notification{}, err, errCode
	}
	settings, err, errCode := this.getSettings(token.GetUserId())
	if err != nil {
		return model.Notification{}, err, errCode
	}
	err, errCode = this.db.SetNotification(notification)
	if err == nil {
		this.handleUpdate(notification, token, settings)
	}
	return notification, err, errCode
}

func (this *Controller) CreateNotification(token *auth.Token, notification model.Notification, ignoreDuplicatesWithinSeconds *int64) (result model.Notification, err error, errCode int) {
	if token == nil { //internal access
		token, err = this.createInternalUserToken(notification.UserId)
		if err != nil {
			return model.Notification{}, fmt.Errorf("unable to get user info"), http.StatusInternalServerError
		}
	}
	if notification.Id != "" {
		return result, errors.New("specifing id is not allowed"), http.StatusBadRequest
	}
	if len(notification.Topic) == 0 {
		notification.Topic = model.TopicUnknown
	}
	if !slices.Contains(append(model.AllTopics(), model.TopicUnknown), notification.Topic) {
		return result, errors.New("the specified topic is not allowed"), http.StatusBadRequest
	}
	if ignoreDuplicatesWithinSeconds != nil {
		notOlderThan := time.Unix(time.Now().Unix()-*ignoreDuplicatesWithinSeconds, 0)
		existing, err, code := this.db.ReadNotificationByHash(token.GetUserId(), notification.Hash(), notOlderThan)
		if err == nil {
			return existing, err, code
		}
		if code != http.StatusNotFound {
			return existing, err, code
		}
	}

	notification.Id = primitive.NewObjectID().Hex()
	if notification.CreatedAt.Before(time.UnixMilli(0)) {
		notification.CreatedAt = time.Now().Truncate(time.Millisecond)
	}
	settings, err, errCode := this.getSettings(token.GetUserId())
	if err != nil {
		return model.Notification{}, err, errCode
	}
	err, errCode = this.db.SetNotification(notification)
	if err == nil {
		this.handleCreate(notification, *token, settings)
	}
	return notification, err, errCode
}

func (this *Controller) DeleteMultipleNotifications(token auth.Token, ids []string) (err error, errCode int) {
	err, errCode = this.db.RemoveNotifications(token.GetUserId(), ids)
	if err == nil {
		go this.handleWsNotificationDelete(token.GetUserId(), ids)
		go this.handleFCMNotificationDelete(token.GetUserId(), ids)
	}
	return
	/*
		delete does not consider settings for channels and topics.
		all channels will receive delete messages on all topics.
		expecting the client implementations to handle unsolicitated delete messages.
		reasoning: respecting settings would need to load and inspect each message
		before deletion in order to know its topic.
		this would increase the latency for deleting high numbers of notifications (e.g. in a "delete all" scenario)
	*/
}

func (this *Controller) getSettings(userId string) (model.Settings, error, int) {
	settings, err, errCode := this.db.ReadSettings(userId)
	if err != nil && errCode != http.StatusNotFound {
		return model.Settings{}, err, errCode
	}
	if errCode == http.StatusNotFound {
		settings = model.DefaultSettings()
	}
	return settings, nil, http.StatusOK
}

func (this *Controller) handleCreate(notification model.Notification, token auth.Token, settings model.Settings) {
	if len(notification.Topic) == 0 {
		notification.Topic = model.TopicUnknown
	}

	conf, ok := settings.ChannelTopicConfig[model.ChannelEmail]
	if !ok || slices.Contains(conf, notification.Topic) {
		go this.handleEmailNotificationUpdate(token, notification)
	}

	this.handleUpdate(notification, token, settings)
}

func (this *Controller) handleUpdate(notification model.Notification, token auth.Token, settings model.Settings) {
	if len(notification.Topic) == 0 {
		notification.Topic = model.TopicUnknown
	}

	conf, ok := settings.ChannelTopicConfig[model.ChannelWebsocket]
	if !ok || slices.Contains(conf, notification.Topic) {
		go this.handleWsNotificationUpdate(token.GetUserId(), notification)
	}
	conf, ok = settings.ChannelTopicConfig[model.ChannelMqtt]
	if !ok || slices.Contains(conf, notification.Topic) {
		go this.handleMqttNotificationUpdate(token.GetUserId(), notification)
	}
	conf, ok = settings.ChannelTopicConfig[model.ChannelFcm]
	if !ok || slices.Contains(conf, notification.Topic) {
		go this.handleFCMNotificationUpdate(token.GetUserId(), notification)
	}
}
