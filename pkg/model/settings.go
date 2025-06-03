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

package model

type Channel = string

const ChannelWebsocket = "websoket"
const ChannelMqtt = "mqtt"
const ChannelFcm = "push"
const ChannelEmail = "email"

func AllChannels() []Channel {
	return []Channel{
		ChannelWebsocket,
		ChannelMqtt,
		ChannelFcm,
		ChannelEmail,
	}
}

type Settings struct {
	UserId             string              `json:"-" bson:"user_id"`
	ChannelTopicConfig map[Channel][]Topic `json:"channel_topic_config" bson:"channel_topic_config"`
}

func DefaultSettings() Settings {
	ChannelTopicConfig := map[Channel][]Topic{}
	topics := append(AllTopics(), TopicUnknown) // here the user should be able to set settings for TopicUnknown
	for _, channel := range AllChannels() {
		ChannelTopicConfig[channel] = topics // all channels use all topics
	}
	ChannelTopicConfig[ChannelFcm] = []Topic{TopicProcesses} // ChannelFcm uses only TopicProcesses
	return Settings{ChannelTopicConfig: ChannelTopicConfig}
}
