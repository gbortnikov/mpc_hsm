package server

import (
	pb "github.com/mpc_hsm/node/proto"
)

// findPartyIndex находит индекс текущей ноды среди участников
func (s *MPCNodeServer) findPartyIndex(parties []*pb.PartyInfo) int {
	for _, p := range parties {
		if p.PartyId == s.partyID {
			return int(p.PartyIndex)
		}
	}
	return 0
}
