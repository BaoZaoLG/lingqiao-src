import os
import re
import glob

# Files to process
os.chdir(r"c:\Users\Li\Downloads\Lingqiao_src\server")
files = glob.glob("*.go")

for f in files:
    with open(f, "r", encoding="utf-8") as file:
        content = file.read()
    
    if f == "card_types.go":
        content = content.replace("type CardManager struct {", "type JSONCardManager struct {")
    elif f == "card_ops.go":
        content = content.replace("func NewCardManager(storage *JSONStorage) *CardManager", "func NewJSONCardManager(storage *JSONStorage) *JSONCardManager")
        content = re.sub(r"func \([^)]+\*CardManager\)", lambda m: m.group(0).replace("*CardManager", "*JSONCardManager"), content)
    elif f == "card_extra.go":
        content = re.sub(r"func \([^)]+\*CardManager\)", lambda m: m.group(0).replace("*CardManager", "*JSONCardManager"), content)
    elif f == "card_manager_test.go":
        content = content.replace("func newTestCardManager(t *testing.T) *CardManager", "func newTestCardManager(t *testing.T) CardManager")
        content = content.replace("NewCardManager", "NewJSONCardManager")
    
    # Replace parameter and struct fields `*CardManager` -> `CardManager`
    # Also NewCardManager -> NewJSONCardManager if it's main.go etc.
    if f not in ["card_types.go", "card_ops.go", "card_extra.go", "card_manager_test.go"]:
        content = content.replace("*CardManager", "CardManager")
        
    if f == "main.go":
        content = content.replace("NewCardManager", "NewJSONCardManager")
    
    with open(f, "w", encoding="utf-8") as file:
        file.write(content)

print("Refactoring done.")
