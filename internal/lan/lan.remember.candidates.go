package lan

import (
	"context"
	"database/sql"
	"errors"

	"github.com/egdaemon/eg/compute"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/stringsx"
	"github.com/libp2p/go-libp2p/core/peer"
)

// RememberCandidates records what the control plane reports about the candidate runners, keeping
// the addresses and operator settings already known for them. candidates without a peer id are skipped.
func RememberCandidates(ctx context.Context, db *sql.DB, candidates ...*compute.Compute) (err error) {
	for _, c := range candidates {
		if stringsx.Blank(c.P2Pid) {
			continue
		}

		var p Peer

		if err = PeerFindByID(ctx, db, c.P2Pid).Scan(&p); errors.Is(err, sql.ErrNoRows) {
			p = Peer{PeerID: c.P2Pid}
			if id, cause := peer.Decode(c.P2Pid); cause == nil {
				if pub, cause := id.ExtractPublicKey(); cause == nil {
					p.PublicKey, _ = pub.Raw()
				}
			}
		} else if err != nil {
			return errorsx.Wrap(err, "unable to lookup system")
		}

		p.Description = stringsx.FirstNonBlank(c.Description, p.Description)
		p.OS = c.Os
		p.Arch = c.Arch
		p.Cores = int64(c.Cores)
		p.Memory = int64(c.Memory)
		p.Vram = int64(c.Vram)
		p.Labels = EncodeList(c.Labels)
		p.Uploadable = c.Uploadable

		if err = PeerUpsert(ctx, db, p).Scan(&p); err != nil {
			return errorsx.Wrap(err, "unable to remember system")
		}
	}

	return nil
}
