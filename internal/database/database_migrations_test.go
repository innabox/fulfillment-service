/*
Copyright (c) 2025 Red Hat Inc.

Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the
License. You may obtain a copy of the License at

  http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an
"AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific
language governing permissions and limitations under the License.
*/

package database

import (
	"path/filepath"

	. "github.com/onsi/ginkgo/v2/dsl/core"
	. "github.com/onsi/gomega"
)

var _ = Describe("Migrations", func() {
	It("All migrations have the '.up.sql' suffix", func() {
		files, err := filepath.Glob("migrations/*.sql")
		Expect(err).ToNot(HaveOccurred())
		Expect(files).ToNot(BeEmpty())
		for _, file := range files {
			Expect(file).To(MatchRegexp(`\.up\.sql$`))
		}
	})
})
