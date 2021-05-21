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

import os

import jwt

jwt_signing_key = "-----BEGIN PUBLIC KEY-----\n" + os.getenv('JWT_SIGNING_KEY', '') + "\n-----END PUBLIC KEY-----"
jwt_method = os.getenv('JWT_SIGNING_METHOD', 'RS256')

print(jwt_signing_key)

def decode(token: str) -> dict:
    return jwt.decode(token, jwt_signing_key, algorithms=[jwt_method], audience='account')
