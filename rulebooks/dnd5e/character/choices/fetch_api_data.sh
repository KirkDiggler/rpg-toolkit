#!/bin/bash

# Base URL for the D&D 5e API (2014 version for 5e rules)
BASE_URL="https://www.dnd5eapi.co/api/2014"
DATA_DIR="character/choices/testdata/api"

# Create directories
mkdir -p "$DATA_DIR/classes"
mkdir -p "$DATA_DIR/races"
mkdir -p "$DATA_DIR/subraces"
mkdir -p "$DATA_DIR/subclasses"

# Function to fetch data
fetch_data() {
    local type=$1
    local name=$2
    local output_dir=$3

    echo "Fetching $type: $name"
    curl -s "$BASE_URL/$type/$name" > "$output_dir/$name.json"

    # Small delay to be nice to the API
    sleep 0.1
}

# Classes (12 total)
echo "=== Fetching Classes (12 files) ==="
classes=(
    "barbarian"
    "bard"
    "cleric"
    "druid"
    "fighter"
    "monk"
    "paladin"
    "ranger"
    "rogue"
    "sorcerer"
    "warlock"
    "wizard"
)

for class in "${classes[@]}"; do
    fetch_data "classes" "$class" "$DATA_DIR/classes"
done

echo ""
echo "=== Classes fetched! ==="
echo "Run with argument 'races' to fetch races next"
echo "Run with argument 'subclasses' to fetch subclasses"
echo ""

# Only continue if argument provided
if [ "$1" == "races" ]; then
    echo "=== Fetching Races (9 files) ==="
    races=(
        "dragonborn"
        "dwarf"
        "elf"
        "gnome"
        "half-elf"
        "half-orc"
        "halfling"
        "human"
        "tiefling"
    )

    for race in "${races[@]}"; do
        fetch_data "races" "$race" "$DATA_DIR/races"
    done

    echo "=== Races fetched! ==="
fi

if [ "$1" == "subraces" ]; then
    echo "=== Fetching Subraces (8 files) ==="
    subraces=(
        "hill-dwarf"
        "mountain-dwarf"
        "high-elf"
        "wood-elf"
        "dark-elf"
        "lightfoot-halfling"
        "stout-halfling"
        "rock-gnome"
    )

    for subrace in "${subraces[@]}"; do
        fetch_data "subraces" "$subrace" "$DATA_DIR/subraces"
    done

    echo "=== Subraces fetched! ==="
fi

if [ "$1" == "subclasses" ]; then
    echo "=== Fetching Subclasses (many files) ==="

    # Fighter subclasses
    echo "Fighter subclasses:"
    fetch_data "subclasses" "champion" "$DATA_DIR/subclasses"
    fetch_data "subclasses" "battle-master" "$DATA_DIR/subclasses"
    fetch_data "subclasses" "eldritch-knight" "$DATA_DIR/subclasses"

    # Cleric domains
    echo "Cleric domains:"
    fetch_data "subclasses" "life" "$DATA_DIR/subclasses"
    fetch_data "subclasses" "light" "$DATA_DIR/subclasses"
    fetch_data "subclasses" "tempest" "$DATA_DIR/subclasses"
    fetch_data "subclasses" "knowledge" "$DATA_DIR/subclasses"
    fetch_data "subclasses" "war" "$DATA_DIR/subclasses"
    fetch_data "subclasses" "trickery" "$DATA_DIR/subclasses"
    fetch_data "subclasses" "nature" "$DATA_DIR/subclasses"

    # Wizard schools
    echo "Wizard schools:"
    fetch_data "subclasses" "evocation" "$DATA_DIR/subclasses"
    fetch_data "subclasses" "abjuration" "$DATA_DIR/subclasses"
    fetch_data "subclasses" "conjuration" "$DATA_DIR/subclasses"
    fetch_data "subclasses" "divination" "$DATA_DIR/subclasses"
    fetch_data "subclasses" "enchantment" "$DATA_DIR/subclasses"
    fetch_data "subclasses" "illusion" "$DATA_DIR/subclasses"
    fetch_data "subclasses" "necromancy" "$DATA_DIR/subclasses"
    fetch_data "subclasses" "transmutation" "$DATA_DIR/subclasses"

    # Rogue archetypes
    echo "Rogue archetypes:"
    fetch_data "subclasses" "thief" "$DATA_DIR/subclasses"
    fetch_data "subclasses" "assassin" "$DATA_DIR/subclasses"
    fetch_data "subclasses" "arcane-trickster" "$DATA_DIR/subclasses"

    # Barbarian paths
    echo "Barbarian paths:"
    fetch_data "subclasses" "berserker" "$DATA_DIR/subclasses"
    fetch_data "subclasses" "totem-warrior" "$DATA_DIR/subclasses"

    # Add more as needed...
    echo "=== Core subclasses fetched! ==="
fi

echo "Done!"
echo "Usage:"
echo "  ./fetch_api_data.sh          # Fetch classes only"
echo "  ./fetch_api_data.sh races    # Fetch races"
echo "  ./fetch_api_data.sh subraces # Fetch subraces"
echo "  ./fetch_api_data.sh subclasses # Fetch subclasses"