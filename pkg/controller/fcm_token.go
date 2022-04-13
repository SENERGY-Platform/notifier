package controller

import (
	"context"
	"encoding/json"
	"firebase.google.com/go/v4/errorutils"
	"firebase.google.com/go/v4/messaging"
	"github.com/SENERGY-Platform/notifier/pkg/auth"
	"github.com/SENERGY-Platform/notifier/pkg/model"
	"log"
	"time"
)

func (this *Controller) PutFcmToken(token auth.Token, fcmToken string) (err error, errCode int) {
	return this.db.UpsertFcmToken(model.FcmToken{
		Token:     fcmToken,
		UserId:    token.GetUserId(),
		UpdatedAt: time.Now(),
	})
}

func (this *Controller) DeleteFcmToken(token auth.Token, fcmToken string) (err error, errCode int) {
	return this.db.DeleteFcmToken(model.FcmToken{
		Token:  fcmToken,
		UserId: token.GetUserId(),
	})
}

func (this *Controller) handleFCMNotificationUpdate(userId string, notification model.Notification) {
	if this.firebaseClient == nil {
		log.Println("WARNING: Skipping FCM messaging since client not configured")
		return
	}

	tokens, err := this.getValidTokens(userId)
	if err != nil {
		log.Println("ERROR:", err.Error())
		return
	}
	if tokens == nil || len(tokens) == 0 {
		return
	}

	encoded, _ := json.Marshal(notification)

	message := &messaging.MulticastMessage{
		Tokens: tokens,
		Data: map[string]string{
			"type":    model.WsUpdateSetType,
			"payload": string(encoded),
		},
	}

	if !notification.IsRead {
		message.Notification = &messaging.Notification{
			Title: notification.Title,
			Body:  notification.Message,
		}
		message.Android = &messaging.AndroidConfig{
			Priority: "high",
		}
	}

	responses, err := this.firebaseClient.SendMulticast(context.Background(), message)
	if err != nil {
		log.Println("ERROR:", err.Error())
		return
	}
	this.handleFcmResponses(responses, tokens, userId)
}

func (this *Controller) handleFCMNotificationDelete(userId string, ids []string) {
	if this.firebaseClient == nil {
		log.Println("WARNING: Skipping FCM messaging since client not configured")
		return
	}

	tokens, err := this.getValidTokens(userId)
	if err != nil {
		log.Println("ERROR:", err.Error())
		return
	}
	if tokens == nil || len(tokens) == 0 {
		return
	}

	encoded, _ := json.Marshal(ids)

	responses, err := this.firebaseClient.SendMulticast(context.Background(), &messaging.MulticastMessage{
		Tokens: tokens,
		Data: map[string]string{
			"type":    model.WsUpdateDeleteManyType,
			"payload": string(encoded),
		},
	})
	if err != nil {
		log.Println("ERROR:", err.Error())
		return
	}
	this.handleFcmResponses(responses, tokens, userId)
}

func (this *Controller) handleFcmResponses(responses *messaging.BatchResponse, tokens []string, userId string) {
	if responses.FailureCount > 0 {
		for i := range responses.Responses {
			if responses.Responses[i].Error != nil {
				if errorutils.IsNotFound(responses.Responses[i].Error) {
					err, _ := this.db.DeleteFcmToken(model.FcmToken{
						Token:  tokens[i],
						UserId: userId,
					})
					if err != nil {
						log.Println("ERROR: could not delete outdated token for user " + userId + ": " + err.Error())
					}
				} else {
					log.Println("ERROR: sending fcm notification ", responses.Responses[i].MessageID, responses.Responses[i].Error.Error())
				}
			}
		}
	}
}

func (this *Controller) getValidTokens(userId string) (tokens []string, err error) {
	fcmTokens, err := this.db.GetFcmTokens(userId)
	if err != nil {
		return nil, err
	}
	tokens = []string{}

	for _, fcmToken := range fcmTokens {
		if time.Now().Sub(fcmToken.UpdatedAt) > time.Hour*24*60 { // older than two months
			err, _ = this.db.DeleteFcmToken(fcmToken)
			if err != nil {
				log.Println("ERROR:", err.Error())
				// best effort
			}
		} else {
			tokens = append(tokens, fcmToken.Token)
		}
	}
	return
}
