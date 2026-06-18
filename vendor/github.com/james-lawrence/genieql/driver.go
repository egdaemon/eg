package genieql

import (
	"fmt"
	"go/ast"
	"log"
	"os"
	"path/filepath"

	"github.com/davecgh/go-spew/spew"
	"github.com/james-lawrence/genieql/internal/debugx"
	"github.com/james-lawrence/genieql/internal/errorsx"
	"gopkg.in/yaml.v3"
)

// ErrMissingDriver - returned when a driver has not been registered.
type missingDriver struct {
	driver string
}

func (t missingDriver) Error() string {
	return fmt.Sprintf("requested driver is not registered: '%s'", t.driver)
}

// ErrDuplicateDriver - returned when a ddriver gets registered twice.
var ErrDuplicateDriver = fmt.Errorf("driver has already been registered")

var drivers = driverRegistry{}

// LookupTypeDefinition converts a expression into a type definition.
type LookupTypeDefinition func(typ ast.Expr) (ColumnDefinition, error)

// RegisterDriver register a database driver with genieql. usually in an init function.
func RegisterDriver(driver string, imp Driver) error {
	return drivers.RegisterDriver(driver, imp)
}

// LookupDriver lookup a registered driver.
func LookupDriver(name string) (Driver, error) {
	return drivers.LookupDriver(name)
}

// PrintRegisteredDrivers print drivers in the registry, debugging utility.
func PrintRegisteredDrivers() {
	for key := range map[string]Driver(drivers) {
		log.Println("Driver", key)
	}
}

// Driver - driver specific details.
type Driver interface {
	LookupType(s string) (ColumnDefinition, error)
	AddColumnDefinitions(supported ...ColumnDefinition)
}

func LoadCustomColumnTypes(c Configuration, d Driver) (Driver, error) {
	var (
		err   error
		raw   []byte
		cfg   []ColumnDefinition
		dpath = filepath.Join(c.Location, "driver.yml")
	)

	if raw, err = os.ReadFile(dpath); os.IsNotExist(err) {
		return d, nil
	} else if err != nil {
		return nil, errorsx.Wrapf(err, "failed to read driver file: %s", dpath)
	}

	if err = errorsx.Wrapf(yaml.Unmarshal(raw, &cfg), "failed to parse driver file: %s", dpath); err != nil {
		return nil, err
	}

	if len(cfg) > 0 {
		debugx.Println("customizations detected", spew.Sdump(cfg))
		d.AddColumnDefinitions(cfg...)
	}

	return d, nil
}

// ColumnDefinition defines a type supported by the driver.
type ColumnDefinition struct {
	Type       string // dialect type
	Native     string // golang type
	DBTypeName string `yaml:"database_type_name"`
	ColumnType string `yaml:"column_type"` // sql type
	Nullable   bool   // does this type represent a pointer type.
	PrimaryKey bool   // is the column part of the primary key
	Decode     string // template function that decodes from the Driver type to Native type
	Encode     string // template function that encodes from the Native type to Driver type
}

type driverRegistry map[string]Driver

func (t driverRegistry) RegisterDriver(driver string, imp Driver) error {
	if _, exists := t[driver]; exists {
		return ErrDuplicateDriver
	}

	t[driver] = imp

	return nil
}

func (t driverRegistry) LookupDriver(name string) (Driver, error) {
	impl, exists := t[name]
	if !exists {
		return nil, missingDriver{driver: name}
	}

	return impl, nil
}

func DebugColumnDefinitions(supported ...ColumnDefinition) {
	for _, typedef := range supported {
		log.Println("column definition debug", typedef.Type, typedef.DBTypeName, spew.Sdump(typedef))
	}
}

// NewDriver builds a new driver from the component parts
func NewDriver(path string, supported ...ColumnDefinition) Driver {
	mapped := make(map[string]ColumnDefinition, len(supported))
	for _, typedef := range supported {
		mapped[typedef.Type] = typedef
		if typedef.DBTypeName != "" {
			mapped[typedef.DBTypeName] = typedef
		}
	}

	return &driver{importPath: path, supported: mapped}
}

type driver struct {
	importPath string
	supported  map[string]ColumnDefinition
}

func (t driver) LookupType(l string) (ColumnDefinition, error) {
	if typedef, ok := t.supported[l]; ok {
		return typedef, nil
	}

	return ColumnDefinition{}, errorsx.Errorf("%s - unsupported type: %s", t.importPath, l)
}

func (t *driver) AddColumnDefinitions(supported ...ColumnDefinition) {
	mapped := make(map[string]ColumnDefinition, len(supported)+len(t.supported))
	for k, v := range t.supported {
		mapped[k] = v
	}

	for _, typedef := range supported {
		mapped[typedef.Type] = typedef
		if typedef.DBTypeName != "" {
			mapped[typedef.DBTypeName] = typedef
		}
	}

	t.supported = mapped

	// DebugColumnDefinitions(slicesx.FromIter(maps.Values(t.supported))...)
}
