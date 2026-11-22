// Package fixtures holds the data that seeds development and tests: one city
// and two plans (docs/spec/architecture.md, "The seams"; docs/spec/build-order.md,
// "The milestones").
//
// It is only bytes. The packages that own the tables read their own part of it
// (cities.LoadFixture, plans.LoadFixture), and the only code that loads it
// outside a test is cmd/seed, which is not in the production image. No process
// the image holds imports this package: TestProductionProcessesDoNotImportTheFixtures
// (cmd/cmd_test.go) fails if one does.
package fixtures

import _ "embed"

// DaNangHoiAn is the fixture city, Da Nang–Hoi An, with two plans, as JSON.
//
//go:embed da-nang-hoi-an.json
var DaNangHoiAn []byte
