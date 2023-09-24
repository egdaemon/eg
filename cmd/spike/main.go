package main

import (
	"log"
	"os"
	"time"

	"github.com/james-lawrence/eg/internal/iox"
	"github.com/james-lawrence/eg/internal/protobuflog"
	"github.com/james-lawrence/eg/interp/events"
)

func main() {
	log.SetFlags(log.Flags() | log.Lshortfile)
	const (
		ngen = 100
		path = "proto.log"
	)

	var (
		err       error
		fh        *os.File
		generated []*events.Message
		decoded   = make([]*events.Message, 0, ngen+5)
		ts        = time.Now().Truncate(time.Hour)
	)

	if fh, err = os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0600); err != nil {
		log.Fatalln(err)
	}
	defer fh.Close()

	encoder := protobuflog.NewEncoder[*events.Message](fh)

	generated = append(generated, events.NewPreambleV0(ts, ts.Add(time.Hour)))
	for i := 0; i < ngen; i++ {
		generated = append(generated, events.NewHeartbeat())
	}

	if err = encoder.Encode(generated...); err != nil {
		log.Fatalln(err)
	}

	if err = iox.Rewind(fh); err != nil {
		log.Fatalln(err)
	}

	decoder := protobuflog.NewDecoder[*events.Message](fh)
	log.Println("decoding", len(decoded), "/", cap(decoded))
	if err = decoder.Decode(&decoded); err != nil {
		log.Fatalln(err)
	}

	log.Println("generated", len(generated))
	log.Println("decoded", len(decoded))

	// for _, m := range decoded {
	// 	log.Println(spew.Sdump(m))
	// }
	// g := graph.New(func(n *ux.Node) string { return n.ID }, graph.Directed(), graph.PreventCycles(), graph.Tree())

	// const (
	// 	path = "derp.dot"
	// )

	// var (
	// 	fh   *os.File
	// 	rawg []byte
	// 	rg   *gographviz.Graph
	// )

	// if fh, err = os.Open(path); err != nil {
	// 	log.Fatalln(err)
	// }
	// defer fh.Close()

	// if rawg, err = io.ReadAll(fh); err != nil {
	// 	log.Fatalln(err)
	// }

	// if rg, err = gographviz.Read(rawg); err != nil {
	// 	log.Fatalln(err)
	// }

	// if g, err = ux.TranslateGraphiz(rg); err != nil {
	// 	log.Fatalln(err)
	// }

	// p := tea.NewProgram(ux.NewGraph(g))

	// go func() {
	// 	for {
	// 		time.Sleep(time.Second)
	// 		nodes, _ := graph.TopologicalSort(g)
	// 		choice := rand.Intn(len(nodes))
	// 		n, _ := g.Vertex(nodes[choice])
	// 		p.Send(ux.EventTask{ID: n.ID, State: (n.State + 1) % (ux.StateError + 1)})
	// 	}
	// }()

	// if _, err := p.Run(); err != nil {
	// 	fmt.Printf("Alas, there's been an error: %v", err)
	// 	os.Exit(1)
	// }

	// err = draw.DOT(g, langx.Must(os.OpenFile("derp2.dot", os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0600)))
	// if err != nil {
	// 	log.Fatalln(err)
	// }
}
