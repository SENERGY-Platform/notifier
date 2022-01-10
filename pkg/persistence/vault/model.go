package vault

import "github.com/SENERGY-Platform/notifier/pkg/model"

type SecretBrokerData struct {
	Address  string `json:"address"`
	User     string `json:"user"`
	Password string `json:"password"`
}

func secretBrokerDataFromBroker(broker *model.Broker) *SecretBrokerData {
	return &SecretBrokerData{
		Address:  broker.Address,
		User:     broker.User,
		Password: broker.Password,
	}
}

func (secret *SecretBrokerData) fillModel(broker *model.Broker) {
	broker.Address = secret.Address
	broker.User = secret.User
	broker.Password = secret.Password
}

func stripBroker(broker *model.Broker) {
	broker.Address = ""
	broker.User = ""
	broker.Password = ""
}

func NeedsMigration(broker *model.Broker) bool {
	return broker.Address != "" ||
		broker.User != "" ||
		broker.Password != ""
}
