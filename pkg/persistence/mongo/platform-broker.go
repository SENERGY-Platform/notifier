package mongo

import (
	"github.com/SENERGY-Platform/notifier/pkg/model"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"net/http"
)

var platformBrokerUserIdKey = "user_id"

func initPlatformBrokers() {
	CreateCollections = append(CreateCollections, func(db *Mongo) error {
		collection := db.client.Database(db.config.MongoTable).Collection(db.config.MongoPlatformBrokerCollection)
		return db.ensureIndex(collection, "platformbrokeruseridindex", platformBrokerUserIdKey, true, true)
	})
}

func (this *Mongo) platformBrokerCollection() *mongo.Collection {
	return this.client.Database(this.config.MongoTable).Collection(this.config.MongoPlatformBrokerCollection)
}

func (this *Mongo) ReadPlatformBroker(userId string) (result model.PlatformBroker, err error, errCode int) {
	ctx, _ := getTimeoutContext()
	temp := this.platformBrokerCollection().FindOne(
		ctx,
		bson.M{
			platformBrokerUserIdKey: userId,
		})
	err = temp.Err()
	if err == mongo.ErrNoDocuments {
		return result, err, http.StatusNotFound
	}
	if err != nil {
		return result, err, http.StatusInternalServerError
	}
	err = temp.Decode(&result)
	if err == mongo.ErrNoDocuments {
		return result, err, http.StatusNotFound
	}
	if err != nil {
		return result, err, http.StatusInternalServerError
	}
	return result, nil, http.StatusOK
}

func (this *Mongo) SetPlatformBroker(broker model.PlatformBroker) (error, int) {
	ctx, _ := getTimeoutContext()
	_, err := this.platformBrokerCollection().ReplaceOne(
		ctx,
		bson.M{
			platformBrokerUserIdKey: broker.UserId,
		},
		broker,
		options.Replace().SetUpsert(true))
	if err != nil {
		return err, http.StatusInternalServerError
	}
	return nil, http.StatusOK
}

func (this *Mongo) RemovePlatformBroker(userId string) (error, int) {
	ctx, _ := getTimeoutContext()
	collection := this.platformBrokerCollection()

	filter := bson.M{
		platformBrokerUserIdKey: userId,
	}

	_, err := collection.DeleteOne(ctx, filter)
	if err != nil {
		return err, http.StatusInternalServerError
	}
	return nil, http.StatusOK
}
