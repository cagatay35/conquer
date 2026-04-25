// Example: ETL pipeline using conquer.Pipeline2.
//
// Simulates reading raw records, transforming them, and writing results.
package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/cagatay35/conquer"
)

type RawRecord struct {
	ID   int
	Data string
}

type ProcessedRecord struct {
	ID   int
	Data string
	Hash int
}

func main() {
	ctx := context.Background()

	source := make(chan RawRecord, 10)
	go func() {
		defer close(source)
		for i := 1; i <= 20; i++ {
			source <- RawRecord{ID: i, Data: fmt.Sprintf("record-%d", i)}
		}
	}()

	out, errCh := conquer.Pipeline2(
		ctx,
		source,
		conquer.NewStage("transform", func(ctx context.Context, in RawRecord) (ProcessedRecord, error) {
			return ProcessedRecord{
				ID:   in.ID,
				Data: strings.ToUpper(in.Data),
				Hash: len(in.Data) * 31,
			}, nil
		}, 4),
		conquer.NewStage("format", func(ctx context.Context, in ProcessedRecord) (string, error) {
			return fmt.Sprintf("[%03d] %s (hash=%d)", in.ID, in.Data, in.Hash), nil
		}, 2),
	)

	for result := range out {
		fmt.Println(result)
	}

	if err := <-errCh; err != nil {
		log.Fatalf("Pipeline error: %v", err)
	}

	fmt.Println("\nPipeline completed successfully!")
}
