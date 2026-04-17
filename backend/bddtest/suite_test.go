package bddtest

import (
	"context"
	"database/sql"
	"net/http"
	"testing"

	"github.com/brian/config-generation/backend/dbtest"
	"github.com/brian/config-generation/backend/handlers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var (
	testDB     *sql.DB
	dbCleanup  func()
	router     http.Handler
	jwtSecret  = []byte("test-secret-key-for-bdd-tests")
)

func TestBDD(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Config Generation BDD Suite")
}

var _ = BeforeSuite(func() {
	ctx := context.Background()
	var err error
	testDB, dbCleanup, err = dbtest.StartPostgres(ctx)
	Expect(err).NotTo(HaveOccurred())

	router = handlers.NewRouter(testDB, jwtSecret)
})

var _ = AfterSuite(func() {
	if testDB != nil {
		testDB.Close()
	}
	if dbCleanup != nil {
		dbCleanup()
	}
})
