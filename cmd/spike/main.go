package main

import (
	"io/ioutil"
	"log"
	"net/http"

	"github.com/james-lawrence/eg/internal/bytesx"
	"github.com/pbnjay/memory"
)

func main() {
	log.Println("DERP", memory.TotalMemory()/bytesx.GiB)
	// log.SetFlags(log.Flags() | log.Lshortfile)

	// var (
	// 	err       error
	// 	id        = uuid.Must(uuid.NewV7())
	// 	generated []*events.Message
	// 	ts        = time.Now().Truncate(time.Hour)
	// 	ctx, done = context.WithCancel(context.Background())
	// 	ragent    *runners.Agent
	// )

	// dir := langx.Must(filepath.Abs(runners.DefaultManagerDirectory()))

	// if err := os.MkdirAll(dir, 0700); err != nil {
	// 	log.Fatalln(err)
	// }

	// log.Println("DAEMON DIR", dir)
	// m := runners.NewManager(ctx, dir)
	// defer time.Sleep(30 * time.Second)
	// defer done()

	// if ragent, err = m.NewRun(ctx, workspaces.Context{}, id.String()); err != nil {
	// 	log.Fatalln(err)
	// }

	// cc, err := ragent.Dial(ctx)
	// if err != nil {
	// 	log.Fatalln(err)
	// }

	// generated = append(generated, events.NewPreambleV0(ts, ts.Add(time.Hour)))
	// for i := 0; i < 100; i++ {
	// 	generated = append(generated, events.NewHeartbeat())
	// }

	// if _, err = events.NewEventsClient(cc).Dispatch(ctx, &events.DispatchRequest{Messages: generated}); err != nil {
	// 	log.Fatalln(err)
	// }
	res, err := http.Get("http://127.0.0.1:45571")
	if err != nil {
		log.Fatalln(err)
	}

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		log.Fatalln(err)
	}

	log.Println("DERP", string(body))
}
