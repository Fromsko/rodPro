package input_test

import (
	"testing"

	"github.com/Fromsko/rodPro/lib/input"
	"github.com/Fromsko/rodPro/lib/proto"
	"github.com/ysmood/got"
)

func TestMouseEncode(t *testing.T) {
	g := got.T(t)

	b, flag := input.EncodeMouseButton([]proto.InputMouseButton{proto.InputMouseButtonLeft})

	g.Eq(b, proto.InputMouseButtonLeft)
	g.Eq(flag, 1)
}
