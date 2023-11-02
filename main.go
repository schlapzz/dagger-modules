package main

import (
	"context"
)

type Modest struct{}

func (m *Modest) GetSecret(ctx context.Context) (string, error) {

	return "", nil
}
