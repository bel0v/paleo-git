package engine

import (
	"fmt"

	"github.com/bel0v/paleo-git/config"
)

type ScanOptions struct {
	AlreadyMeasured []string
}

func Measure(cfg config.Config, commit string) ([]Result, error) {
	return nil, fmt.Errorf("not implemented")
}

func Scan(cfg config.Config, opts ScanOptions, onResult func(Result)) error {
	return fmt.Errorf("not implemented")
}
