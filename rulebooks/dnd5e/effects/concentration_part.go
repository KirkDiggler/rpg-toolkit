// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package effects

import (
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	generic "github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
	"github.com/KirkDiggler/rpg-toolkit/mechanics/effects/machine"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	dndevents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

const (
	reasonConcentrationDamage = "damage"
	reasonMaintainerDown      = "caster_down"
	concentrationFloor        = 10
)

type concentrationOutputParams struct {
	Owner dndevents.ConditionAddress `json:"owner"`
}

func checkUnparameterizedMaintenance(declaration PartDeclaration) error {
	if declaration.OnTerminal != generic.EndApplication {
		return fmt.Errorf("maintenance must end its application")
	}
	if len(declaration.Params) != 0 {
		_, err := decodeParams[struct{}](jsontext.Value(declaration.Params))
		return err
	}
	return nil
}

func bindConcentrationPart(in *BindPartInput) (boundPart, error) {
	if err := checkUnparameterizedMaintenance(in.Declaration); err != nil {
		return boundPart{}, err
	}
	if in.Owner.MemberID != in.HolderID || in.Owner.SourceID != in.ApplicationID {
		return boundPart{}, fmt.Errorf("concentration must bind its own maintainer/application coordinator")
	}
	params, err := json.Marshal(concentrationOutputParams{Owner: in.Owner})
	if err != nil {
		return boundPart{}, err
	}
	out := generic.Output{Kind: KindConcentration, Params: params}
	if err := checkConcentrationOutput(out); err != nil {
		return boundPart{}, err
	}
	raw, err := json.Marshal(out)
	if err != nil {
		return boundPart{}, err
	}
	return boundPart{recipe: machine.Recipe{
		Ref: in.Ref, Initial: partStateActive, Bag: machine.Bag{},
		States: map[string]machine.State{
			partStateActive: {Outputs: []jsontext.Value{raw}, Transitions: []machine.Transition{
				{On: BeatDroppedToZero, About: Holder, To: partStateEnded, Reason: reasonMaintainerDown},
			}},
			partStateEnded: {Terminal: true},
		},
	}, capabilities: PartCapabilities{Concentration: true}}, nil
}

func concentrationMeaning(raw jsontext.Value) (meaning, error) {
	params, err := decodeParams[concentrationOutputParams](raw)
	if err != nil {
		return meaning{}, err
	}
	if strings.TrimSpace(params.Owner.MemberID) == "" || strings.TrimSpace(params.Owner.SourceID) == "" {
		return meaning{}, fmt.Errorf("concentration requires an exact application owner")
	}
	if _, err := core.ParseString(params.Owner.ConditionRef); err != nil {
		return meaning{}, fmt.Errorf("concentration owner ref: %w", err)
	}
	return meaning{domain: domainMaintenance, description: "Maintains this application by concentration; damage can require a Constitution save."}, nil
}

func checkConcentrationOutput(out generic.Output) error {
	if out.Amount != nil {
		return fmt.Errorf("concentration does not take an amount")
	}
	_, err := concentrationMeaning(out.Params)
	return err
}

func attachConcentration(ctx context.Context, bus events.EventBus, out *generic.Bound, in BindInput) (generic.Detach, error) {
	params, err := decodeParams[concentrationOutputParams](out.Params())
	if err != nil {
		return nil, err
	}
	if params.Owner.MemberID != out.Host().Holder {
		return nil, fmt.Errorf("concentration owner is not its maintainer")
	}
	id, err := dndevents.DamageTakenTopic.On(bus).Subscribe(ctx, func(_ context.Context, fact *dndevents.DamageTakenEvent) error {
		if !out.CanReact() || fact == nil || fact.MemberID != out.Host().Holder || fact.Amount <= 0 || fact.DroppedToZero {
			return nil
		}
		dc := max(concentrationFloor, fact.Amount/2)
		source := in.Origin.Ref
		fact.FollowUps = append(fact.FollowUps, dndevents.FollowUp{
			SaverID: out.Host().Holder, Ability: abilities.CON, DC: dc,
			Cause: dndevents.SaveCause{
				Trigger: dndevents.SaveTriggerConcentration, EffectRef: &source,
				InstigatorID: fact.Cause.InstigatorID, InstigatorType: fact.Cause.InstigatorType,
			},
			OnFailure: dndevents.Consequence{Owner: params.Owner, Reason: reasonConcentrationDamage},
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return detachSubscription(bus, id), nil
}

func inspectConcentrationPart(in *InspectPartInput) (*PartInfo, error) {
	if err := checkUnparameterizedMaintenance(in.Declaration); err != nil {
		return nil, err
	}
	if in.Snapshot.Bag == nil || len(in.Snapshot.Bag) != 0 {
		return nil, fmt.Errorf("concentration requires its explicit empty state bag")
	}
	switch in.Snapshot.State {
	case partStateActive:
		return &PartInfo{Concentration: true}, nil
	case partStateEnded:
		return &PartInfo{}, nil
	default:
		return nil, fmt.Errorf("unknown concentration state %q", in.Snapshot.State)
	}
}
