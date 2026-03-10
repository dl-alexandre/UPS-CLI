#!/bin/bash
set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Template placeholders
DEFAULT_OWNER="yourusername"
DEFAULT_APPNAME="mycli"
DEFAULT_DESCRIPTION="A modern Go CLI application"

# Get new values from user
echo -e "${GREEN}=== Go CLI Template Setup ===${NC}"
echo ""

# GitHub/GitLab username
read -p "GitHub/GitLab username/organization [$DEFAULT_OWNER]: " owner
owner=${owner:-$DEFAULT_OWNER}

# Application name
read -p "Application name (single word, lowercase) [$DEFAULT_APPNAME]: " appname
appname=${appname:-$DEFAULT_APPNAME}

# Repository name
read -p "Repository name (usually '${appname}' or '${appname}-cli') [${appname}]: " reponame
reponame=${reponame:-$appname}

# Description
read -p "Short description [$DEFAULT_DESCRIPTION]: " description
description=${description:-$DEFAULT_DESCRIPTION}

echo ""
echo -e "${YELLOW}Setting up project with:${NC}"
echo "  Owner:        $owner"
echo "  App Name:     $appname"
echo "  Repo Name:    $reponame"
echo "  Description:  $description"
echo ""

# Confirmation
read -p "Is this correct? (y/N): " confirm
if [[ ! $confirm =~ ^[Yy]$ ]]; then
    echo -e "${RED}Setup cancelled.${NC}"
    exit 1
fi

# Check if this is a git clone of the template (not "Use this template")
if [ -d ".git" ]; then
    echo -e "${YELLOW}⚠️  Template git history detected!${NC}"
    echo ""
    echo "You appear to have cloned the template repository."
    echo "To start fresh without the template's commit history:"
    echo ""
    echo -e "  ${GREEN}rm -rf .git && git init${NC}"
    echo ""
    read -p "Remove template git history now? (recommended) (Y/n): " remove_git
    remove_git=${remove_git:-Y}
    if [[ $remove_git =~ ^[Yy]$ ]]; then
        rm -rf .git
        git init
        echo -e "${GREEN}✅ Git history reset. Fresh repository initialized.${NC}"
    else
        echo -e "${YELLOW}⚠️  Keeping template git history. You may want to remove .git manually.${NC}"
    fi
    echo ""
fi

echo ""
echo -e "${YELLOW}Replacing placeholders...${NC}"

# Function to replace in files
replace_in_files() {
    local placeholder="$1"
    local value="$2"
    
    # Find and replace in all files except this script
    find . -type f \
        -not -path "./.git/*" \
        -not -path "./scripts/setup.sh" \
        -not -path "./dist/*" \
        -not -path "./bin/*" \
        -exec grep -l "$placeholder" {} \; | while read -r file; do
        
        # Handle different OS sed commands
        if [[ "$OSTYPE" == "darwin"* ]]; then
            # macOS
            sed -i '' "s/$placeholder/$value/g" "$file"
        else
            # Linux
            sed -i "s/$placeholder/$value/g" "$file"
        fi
    done
}

# Replace placeholders
replace_in_files "{{OWNER}}" "$owner"
replace_in_files "{{APPNAME}}" "$appname"
replace_in_files "{{REPO_NAME}}" "$reponame"
replace_in_files "{{DESCRIPTION}}" "$description"

# Rename cmd directory
if [ -d "cmd/{{APPNAME}}" ]; then
    mv "cmd/{{APPNAME}}" "cmd/$appname"
    echo -e "${GREEN}Renamed cmd/{{APPNAME}} to cmd/$appname${NC}"
fi

echo ""
echo -e "${GREEN}Setup complete!${NC}"
echo ""
echo -e "${YELLOW}Next steps:${NC}"

# Check if git was already initialized above
if [ -d ".git" ]; then
    echo "  1. Add remote: git remote add origin https://github.com/$owner/$reponame.git"
else
    echo "  1. Initialize git: git init"
    echo "  2. Add remote: git remote add origin https://github.com/$owner/$reponame.git"
fi

echo "  - Download dependencies: go mod download"
echo "  - Install git hooks: make install-hooks"
echo "  - Build and test: make build && make test"
echo "  - Customize internal/api/client.go for your API"
echo "  - Add your commands to internal/cli/cli.go"
echo ""
echo -e "${YELLOW}Optional:${NC}"
echo "  - Uncomment Homebrew section in .goreleaser.yml for tap releases"
echo "  - Uncomment Scoop section in .goreleaser.yml for Windows package manager"
echo "  - Update .github/workflows/*.yml with your repository information"
echo ""

# Remove setup script
read -p "Remove setup script? (y/N): " remove_script
if [[ $remove_script =~ ^[Yy]$ ]]; then
    rm "$0"
    echo -e "${GREEN}Setup script removed.${NC}"
fi

echo ""
echo -e "${GREEN}Happy coding! 🚀${NC}"
