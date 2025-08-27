#!/usr/bin/env python3

import re
import sys

def fix_eventbus_connections(filename):
    with open(filename, 'r') as f:
        lines = f.readlines()
    
    output_lines = []
    i = 0
    
    while i < len(lines):
        line = lines[i]
        output_lines.append(line)
        
        # Check if this line starts a struct assignment
        match = re.match(r'\s*(\w+)\s*:=\s*(spatial\.)?New(BasicRoom|BasicRoomOrchestrator)\(', line)
        if match:
            var_name = match.group(1)
            
            # Find the closing bracket for this struct
            bracket_count = 0
            found_opening = False
            j = i
            
            while j < len(lines):
                current_line = lines[j]
                for char in current_line:
                    if char == '{':
                        bracket_count += 1
                        found_opening = True
                    elif char == '}':
                        bracket_count -= 1
                        
                        if found_opening and bracket_count == 0:
                            # Found the closing bracket, add ConnectToEventBus call
                            output_lines.append(f"\t{var_name}.ConnectToEventBus(eventBus)\n")
                            break
                
                if found_opening and bracket_count == 0:
                    break
                    
                if j > i:  # Don't add the same line twice
                    output_lines.append(current_line)
                j += 1
            
            i = j
        
        i += 1
    
    # Write the result back
    with open(filename, 'w') as f:
        f.writelines(output_lines)

if __name__ == "__main__":
    if len(sys.argv) != 2:
        print("Usage: python3 add-connect-calls.py <filename>")
        sys.exit(1)
    
    fix_eventbus_connections(sys.argv[1])
    print(f"Fixed EventBus connections in {sys.argv[1]}")