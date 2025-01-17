package daemons

import (
	"context"
	"log"
	"net"

	"github.com/egdaemon/eg/internal/contextx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/pion/ice/v4"
	"github.com/pion/stun/v3"
	"golang.org/x/crypto/ssh"
)

func P2PProxy(ctx context.Context, signer ssh.Signer, httpl net.Listener) (err error) {
	var (
		agent *ice.Agent
	)

	if agent, err = ice.NewAgent(&ice.AgentConfig{
		LocalUfrag: "JqpzGrjUvjhNAadT",
		LocalPwd:   "DqqhpxAivceHFObeDqMkHxrqluEzGTRf",
		Urls:       []*stun.URI{errorsx.Must(stun.ParseURI("stun:stun.l.google.com:19302"))},
		BindingRequestHandler: func(m *stun.Message, local, remote ice.Candidate, pair *ice.CandidatePair) bool {
			log.Println("STUN MESSAGE", local.ID(), local.Address(), "->", remote.ID(), remote.Address())
			return true
		},
		// Lite:           true,
		CandidateTypes: []ice.CandidateType{ice.CandidateTypeHost, ice.CandidateTypePeerReflexive, ice.CandidateTypeServerReflexive},
	}); err != nil {
		return errorsx.Wrap(err, "unable to initialize ice agent")
	}

	agent.OnConnectionStateChange(func(cs ice.ConnectionState) {
		log.Println("ice connection state change", cs.String())
	})

	agent.OnCandidate(func(c ice.Candidate) {
		if c == nil {
			return
		}

		log.Println("candidate", c.ID(), c.Address(), c.Type().String())
	})

	go func() {
		contextx.WaitGroupAdd(ctx, 1)
		defer contextx.WaitGroupDone(ctx)
		<-ctx.Done()
		agent.Close()
	}()

	// go func() {
	// 	for {

	// 		time.Sleep(3 * time.Second)
	// 	}
	// }()
	if err = agent.GatherCandidates(); err != nil {
		log.Println("gather candidates failed", err)
	}
	// agent.AddRemoteCandidate(ice.Peer())
	// Get the local auth details and send to remote peer
	localufrag, localpwd, err := agent.GetLocalUserCredentials()
	if err != nil {
		return errorsx.Wrap(err, "unable to determine local user credentials for ice")
	}
	log.Println("ice credentials", localufrag, localpwd)

	conn, err := agent.Accept(ctx, "UDIAfhnZRVOFyLNh", "LexqyzofvnJzHzLaCUnqifWQkRxJGpVz")
	if err != nil {
		return errorsx.Wrap(err, "unable accept remote connection")
	}
	defer conn.Close()

	return nil
}
