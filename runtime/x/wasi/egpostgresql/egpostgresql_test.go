package egpostgresql_test

import (
	"context"
	"log"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/x/wasi/egpostgresql"
)

func ExampleAuto() {
	var (
		err error
	)

	ctx := context.Background()
	err = eg.Perform(
		ctx,
		egpostgresql.Auto,                        // wait for postgresql to become ready
		egpostgresql.RecreateDatabase("example"), // create a database.
		egpostgresql.InsertSuperuser("soandso"),  // create a superuser to use.
	)
	if err != nil {
		log.Fatalln(err)
	}
}
