// Copyright (C) 2022 Check Point Software Technologies Ltd. All rights reserved.

// Licensed under the Apache License, Version 2.0 (the "License");
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"os"
	"time"

	"openappsec.io/log"
	"openappsec.io/smartsync-service/internal/app"
	"openappsec.io/smartsync-service/internal/app/ingector"
)

const (
	initTimeout    = 15
	stopTimeout    = 15
	exitCauseError = 1
	exitCauseDone  = 0

	envKeyMode = "APPSEC_MODE"
)

func main() {
	initCtx, initCancel := context.WithTimeout(context.Background(), initTimeout*time.Second)

	appsecMode := os.Getenv(envKeyMode)
	isStandAloneMode := appsecMode == "standalone" || appsecMode == "stand-alone"
	var app *app.App
	var err error
	if isStandAloneMode {
		app, err = ingector.InitializeStandAloneApp(initCtx)
		if err != nil {
			log.WithContext(initCtx).Error("Failed to inject dependencies! Error: ", err.Error())
			initCancel()
			os.Exit(exitCauseError)
		}
	} else {
		app, err = ingector.InitializeApp(initCtx)
		if err != nil {
			log.WithContext(initCtx).Error("Failed to inject dependencies! Error: ", err.Error())
			initCancel()
			os.Exit(exitCauseError)
		}
	}
	initCancel()

	exitCode := exitCauseDone
	if err := app.Start(); err != nil {
		log.WithContext(initCtx).Error("Failed to start app: ", err.Error())
		exitCode = exitCauseError
	}

	stopCtx, stopCancel := context.WithTimeout(context.Background(), stopTimeout*time.Second)
	if errs := app.Stop(stopCtx); len(errs) > 0 {
		log.WithContext(context.Background()).Error("Failed to stop app: ", errs)
		exitCode = exitCauseError
	}

	stopCancel()
	os.Exit(exitCode)
}
