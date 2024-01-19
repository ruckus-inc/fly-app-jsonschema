package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"

	"github.com/superfly/flyctl/internal/appconfig"
	jsonschema "github.com/swaggest/jsonschema-go"
)

func main() {
	reflector := jsonschema.Reflector{}
	reflector.DefaultOptions = append(reflector.DefaultOptions, jsonschema.InterceptDefName(
		func(t reflect.Type, defaultDefName string) string {
			defName := defaultDefName
			defName = strings.TrimPrefix(defName, "Appconfig")
			defName = strings.TrimPrefix(defName, "Api")
			return defName
		},
	))

	schema, err := reflector.Reflect(appconfig.Config{})
	title := "FlyAppConfig"
	schema.Title = &title
	if err != nil {
		panic(err)
	}
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
