package engine

import (
	"testing"

	"github.com/xhd2015/xgo/runtime/mock"
)

func TestNewAlertExistPoli(t *testing.T) {
	mock.Patch(Store.FindOrCreateAlert, func() bool {
		return true
	})
}
