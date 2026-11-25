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

package controller

import (
	"testing"

	"github.com/SENERGY-Platform/notifier/pkg/model"
)

func TestBuildFcmMessageChannel(t *testing.T) {
	n := model.Notification{Id: "id1", Title: "t", Message: "m"}

	msg := buildFcmMessage([]string{"tok"}, n, "{}")
	if msg.Android == nil || msg.Android.Notification.ChannelID != "" {
		t.Fatal("non-silent notification must use the default channel")
	}
	if msg.Android.Notification.Tag != "id1" {
		t.Error("android tag must be the notification id")
	}

	n.Silent = true
	msg = buildFcmMessage([]string{"tok"}, n, "{}")
	if msg.Android.Notification.ChannelID != "silent" {
		t.Error("silent notification must use channel silent, got", msg.Android.Notification.ChannelID)
	}

	n.IsRead = true
	msg = buildFcmMessage([]string{"tok"}, n, "{}")
	if msg.Android != nil || msg.Notification != nil {
		t.Error("read notification must not carry a visible notification")
	}
}
