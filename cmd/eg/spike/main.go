package main

import (
	"context"
	"log"
	"os"
	"sync"

	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/pion/ice/v4"
	"github.com/pion/stun/v3"
)

func main() {
	var (
		err   error
		agent *ice.Agent
		conn  *ice.Conn
	)

	log.SetFlags(log.Flags() | log.Lshortfile)
	ctx, done := context.WithCancelCause(context.Background())
	go cmdopts.Cleanup(ctx, done, &sync.WaitGroup{}, func() {
		log.Println("waiting for systems to shutdown")
	}, os.Kill, os.Interrupt)

	log.Println("generating agent")
	if agent, err = ice.NewAgent(&ice.AgentConfig{
		LocalUfrag: "UDIAfhnZRVOFyLNh",
		LocalPwd:   "LexqyzofvnJzHzLaCUnqifWQkRxJGpVz",
		Urls:       []*stun.URI{errorsx.Must(stun.ParseURI("stun:stun.l.google.com:19302"))},
		BindingRequestHandler: func(m *stun.Message, local, remote ice.Candidate, pair *ice.CandidatePair) bool {
			log.Println("STUN MESSAGE", local.ID(), local.Address(), "->", remote.ID(), remote.Address())
			return true
		},
		CandidateTypes: []ice.CandidateType{ice.CandidateTypeHost, ice.CandidateTypePeerReflexive, ice.CandidateTypeServerReflexive},
	}); err != nil {
		log.Fatalln(errorsx.Wrap(err, "unable to initialize ice agent"))
	}

	localufrag, localpwd, err := agent.GetLocalUserCredentials()
	if err != nil {
		log.Fatalln(errorsx.Wrap(err, "unable to determine local user credentials for ice"))
	}
	log.Println("ice credentials", localufrag, localpwd)

	agent.OnCandidate(func(c ice.Candidate) {
		if c == nil {
			return
		}

		log.Println("candidate", c.ID(), c.Address(), c.Type().String())
	})

	if err = agent.GatherCandidates(); err != nil {
		log.Println("gather candidates failed", err)
	}

	log.Println("dialing agent")
	if conn, err = agent.Dial(ctx, "JqpzGrjUvjhNAadT", "DqqhpxAivceHFObeDqMkHxrqluEzGTRf"); err != nil {
		log.Fatalln(errorsx.Wrap(err, "unable to diealing failed"))
	}

	log.Println("CONNECT SUCCESSFULLY", conn.RemoteAddr().String())
}
