package main

import (
	"context"
	"log"

	"github.com/dalmarcogd/dock-test/pkg/tracer"
)

func main() {
	t, err := tracer.New("localhost:55681", "dock-test", "local", "1.0.0")
	if err != nil {
		log.Panic(err)
	}

	ctx := context.Background()

	defer func() {
		err := t.Stop(ctx)
		if err != nil {
			log.Fatal(err)
		}
	}()

	_, span := t.Span(ctx)
	defer span.End()
}
