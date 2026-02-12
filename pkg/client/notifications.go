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

func (c *Client) ListNotifications(token *string, options persistence.ListOptions, channel model.Channel) (result model.NotificationList, err error) {
	return do[model.NotificationList]("GET", c.notifierUrl+"/notifications", token, nil, map[string]string{
		"limit":   fmt.Sprintf("%d", options.Limit),
		"offset":  fmt.Sprintf("%d", options.Offset),
		"channel": string(channel),
	})
}
func (c *Client) ReadNotification(token *string, id string) (result model.Notification, err error) {
	return do[model.Notification]("GET", c.notifierUrl+"/notifications/"+id, token, nil, nil)
}
func (c *Client) CreateNotification(token *string, notification model.Notification, ignoreDuplicatesWithinSeconds *int64) (result model.Notification, err error) {
	return do[model.Notification]("POST", c.notifierUrl+"/notifications", token, notification, map[string]string{
		"ignore_duplicates_within_seconds": fmt.Sprintf("%d", *ignoreDuplicatesWithinSeconds),
	})
}
func (c *Client) SetNotification(token *string, notification model.Notification) (result model.Notification, err error) {
	return do[model.Notification]("PUT", c.notifierUrl+"/notifications/"+notification.Id, token, notification, nil)
}
func (c *Client) DeleteMultipleNotifications(token *string, ids []string) (err error) {
	_, err = do[any]("DELETE", c.notifierUrl+"/notifications", token, ids, nil)
	return err
}
