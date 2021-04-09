# Copyright 2021 InfAI (CC SES)
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#   http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

from geventwebsocket.websocket import WebSocket

from util import jwt


class WebSocketContainer():
    def __init__(self, ws: WebSocket):
        self.authenticated_until = 0
        self.user_id: str = ''
        self.ws = ws

    def authenticate(self, token: str) -> str:
        decoded = jwt.decode(token)
        self.user_id = decoded["sub"]
        self.authenticated_until = decoded["exp"]
        return self.user_id
