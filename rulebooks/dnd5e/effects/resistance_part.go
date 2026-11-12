// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package effects

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"

	generic "github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
	"github.com/KirkDiggler/rpg-toolkit/mechanics/effects/machine"
)

// bindResistancePart supplies only payload structure. Its validation, incoming
// fold and information meaning are exactly the existing resistance kind's.
func bindResistancePart(in *BindPartInput) (boundPart, error) {
	if in.Declaration.OnTerminal != generic.EndPart {
		return boundPart{}, fmt.Errorf("resistance payload requires local part scope")
	}
	out := generic.Output{Kind: KindResistance, Params: jsontext.Value(in.Declaration.Params)}
	if err := checkResistance(out); err != nil {
		return boundPart{}, err
	}
	raw, err := json.Marshal(out)
	if err != nil {
		return boundPart{}, err
	}
	return boundPart{recipe: machine.Recipe{
		Ref: in.Ref, Initial: partStateActive, Bag: machine.Bag{},
		States: map[string]machine.State{partStateActive: {Outputs: []jsontext.Value{raw}}},
	}}, nil
}

func inspectResistancePart(in *InspectPartInput) (*PartInfo, error) {
	if in.Declaration.OnTerminal != generic.EndPart {
		return nil, fmt.Errorf("resistance payload requires local part scope")
	}
	if err := checkResistance(generic.Output{Kind: KindResistance, Params: jsontext.Value(in.Declaration.Params)}); err != nil {
		return nil, err
	}
	if in.Snapshot.State != partStateActive || in.Snapshot.Bag == nil || len(in.Snapshot.Bag) != 0 {
		return nil, fmt.Errorf("resistance requires active state and an explicit empty bag")
	}
	return &PartInfo{}, nil
}
