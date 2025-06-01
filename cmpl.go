package otto

import (
	"github.com/nate-anderson/otto/ast"
	"github.com/nate-anderson/otto/file"
)

type compiler struct {
	file    *file.File
	program *ast.Program
}
