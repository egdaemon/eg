// the operation body is transferred verbatim across the breadth of go statements.
package main

import (
	"context"
	"log"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
)

func main() {
	ctx, done := context.WithTimeout(context.Background(), egenv.TTL())
	defer done()

	limit := 3
	items := []string{"a", "b"}

	err := eg.Perform(
		ctx,
		eg.Module(
			ctx,
			eg.DefaultModule(),
			func(ctx context.Context, op eg.Op) error {
				defer log.Println("done")
				results := make(chan int, limit)
				var wg [2]struct{}
				for i := range limit {
					go func() { results <- i }()
				}
			loop:
				for {
					select {
					case v := <-results:
						log.Println(v)
						if v == limit-1 {
							break loop
						}
					case <-ctx.Done():
						return ctx.Err()
					default:
						continue loop
					}
				}
				for idx, item := range items {
					switch {
					case idx == 0:
						log.Println(item)
						fallthrough
					case idx > 5:
						log.Println("big")
					default:
					}
				}
				var anything any = limit
				switch v := anything.(type) {
				case int:
					log.Println(v<<1, 1+2i, 'r', `raw`, len(wg))
				case string, []byte:
					log.Println(v)
				}
				if err := ctx.Err(); err != nil {
					goto end
				}
			end:
				return nil
			},
		),
	)
	if err != nil {
		log.Fatalln(err)
	}
}
