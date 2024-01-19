package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"

	"github.com/invopop/jsonschema"
	"github.com/superfly/flyctl/internal/appconfig"
)

func main() {
	schema := jsonschema.Reflect(&appconfig.Config{})
	schema.Ref = ""
	json, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		panic(err)
	}

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		panic("Cant get filename")
	}
	dirname := filepath.Dir(filename)

	err = os.WriteFile(dirname+"/appconfig.json", json, 0644)
	if err != nil {
		panic(err)
	}
}
