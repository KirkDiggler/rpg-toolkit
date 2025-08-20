package bundles_test

import (
	"fmt"
	"log"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/bundles"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/choices"
)

func ExampleGetBundle() {
	// Get the Explorer's Pack bundle
	bundle, err := bundles.GetBundle(bundles.ExplorersPack)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Bundle: %s\n", bundle.Name)
	fmt.Printf("Description: %s\n", bundle.Description)
	fmt.Printf("Contains %d item types:\n", len(bundle.Items))

	for _, item := range bundle.Items {
		if item.Quantity > 1 {
			fmt.Printf("  - %s (x%d)\n", item.ItemID, item.Quantity)
		} else {
			fmt.Printf("  - %s\n", item.ItemID)
		}
	}

	// Output:
	// Bundle: Explorer's Pack
	// Description: Includes a backpack, bedroll, mess kit, tinderbox, torches, rations, waterskin, and rope
	// Contains 8 item types:
	//   - backpack
	//   - bedroll
	//   - mess-kit
	//   - tinderbox
	//   - torch (x10)
	//   - rations (x10)
	//   - waterskin
	//   - hempen-rope
}

func ExampleResolve() {
	// Resolve a bundle ID to get its items
	input := &bundles.ResolveInput{
		BundleID: bundles.ScholarsPack,
	}

	output, err := bundles.Resolve(input)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Scholar's Pack contains %d item types\n", len(output.Items))
	for _, item := range output.Items {
		fmt.Printf("  - %s\n", item.ItemID)
	}

	// Output:
	// Scholar's Pack contains 7 item types
	//   - backpack
	//   - book
	//   - ink
	//   - ink-pen
	//   - parchment
	//   - little-bag-of-sand
	//   - small-knife
}

func ExampleExpandChoiceOptions() {
	// Create a choice with bundle references that need expansion
	choice := &choices.Choice{
		ID:          choices.ChoiceID("equipment-pack"),
		Category:    choices.CategoryEquipment,
		Description: "Choose your adventuring pack",
		Choose:      1,
		Source:      choices.SourceClass,
		Options: []choices.Option{
			choices.BundleOption{
				ID:    string(bundles.ExplorersPack),
				Items: nil, // Will be resolved
			},
			choices.BundleOption{
				ID:    string(bundles.DungeoneersPack),
				Items: nil, // Will be resolved
			},
		},
	}

	// Expand the bundle references
	input := &bundles.ExpandChoiceOptionsInput{
		Choice: choice,
	}

	output, err := bundles.ExpandChoiceOptions(input)
	if err != nil {
		log.Fatal(err)
	}

	// Now the bundles are expanded with their items
	for i, option := range output.ExpandedOptions {
		bundleOpt := option.(choices.BundleOption)
		fmt.Printf("Option %d: %s with %d item types\n",
			i+1, bundleOpt.ID, len(bundleOpt.Items))
	}

	// Output:
	// Option 1: explorers-pack with 8 item types
	// Option 2: dungeoneers-pack with 9 item types
}

func ExampleCreateBundleOption() {
	// Create a BundleOption from a BundleID
	option, err := bundles.CreateBundleOption(bundles.PriestsPack)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Created bundle option: %s\n", option.ID)
	fmt.Printf("Contains %d item types\n", len(option.Items))

	// Check for specific religious items
	for _, item := range option.Items {
		if item.ItemID == "incense" || item.ItemID == "censer" {
			fmt.Printf("  - %s (religious item)\n", item.ItemID)
		}
	}

	// Output:
	// Created bundle option: priests-pack
	// Contains 9 item types
	//   - incense (religious item)
	//   - censer (religious item)
}
