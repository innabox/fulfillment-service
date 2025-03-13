/*
Copyright (c) 2025 Red Hat, Inc.

Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the
License. You may obtain a copy of the License at

  http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an
"AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific
language governing permissions and limitations under the License.
*/

package database

import (
	"log/slog"
	"testing"

	. "github.com/onsi/ginkgo/v2/dsl/core"
	. "github.com/onsi/gomega"

	"github.com/innabox/fulfillment-service/internal/logging"
	. "github.com/innabox/fulfillment-service/internal/testing"
)

func TestDatabase(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Database")
}

// logger is the logger that will be used by the tests.
var logger *slog.Logger

// dbServer is the database dbServer that will be used to create the databases used by the tests.
var dbServer *DatabaseServer

var _ = BeforeSuite(func() {
	var err error

	// Create a logger that writes to the Ginkgo stream:
	logger, err = logging.NewLogger().
		SetOut(GinkgoWriter).
		Build()
	Expect(err).ToNot(HaveOccurred())

	// Start the database server:
	dbServer = MakeDatabaseServer()
})

var _ = AfterSuite(func() {
	// Stop the database server:
	dbServer.Close()
})
