package core

import "context"

type Command func(ctx context.Context) error

type Suplier interface {
	Run(ctx context.Context, cmd Command) error
}
