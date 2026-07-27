package websocket

import "errors"

// ReplayGuard enforces monotonic per-direction sequence numbers.
type ReplayGuard struct {
	last uint64
	seen map[uint64]struct{}
}

func NewReplayGuard() *ReplayGuard {
	return &ReplayGuard{seen: make(map[uint64]struct{})}
}

func (g *ReplayGuard) Accept(seq uint64) error {
	if seq == 0 {
		return errors.New("sequence must be > 0")
	}
	if _, ok := g.seen[seq]; ok {
		return errors.New("duplicate sequence")
	}
	if seq <= g.last {
		return errors.New("replayed or older sequence")
	}
	g.seen[seq] = struct{}{}
	g.last = seq
	if len(g.seen) > 4096 {
		g.seen = map[uint64]struct{}{seq: {}}
	}
	return nil
}

func (g *ReplayGuard) Reset() {
	g.last = 0
	g.seen = make(map[uint64]struct{})
}
