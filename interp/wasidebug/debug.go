package wasidebug

import (
	"log"
	"strings"

	"github.com/tetratelabs/wazero/api"
)

type module interface {
	Name() string
	ExportedFunctions() map[string]api.FunctionDefinition
	ImportedFunctions() []api.FunctionDefinition
}

type host interface {
	Name() string
	ExportedFunctionDefinitions() map[string]api.FunctionDefinition
}

func Module(m module) {
	log.Println("module debug", m.Name())
	for _, imp := range m.ExportedFunctions() {
		paramtypestr := typeliststr(imp.ParamTypes()...)
		resulttypestr := typeliststr(imp.ResultTypes()...)
		log.Println("exported", imp.Name(), "(", paramtypestr, ")", resulttypestr)
	}

	for _, imp := range m.ImportedFunctions() {
		paramtypestr := typeliststr(imp.ParamTypes()...)
		resulttypestr := typeliststr(imp.ResultTypes()...)
		log.Println("imported", imp.Name(), "(", paramtypestr, ")", resulttypestr)
	}
}

func Host(m host) {
	log.Println("module debug", m.Name())
	for _, imp := range m.ExportedFunctionDefinitions() {
		paramtypestr := typeliststr(imp.ParamTypes()...)
		resulttypestr := typeliststr(imp.ResultTypes()...)
		log.Println("exported", imp.Name(), "(", paramtypestr, ")", resulttypestr)
	}
}

func typeliststr(types ...api.ValueType) string {
	typesstr := []string(nil)
	for _, t := range types {
		typesstr = append(typesstr, api.ValueTypeName(t))
	}

	return strings.Join(typesstr, ", ")
}
