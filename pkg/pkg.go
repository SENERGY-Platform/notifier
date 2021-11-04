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

package pkg

import (
	"context"
	"github.com/SENERGY-Platform/notifier/pkg/api"
	"github.com/SENERGY-Platform/notifier/pkg/configuration"
	"github.com/SENERGY-Platform/notifier/pkg/controller"
	"github.com/SENERGY-Platform/notifier/pkg/persistence/mongo"
	"sync"
)

func Start(ctx context.Context, wg *sync.WaitGroup, config configuration.Config) (err error) {
	db, err := mongo.New(config)
	if err != nil {
		return err
	}
	wg.Add(1)
	go func() {
		<-ctx.Done()
		db.Disconnect()
		wg.Done()
	}()
	ctrl := controller.New(config, db)
	api.Start(ctx, wg, config, ctrl)
	return nil
}
