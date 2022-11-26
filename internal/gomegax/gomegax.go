package gomegax

import (
	"fmt"
	"os"

	"github.com/onsi/gomega/format"
	"github.com/onsi/gomega/types"
)

// HaveFilePermissions ...
func HaveFilePermissions(m os.FileMode) types.GomegaMatcher {
	return &haveFilePermissions{expected: m}
}

// haveFilePermissions ...
type haveFilePermissions struct {
	expected   os.FileMode
	actualMode os.FileMode
}

// Match ...
func (t *haveFilePermissions) Match(actual interface{}) (success bool, err error) {
	actualFilename, ok := actual.(string)
	if !ok {
		return false, fmt.Errorf("HaveFilePermissions matcher expects a path")
	}

	fileInfo, err := os.Stat(actualFilename)
	if err != nil {
		return false, err
	}
	t.actualMode = fileInfo.Mode().Perm()

	return fileInfo.Mode().Perm() == t.expected, nil
}

// FailureMessage ...
func (t *haveFilePermissions) FailureMessage(actual interface{}) (message string) {
	return format.Message(actual, fmt.Sprintf("%s to equal %s", t.expected, t.actualMode))
}

// NegatedFailureMessage ...
func (t *haveFilePermissions) NegatedFailureMessage(actual interface{}) (message string) {
	return format.Message(actual, fmt.Sprintf("%s not to equal %s", t.expected, t.actualMode))
}
