#!/bin/bash
# Script to fix EventBus references in test files
# Converts from EventBus field in config to ConnectToEventBus() method call

set -e

# Files to process
FILES=(
    "tools/spatial/data_test.go"
    "tools/spatial/examples_test.go"
    "tools/spatial/query_handler_test.go" 
    "tools/spatial/room_test.go"
    "tools/spatial/orchestrator_test.go"
)

echo "🔧 Fixing EventBus references in test files..."

for file in "${FILES[@]}"; do
    if [[ -f "$file" ]]; then
        echo "Processing $file..."
        
        # Create a temporary file
        temp_file="${file}.tmp"
        
        # Process the file line by line
        while IFS= read -r line; do
            # Skip lines that contain "EventBus:" - we'll add the ConnectToEventBus call later
            if [[ "$line" =~ EventBus:[[:space:]] ]]; then
                echo "  Removing: $line"
                continue
            fi
            
            # If this line contains a struct assignment ending with }), add ConnectToEventBus call
            if [[ "$line" =~ ^[[:space:]]*([a-zA-Z_][a-zA-Z0-9_]*)[[:space:]]*:=[[:space:]]*New.*Config\{$ ]] || 
               [[ "$line" =~ ^[[:space:]]*([a-zA-Z_][a-zA-Z0-9_]*)[[:space:]]*:=[[:space:]]*.*\(.*Config\{$ ]]; then
                # Extract variable name
                var_name=$(echo "$line" | sed -E 's/^[[:space:]]*([a-zA-Z_][a-zA-Z0-9_]*)[[:space:]]*:=.*/\1/')
                echo "$line" >> "$temp_file"
                in_struct=true
                struct_var="$var_name"
            elif [[ "$line" =~ ^\}) ]] && [[ -n "$struct_var" ]]; then
                # End of struct - remove EventBus and add ConnectToEventBus call
                echo "$line" >> "$temp_file"
                
                # Add ConnectToEventBus call - try to match indentation of the struct creation
                if [[ "$struct_var" == "room" ]] || [[ "$struct_var" =~ room.* ]]; then
                    echo "	${struct_var}.ConnectToEventBus(s.eventBus)" >> "$temp_file"
                elif [[ "$struct_var" == "orchestrator" ]] || [[ "$struct_var" =~ orchestrator.* ]]; then
                    echo "	${struct_var}.ConnectToEventBus(eventBus)" >> "$temp_file"
                else
                    # Generic case - try to detect which eventBus variable to use
                    if grep -q "s\.eventBus" "$file"; then
                        echo "	${struct_var}.ConnectToEventBus(s.eventBus)" >> "$temp_file"
                    else
                        echo "	${struct_var}.ConnectToEventBus(eventBus)" >> "$temp_file"
                    fi
                fi
                
                echo "  Added: ${struct_var}.ConnectToEventBus(...)"
                struct_var=""
            else
                echo "$line" >> "$temp_file"
            fi
        done < "$file"
        
        # Replace original file
        mv "$temp_file" "$file"
        echo "✅ Updated $file"
    else
        echo "⚠️  File not found: $file"
    fi
done

echo "🎉 All test files updated!"
echo ""
echo "Next steps:"
echo "1. cd /home/frank/projects/rpg-toolkit/tools/spatial"  
echo "2. go test ./... to verify tests pass"