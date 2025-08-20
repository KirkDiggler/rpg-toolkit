package bundles

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/choices"
)

// ResolveInput contains the parameters for resolving a bundle
type ResolveInput struct {
	BundleID BundleID
}

// ResolveOutput contains the resolved bundle items
type ResolveOutput struct {
	Items []choices.CountedItem
}

// Resolve expands a bundle ID into its constituent items
func Resolve(input *ResolveInput) (*ResolveOutput, error) {
	if input == nil {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "input is required")
	}

	if input.BundleID == "" {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "bundle ID is required")
	}

	bundle, err := GetBundle(input.BundleID)
	if err != nil {
		return nil, fmt.Errorf("failed to get bundle: %w", err)
	}

	return &ResolveOutput{
		Items: bundle.Items,
	}, nil
}

// ResolveBundleOption takes a BundleOption and returns the items it contains
func ResolveBundleOption(option choices.BundleOption) ([]choices.CountedItem, error) {
	// If the option already has items defined, return those
	if len(option.Items) > 0 {
		return option.Items, nil
	}

	// Otherwise, try to resolve by ID
	bundleID := BundleID(option.ID)
	input := &ResolveInput{BundleID: bundleID}
	output, err := Resolve(input)
	if err != nil {
		return nil, err
	}

	return output.Items, nil
}

// CreateBundleOption creates a BundleOption from a BundleID
func CreateBundleOption(id BundleID) (choices.BundleOption, error) {
	bundle, err := GetBundle(id)
	if err != nil {
		return choices.BundleOption{}, err
	}

	return choices.BundleOption{
		ID:    string(id),
		Items: bundle.Items,
	}, nil
}

// ExpandChoiceOptionsInput contains parameters for expanding choice options
type ExpandChoiceOptionsInput struct {
	Choice *choices.Choice
}

// ExpandChoiceOptionsOutput contains the expanded choices
type ExpandChoiceOptionsOutput struct {
	ExpandedOptions []choices.Option
}

// ExpandChoiceOptions expands any bundle references in a choice's options
func ExpandChoiceOptions(input *ExpandChoiceOptionsInput) (*ExpandChoiceOptionsOutput, error) {
	if input == nil || input.Choice == nil {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "choice is required")
	}

	expanded := make([]choices.Option, 0, len(input.Choice.Options))

	for _, option := range input.Choice.Options {
		switch opt := option.(type) {
		case choices.BundleOption:
			// If the bundle option doesn't have items, try to resolve them
			if len(opt.Items) == 0 && opt.ID != "" {
				bundleID := BundleID(opt.ID)
				if ValidateBundleID(bundleID) {
					bundle, err := GetBundle(bundleID)
					if err != nil {
						return nil, fmt.Errorf("failed to expand bundle %s: %w", opt.ID, err)
					}
					opt.Items = bundle.Items
				}
			}
			expanded = append(expanded, opt)
		default:
			// Pass through non-bundle options unchanged
			expanded = append(expanded, option)
		}
	}

	return &ExpandChoiceOptionsOutput{
		ExpandedOptions: expanded,
	}, nil
}
