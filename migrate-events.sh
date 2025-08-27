#!/bin/bash

# Event Bus Migration Script
# Migrates modules from legacy string-based events to typed event system

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BACKUP_DIR="${SCRIPT_DIR}/migration-backups"

# Logging
log_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
log_success() { echo -e "${GREEN}[SUCCESS]${NC} $1"; }
log_warning() { echo -e "${YELLOW}[WARNING]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

# Event constant to topic mapping for spatial module
declare -A SPATIAL_EVENT_MAP=(
    ["EventEntityPlaced"]="EntityPlacedTopic"
    ["EventEntityMoved"]="EntityMovedTopic"  
    ["EventEntityRemoved"]="EntityRemovedTopic"
    ["EventRoomCreated"]="RoomCreatedTopic"
    ["EventRoomAdded"]="RoomAddedTopic"
    ["EventRoomRemoved"]="RoomRemovedTopic"
    ["EventConnectionAdded"]="ConnectionAddedTopic"
    ["EventConnectionRemoved"]="ConnectionRemovedTopic"
    ["EventEntityTransitionBegan"]="EntityTransitionBeganTopic"
    ["EventEntityTransitionEnded"]="EntityTransitionEndedTopic"
    ["EventEntityRoomTransition"]="EntityRoomTransitionTopic"
    ["EventLayoutChanged"]="LayoutChangedTopic"
)

# Event constant to event type mapping for spatial module
declare -A SPATIAL_TYPE_MAP=(
    ["EventEntityPlaced"]="EntityPlacedEvent"
    ["EventEntityMoved"]="EntityMovedEvent"
    ["EventEntityRemoved"]="EntityRemovedEvent"
    ["EventRoomCreated"]="RoomCreatedEvent"
    ["EventRoomAdded"]="RoomAddedEvent"
    ["EventRoomRemoved"]="RoomRemovedEvent"
    ["EventConnectionAdded"]="ConnectionAddedEvent"
    ["EventConnectionRemoved"]="ConnectionRemovedEvent"
    ["EventEntityTransitionBegan"]="EntityTransitionBeganEvent"
    ["EventEntityTransitionEnded"]="EntityTransitionEndedEvent"
    ["EventEntityRoomTransition"]="EntityRoomTransitionEvent"
    ["EventLayoutChanged"]="LayoutChangedEvent"
)

# Environment module mappings
declare -A ENV_EVENT_MAP=(
    ["EventGenerationStarted"]="GenerationStartedTopic"
    ["EventGenerationProgress"]="GenerationProgressTopic"
    ["EventGenerationCompleted"]="GenerationCompletedTopic"
    ["EventGenerationFailed"]="GenerationFailedTopic"
    ["EventEmergencyFallbackTriggered"]="EmergencyFallbackTopic"
    ["EventEnvironmentGenerated"]="EnvironmentGeneratedTopic"
    ["EventEnvironmentDestroyed"]="EnvironmentDestroyedTopic"
    ["EventEnvironmentEntityAdded"]="EnvironmentEntityAddedTopic"
    ["EventEnvironmentEntityRemoved"]="EnvironmentEntityRemovedTopic"
    ["EventEnvironmentRoomAdded"]="EnvironmentRoomAddedTopic"
    ["EventEnvironmentRoomRemoved"]="EnvironmentRoomRemovedTopic"
    ["EventFeatureAdded"]="FeatureAddedTopic"
    ["EventFeatureRemoved"]="FeatureRemovedTopic"
    ["EventHazardTriggered"]="HazardTriggeredTopic"
)

declare -A ENV_TYPE_MAP=(
    ["EventGenerationStarted"]="GenerationStartedEvent"
    ["EventGenerationProgress"]="GenerationProgressEvent"
    ["EventGenerationCompleted"]="GenerationCompletedEvent"
    ["EventGenerationFailed"]="GenerationFailedEvent"
    ["EventEmergencyFallbackTriggered"]="EmergencyFallbackEvent"
    ["EventEnvironmentGenerated"]="EnvironmentGeneratedEvent"
    ["EventEnvironmentDestroyed"]="EnvironmentDestroyedEvent"
    ["EventEnvironmentEntityAdded"]="EnvironmentEntityAddedEvent"
    ["EventEnvironmentEntityRemoved"]="EnvironmentEntityRemovedEvent"
    ["EventEnvironmentRoomAdded"]="EnvironmentRoomAddedEvent"
    ["EventEnvironmentRoomRemoved"]="EnvironmentRoomRemovedEvent"
    ["EventFeatureAdded"]="FeatureAddedEvent"
    ["EventFeatureRemoved"]="FeatureRemovedEvent"
    ["EventHazardTriggered"]="HazardTriggeredEvent"
)

# Spawn module mappings
declare -A SPAWN_EVENT_MAP=(
    ["\"spawn.entity.spawned\""]="EntitySpawnedTopic"
    ["\"spawn.split.recommended\""]="SplitRecommendedTopic"
    ["\"spawn.room.scaled\""]="RoomScaledTopic"
)

declare -A SPAWN_TYPE_MAP=(
    ["\"spawn.entity.spawned\""]="EntitySpawnedEvent"
    ["\"spawn.split.recommended\""]="SplitRecommendedEvent"
    ["\"spawn.room.scaled\""]="RoomScaledEvent"
)

# Function to create backup
create_backup() {
    local module_path="$1"
    local module_name=$(basename "$module_path")
    local backup_path="${BACKUP_DIR}/${module_name}_$(date +%Y%m%d_%H%M%S)"
    
    mkdir -p "$backup_path"
    cp -r "$module_path"/* "$backup_path/" 2>/dev/null || true
    log_info "Created backup: $backup_path"
}

# Function to detect Go files that need migration
find_migration_files() {
    local module_path="$1"
    
    # Find .go files (excluding test files for now) that contain old event patterns
    find "$module_path" -name "*.go" ! -name "*_test.go" -exec grep -l \
        -e "events\.NewGameEvent" \
        -e "eventBus\.Publish" \
        -e "eventBus\.Subscribe" \
        -e "EventEntity\|EventRoom\|EventConnection\|EventGeneration\|EventEnvironment\|EventFeature\|EventHazard\|EventLayout\|EventTransition" \
        -e "spawn\.entity\.spawned\|spawn\.split\|spawn\.room" \
        {} \;
}

# Function to update struct fields (eventBus -> typed topics)
update_struct_fields() {
    local file="$1"
    local module_type="$2"
    
    log_info "Updating struct fields in $(basename "$file")"
    
    # Replace eventBus field with comment directing to ConnectToEventBus
    sed -i 's/eventBus events\.EventBus/\/\/ Event topics connected via ConnectToEventBus() method/g' "$file"
    
    # Add import for time if not present (needed for event timestamps)
    if ! grep -q '"time"' "$file"; then
        sed -i '/import (/a\\t"time"' "$file"
    fi
}

# Function to convert event publishing
convert_event_publishing() {
    local file="$1"
    local module_type="$2"
    
    log_info "Converting event publishing in $(basename "$file")"
    
    # Get the appropriate mappings for this module
    local -n event_map_ref
    local -n type_map_ref
    
    case "$module_type" in
        "spatial")
            event_map_ref=SPATIAL_EVENT_MAP
            type_map_ref=SPATIAL_TYPE_MAP
            ;;
        "environments")
            event_map_ref=ENV_EVENT_MAP
            type_map_ref=ENV_TYPE_MAP
            ;;
        "spawn")
            event_map_ref=SPAWN_EVENT_MAP
            type_map_ref=SPAWN_TYPE_MAP
            ;;
    esac
    
    # Convert each event publishing pattern
    for old_event in "${!event_map_ref[@]}"; do
        local topic_name="${event_map_ref[$old_event]}"
        local event_type="${type_map_ref[$old_event]}"
        
        # Convert: events.NewGameEvent(EventX, ...) patterns to typed publishing
        # This is a complex transformation, so we'll do it step by step
        
        # First, mark the old patterns for conversion
        sed -i "s/events\.NewGameEvent($old_event,/MIGRATE_EVENT_PUBLISH_${old_event}_START/g" "$file"
    done
    
    # Now convert the marked patterns to typed publishing
    # Note: This is simplified - full implementation would need to parse context data
    for old_event in "${!event_map_ref[@]}"; do
        local topic_name="${event_map_ref[$old_event]}"
        local event_type="${type_map_ref[$old_event]}"
        
        # Replace the marked pattern with typed publishing template
        sed -i "s/MIGRATE_EVENT_PUBLISH_${old_event}_START/\\/\\/ TODO: Convert to ${topic_name}.Publish(ctx, ${event_type}{...})/g" "$file"
    done
}

# Function to convert event subscriptions
convert_event_subscriptions() {
    local file="$1" 
    local module_type="$2"
    
    log_info "Converting event subscriptions in $(basename "$file")"
    
    # Convert SubscribeFunc patterns to typed subscriptions
    sed -i 's/eventBus\.SubscribeFunc(/\/\/ TODO: Convert to typedTopic.Subscribe(/g' "$file"
}

# Function to add ConnectToEventBus method
add_connect_method() {
    local file="$1"
    local module_type="$2"
    
    # Check if this file defines structs that need ConnectToEventBus
    if grep -q "type.*struct" "$file" && grep -q "Event topics connected via ConnectToEventBus" "$file"; then
        log_info "Adding ConnectToEventBus method template to $(basename "$file")"
        
        echo "" >> "$file"
        echo "// ConnectToEventBus connects this component to an event bus for typed event publishing" >> "$file"
        echo "// TODO: Implement ConnectToEventBus method with appropriate topic connections" >> "$file"
        echo "// func (x *StructName) ConnectToEventBus(bus events.EventBus) {" >> "$file"
        echo "//     x.topicName = TopicName.On(bus)" >> "$file" 
        echo "//     // ... connect other topics" >> "$file"
        echo "// }" >> "$file"
    fi
}

# Function to migrate a single module
migrate_module() {
    local module_path="$1"
    local module_name=$(basename "$module_path")
    
    log_info "Starting migration of $module_name module at $module_path"
    
    # Validate module path
    if [[ ! -d "$module_path" ]]; then
        log_error "Module path does not exist: $module_path"
        return 1
    fi
    
    # Check if topics.go exists
    if [[ ! -f "$module_path/topics.go" ]]; then
        log_error "topics.go not found in $module_path - run topic creation first"
        return 1
    fi
    
    # Create backup
    create_backup "$module_path"
    
    # Find files that need migration
    local files_to_migrate=($(find_migration_files "$module_path"))
    
    if [[ ${#files_to_migrate[@]} -eq 0 ]]; then
        log_success "No files need migration in $module_name"
        return 0
    fi
    
    log_info "Found ${#files_to_migrate[@]} files to migrate in $module_name:"
    for file in "${files_to_migrate[@]}"; do
        echo "  - $(basename "$file")"
    done
    
    # Determine module type
    local module_type
    case "$module_name" in
        "spatial") module_type="spatial" ;;
        "environments") module_type="environments" ;;
        "spawn") module_type="spawn" ;;
        *) 
            log_warning "Unknown module type: $module_name"
            module_type="unknown"
            ;;
    esac
    
    # Process each file
    for file in "${files_to_migrate[@]}"; do
        log_info "Processing $(basename "$file")"
        
        # Skip if it's the topics.go file we created
        if [[ "$(basename "$file")" == "topics.go" ]]; then
            continue
        fi
        
        # Apply migrations
        update_struct_fields "$file" "$module_type"
        convert_event_publishing "$file" "$module_type" 
        convert_event_subscriptions "$file" "$module_type"
        add_connect_method "$file" "$module_type"
        
        log_success "Converted $(basename "$file")"
    done
    
    log_success "Migration of $module_name completed!"
    log_warning "Manual review and completion of TODOs required"
}

# Function to validate migration
validate_migration() {
    local module_path="$1"
    local module_name=$(basename "$module_path")
    
    log_info "Validating migration for $module_name"
    
    # Check if module compiles
    cd "$module_path"
    
    if go build ./...; then
        log_success "$module_name builds successfully"
    else
        log_warning "$module_name has build issues - manual fixes needed"
    fi
    
    # Check for remaining old patterns
    local old_patterns=0
    old_patterns+=$(find . -name "*.go" -exec grep -l "events\.NewGameEvent" {} \; | wc -l)
    old_patterns+=$(find . -name "*.go" -exec grep -l "eventBus\.Publish" {} \; | wc -l)
    old_patterns+=$(find . -name "*.go" -exec grep -l "eventBus\.Subscribe" {} \; | wc -l)
    
    if [[ $old_patterns -gt 0 ]]; then
        log_warning "Found $old_patterns files with old event patterns - manual review needed"
    else
        log_success "No old event patterns detected"
    fi
    
    cd "$SCRIPT_DIR"
}

# Main execution
main() {
    local module_path="${1:-}"
    
    # Script header
    echo -e "${BLUE}=================================="
    echo -e "Event Bus Migration Script"
    echo -e "==================================${NC}"
    echo
    
    # Validate arguments
    if [[ -z "$module_path" ]]; then
        log_error "Usage: $0 <module_path>"
        echo
        echo "Examples:"
        echo "  $0 tools/spatial"
        echo "  $0 tools/environments" 
        echo "  $0 tools/spawn"
        exit 1
    fi
    
    # Create backup directory
    mkdir -p "$BACKUP_DIR"
    
    # Run migration
    if migrate_module "$module_path"; then
        echo
        validate_migration "$module_path"
        echo
        log_success "Migration complete! Next steps:"
        echo "  1. Review TODO comments in migrated files"
        echo "  2. Complete ConnectToEventBus method implementations"
        echo "  3. Test the migrated module"
        echo "  4. Update any dependent modules"
    else
        log_error "Migration failed"
        exit 1
    fi
}

# Run main function
main "$@"