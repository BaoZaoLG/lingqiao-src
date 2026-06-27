import json
import sys

transcript_path = r'C:\Users\Li\.gemini\antigravity\brain\2975272a-e55b-4577-9904-ed8f123a7ba9\.system_generated\logs\transcript_full.jsonl'

def apply_write(args):
    path = args['TargetFile']
    with open(path, 'w', encoding='utf-8') as f:
        f.write(args['CodeContent'])

def apply_replace(args):
    path = args['TargetFile']
    try:
        with open(path, 'r', encoding='utf-8') as f:
            content = f.read()
    except: return
    
    target = args['TargetContent']
    replacement = args['ReplacementContent']
    allow_multiple = args.get('AllowMultiple', False)
    
    count = content.count(target)
    if count == 0:
        print(f"Warning: Target not found in {path}")
        return
    if count > 1 and not allow_multiple:
        print(f"Warning: Multiple targets found in {path}")
        return
    
    new_content = content.replace(target, replacement)
    with open(path, 'w', encoding='utf-8') as f:
        f.write(new_content)

def apply_multi_replace(args):
    path = args['TargetFile']
    try:
        with open(path, 'r', encoding='utf-8') as f:
            content = f.read()
    except: return
    
    chunks = args['ReplacementChunks']
    for chunk in chunks:
        target = chunk['TargetContent']
        replacement = chunk['ReplacementContent']
        allow_multiple = chunk.get('AllowMultiple', False)
        
        count = content.count(target)
        if count == 0:
            print(f"Warning: Target not found in {path}")
            continue
        if count > 1 and not allow_multiple:
            print(f"Warning: Multiple targets found in {path}")
            continue
            
        content = content.replace(target, replacement)
        
    with open(path, 'w', encoding='utf-8') as f:
        f.write(content)

with open(transcript_path, 'r', encoding='utf-8') as f:
    for line in f:
        try:
            step = json.loads(line)
            if step['type'] == 'PLANNER_RESPONSE' and step['status'] == 'DONE':
                # Stop before the current step where we did the bad UI changes
                # Let's say we stop at step 1485 (before the replace_file_content calls)
                # Wait, what step are we currently on? Let's check step index.
                if step['step_index'] >= 1480: 
                    break
                
                for tool in step.get('tool_calls', []):
                    name = tool['name']
                    args = tool['args']
                    if name == 'write_to_file':
                        apply_write(args)
                    elif name == 'replace_file_content':
                        apply_replace(args)
                    elif name == 'multi_replace_file_content':
                        apply_multi_replace(args)
        except Exception as e:
            print("Error", e)