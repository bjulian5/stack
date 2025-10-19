package main

import (
	"context"

	"github.com/bjulian5/stack/cmd"
)

func main() {
	ctx := context.Background()
	cmd.Execute(ctx)
}
