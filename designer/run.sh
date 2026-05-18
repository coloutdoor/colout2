#!/bin/bash

# 1. Define where the virtual environment is
VENV_DIR="./deck_env"

# 2. Check if the environment exists
if [ -d "$VENV_DIR" ]; then
    # Activate the environment
    source "$VENV_DIR/bin/activate"
    
    # 3. Run your Python script
    echo "Starting Deck Planner..."
    python deck_plan.py
    
    # 4. Deactivate when finished (optional, as the script ends anyway)
    deactivate
else
    echo "Error: Virtual environment 'deck_env' not found!"
    echo "Please run: python3 -m venv deck_env"
fi
