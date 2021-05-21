# Copyright 2018 InfAI (CC SES)
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

from gevent import monkey
monkey.patch_all() # required to do this before any other imports

import json
import logging
import os
import time
from typing import Dict, List

from flask import Flask, request, abort
from flask_cors import CORS
from flask_restx import Api, Resource, fields, reqparse
from flask_uwsgi_websocket import GeventWebSocket, GeventWebSocketClient

from util.db import DB
from util.WebSocketContainer import WebSocketContainer

application = Flask("notification-service")

sockets = GeventWebSocket(application)
application.config.SWAGGER_UI_DOC_EXPANSION = 'list'
CORS(application)
api = Api(application, version='0.1', title='Notification Service API',
          description='Notification Service API')

if os.getenv("DEBUG", False):
    logging.basicConfig()
    logging.getLogger().setLevel(logging.DEBUG)

logger = logging.getLogger('main')


@api.route('/doc')
class Docs(Resource):
    def get(self):
        return api.__schema__


ns = api.namespace('notifications', description='Operations related to notifications')

notification_model = api.model('Notification', {
    'userId': fields.String(required=True, description='User ID'),
    'title': fields.String(required=True, description='Title'),
    'message': fields.String(required=True, description='Message'),
    'isRead': fields.Boolean(required=True, description='If the message has been read'),
    'created_at': fields.String(description='Creation timestamp. Might be null for older messages'),
})

notification_return = notification_model.clone('Notification', {
    '_id': fields.String(required=True, description='Notification id'),
    'userId': fields.String
})

notification_list = api.model('NotificationList', {
    "notifications": fields.List(fields.Nested(notification_return))
})

not_found_msg = "Notification not found"

user_sockets: Dict[str, List[WebSocketContainer]] = {}
db = DB()


@ns.route('/', strict_slashes=False)
class Operator(Resource):
    @api.expect(notification_model)
    @api.marshal_with(notification_return, code=201)
    @api.response(403, 'Forbidden')
    def put(self):
        """Creates a notification."""
        req = request.get_json()
        user_id = get_user_id(request)
        if user_id is None or req['userId'] == user_id:
            o = db.create_notification(req)
            logger.info("Added notification: " + str(o['_id']) + " for user " + req['userId'])
            send_update(o)
            return o, 201
        else:
            abort(403, 'You may only send messages to yourself')

    @api.marshal_with(notification_list, code=200)
    def get(self):
        """Returns a list of notifications."""
        parser = reqparse.RequestParser()
        parser.add_argument('limit', type=int, help='Limit', location='args')
        parser.add_argument('offset', type=int, help='Offset', location='args')
        parser.add_argument('sort', type=str, help='Sort', location='args')
        args = parser.parse_args()
        limit = 0
        if not (args["limit"] is None):
            limit = args["limit"]
        offset = 0
        if not (args["offset"] is None):
            offset = args["offset"]
        if not (args["sort"] is None):
            sort = args["sort"].split(":")
        else:
            sort = ["_id", "desc"]
        user_id = get_user_id(request)
        if user_id is None:
            abort(400, "Missing header X-UserID")
            return

        notifications_list = db.list_notifications(limit=limit, offset=offset, sort=sort, user_id=user_id)
        logger.info("User " + user_id + " read " + str(len(notifications_list)) + " notifications")
        return {"notifications": notifications_list}


@ns.route('/<string:notification_id>', strict_slashes=False)
@api.response(404, 'Notification not found.')
@api.response(400, 'Bad request')
class OperatorUpdate(Resource):
    @api.marshal_with(notification_return)
    def get(self, notification_id):
        """Get a single notification. This will perform userId checks and returns 404, even if this messages exists, but the userId isn't matching """
        user_id = get_user_id(request)
        try:
            o = db.read_notification(notification_id, user_id)
        except Exception as e:
            abort(400, str(e))
            return
        logger.debug(o)
        if o is not None:
            return o, 200
        abort(404, not_found_msg)

    @api.expect(notification_model)
    @api.marshal_with(notification_return)
    @api.response(403, 'Forbidden')
    @api.response(404, 'Notification not found')
    def post(self, notification_id):
        """Updates a notification."""
        user_id = get_user_id(request)
        req = request.get_json()
        if user_id is None or req['userId'] == user_id:
            try:
                n = db.update_notification(req, notification_id, user_id)
            except Exception as e:
                abort(400, str(e))
                return
            if n is None:
                abort(404, not_found_msg)
            send_update(n)
            return n
        else:
            abort(403, 'You may only update your own messages')

    @api.response(204, "Deleted")
    def delete(self, notification_id):
        """Deletes a notification."""
        user_id = get_user_id(request)
        n = db.read_notification(notification_id, user_id)
        d = db.delete_notification(notification_id, user_id)
        if d.deleted_count == 0:
            abort(404, not_found_msg)
        send_delete(notification_id, n["userId"])
        return "Deleted", 204


def get_user_id(req) -> str:
    user_id = req.headers.get('X-UserID')
    return user_id


@sockets.route('/ws')
def sock(ws: GeventWebSocketClient):
    cws = WebSocketContainer(ws)
    ws.receive()
    while True:
        message = cws.ws.receive()
        if message is None:  # Connection closed
            if not cws.user_id == '' and cws.user_id in user_sockets:
                user_sockets[cws.user_id].remove(cws)
            break
        if len(message) == 0:
            continue
        try:
            logger.debug(message.decode('utf-8'))
            message = json.loads(message.decode('utf-8'))
        except Exception as e:
            print("decoding error", str(e))
            cws.ws.close()
            continue

        if "type" not in message or not isinstance(message["type"], str):
            cws.ws.close()
            continue
        if message["type"] == "refresh":
            if cws.user_id == '' or cws.authenticated_until < time.time():
                cws.ws.close()
                continue
            notifications = db.list_notifications(limit=100000, offset=0, sort=["_id", "desc"], user_id=cws.user_id)
            for n in notifications:
                n['_id'] = str(n['_id'])
            cws.ws.send('{"type":"notification list", "payload":' + json.dumps(notifications, ensure_ascii=False) + '}')
            continue
        elif message["type"] == "authentication":
            if "payload" not in message or not isinstance(message["payload"], str):
                print("payload error")
                cws.ws.close()
                continue
            if not cws.user_id == '' and cws.user_id in user_sockets:
                user_sockets[cws.user_id].remove(cws)
            try:
                payload: str = message["payload"]
                token = payload[7:]  # strips "Bearer "
                user_id = cws.authenticate(token)
                if user_id not in user_sockets:
                    user_sockets[user_id] = []
                user_sockets[user_id].append(cws)
                cws.ws.send('{"type": "authentication confirmed"}')
            except Exception as e:
                print("authentication error", str(e))
                cws.ws.close()
            continue
        else:
            print(str("unknown ws message type"))
            cws.ws.close()
            continue


def send_update(notification):
    if notification["userId"] in user_sockets:
        for cws in user_sockets[notification["userId"]]:
            if cws.user_id == '' or cws.authenticated_until < time.time():
                try:
                    cws.ws.send('{"type":"please reauthenticate"')
                except Exception as e:
                    print("Could not send update", str(e))
                continue
            notification['_id'] = str(notification['_id'])
            try:
                cws.ws.send(
                    '{"type":"put notification", "payload":' + json.dumps(notification, ensure_ascii=False) + '}')
            except Exception as e:
                print("Could not send update", str(e))


def send_delete(notification_id: str, user_id: str):
    if user_id in user_sockets:
        for cws in user_sockets[user_id]:
            if cws.user_id == '' or cws.authenticated_until < time.time():
                try:
                    cws.ws.send('{"type":"please reauthenticate"}')
                except Exception as e:
                    print("Could not send delete", str(e))
                continue
            try:
                cws.ws.send('{"type":"delete notification", "payload":"' + notification_id + '"' + '}')
            except Exception as e:
                print("Could not send update", str(e))


if __name__ == "__main__":
    print("Please run using command from Dockerfile")

